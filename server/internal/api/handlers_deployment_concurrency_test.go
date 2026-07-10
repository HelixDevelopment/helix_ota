package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// delayedCheckRepo wraps a real store.Repository and delegates every method to
// it unchanged, EXCEPT ActiveDeploymentForTarget, which pauses for `delay`
// after computing the real answer before returning it. It fakes no business
// decision whatsoever — every returned value comes from the real, fully
// functional MemoryRepository; the only added behavior is a fixed pause that
// widens handleCreateDeployment's check-then-act window so concurrent
// requests reliably overlap inside it, deterministically, instead of the test
// depending on winning an OS-thread-scheduling lottery over a sub-microsecond
// window (real, uncontrolled concurrent HTTP load hits the exact same
// TOCTOU — this wrapper only makes the already-real race reproducible on
// demand in a unit test, per §11.4.27's unit-test allowance).
type delayedCheckRepo struct {
	store.Repository
	delay time.Duration
}

func (d delayedCheckRepo) ActiveDeploymentForTarget(ctx context.Context, os otaprotocol.OSType, targetModel, group string) (store.Deployment, error) {
	dep, err := d.Repository.ActiveDeploymentForTarget(ctx, os, targetModel, group)
	time.Sleep(d.delay)
	return dep, err
}

// TestDeploymentCreateConcurrentNoDuplicateActive proves the fix for the
// check-then-act race in handleCreateDeployment (server.go's deployMu field
// doc + handlers_deployment.go): ActiveDeploymentForTarget (the conflict
// check) and CreateDeployment (the act it gates) are two separate,
// individually-locked store calls. Without a handler-level lock serializing
// the whole sequence, N concurrent POST /deployments requests for the SAME
// release could all observe "no active deployment yet" before any of them
// finishes creating one, so more than one would be created active for the
// same os+target_model+group — violating the documented endpoints.md §11.1
// invariant "an active deployment already targets this set" is a 409
// CONFLICT. This is a business-invariant TOCTOU race, not a low-level memory
// race — `go test -race` alone does not catch it (every individual store
// method is correctly locked); only asserting the actual outcome (exactly one
// 201, the rest 409, exactly one active deployment left in the store) does.
//
// delayedCheckRepo (above) widens the window deterministically so the test
// does not depend on winning a sub-microsecond OS-scheduling race: every
// concurrent request's check call pauses briefly before returning, giving
// every other concurrent request's own (real, unmodified) check call time to
// also run and also observe "not found" before any of them proceeds to
// create — exactly the interleaving a real, unlucky production request burst
// could hit.
//
// This test builds its own Server (rather than reusing the shared testEnv) so
// its own id generator is concurrency-safe (atomic counter) — testEnv's NewID
// callback increments a plain, non-atomic int, which would itself race under
// concurrent handler calls and trip -race for a reason unrelated to the
// product defect under test here.
func TestDeploymentCreateConcurrentNoDuplicateActive(t *testing.T) {
	repo := store.NewMemoryRepository()
	ctx := context.Background()

	const artID = "art-concurrent-dep"
	if err := repo.CreateArtifact(ctx, store.Artifact{
		ArtifactID:  artID,
		SHA256:      "0123456789abcdef",
		Size:        42,
		OSType:      otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max",
		Version:     "5.0.0",
		Verified:    true,
		UploadedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	const relID = "rel-concurrent-dep"
	if err := repo.CreateRelease(ctx, store.Release{
		ReleaseID:   relID,
		ArtifactID:  artID,
		Version:     "5.0.0",
		OSType:      otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max",
		Status:      "published",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed release: %v", err)
	}

	wrapped := delayedCheckRepo{Repository: repo, delay: 20 * time.Millisecond}

	var idSeq int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: 15 * time.Minute,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("test-secret-concurrency"),
		},
		Repo: wrapped,
		Now:  func() time.Time { return time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC) },
		NewID: func() string {
			n := atomic.AddInt64(&idSeq, 1)
			return "dep-id-" + strconv.FormatInt(n, 10)
		},
	})
	router := srv.Router()

	token, err := srv.signer.Mint("admin@concurrency.test",
		[]string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, srv.now())
	if err != nil {
		t.Fatalf("mint admin token: %v", err)
	}

	const n = 10
	body := []byte(`{"release_id":"` + relID + `","strategy":"all-targets"}`)
	codes := make([]int, n)

	var ready sync.WaitGroup
	ready.Add(n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			// Each goroutine builds its own *http.Request + bytes.Reader (an
			// independent read cursor over the shared, read-only body slice) so
			// there is no shared mutable request state across goroutines.
			r := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			ready.Done()
			<-start
			router.ServeHTTP(w, r)
			codes[i] = w.Code
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()

	created, conflicts, other := 0, 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			other++
		}
	}
	t.Logf("concurrent deployment create: created=%d conflicts=%d other=%d", created, conflicts, other)
	if other != 0 {
		t.Fatalf("want every response to be 201 or 409, got %d other status codes: %v", other, codes)
	}
	if created != 1 {
		t.Fatalf("want exactly 1 of %d concurrent creates to succeed (201), got %d (business-invariant TOCTOU: two+ active deployments for the same target)", n, created)
	}

	active, err := repo.ListActiveDeployments(ctx)
	if err != nil {
		t.Fatalf("list active deployments: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("want exactly 1 active deployment left in the store, got %d: %+v", len(active), active)
	}
}
