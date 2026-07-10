package api

import (
	"bytes"
	"context"
	"fmt"
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

// delayedLatestReleaseRepo wraps a real store.Repository and delegates every
// method to it unchanged, EXCEPT LatestRelease, which pauses for `delay` after
// computing the real answer before returning it. Exactly the same
// race-amplification technique as delayedCheckRepo in
// handlers_deployment_concurrency_test.go, applied to the sibling TOCTOU in
// handleCreateRelease's version-monotonicity check. It fakes no business
// decision — every returned value is the real, fully functional
// MemoryRepository's answer; the pause only widens the real check-then-act
// window so concurrent requests reliably overlap inside it deterministically.
type delayedLatestReleaseRepo struct {
	store.Repository
	delay time.Duration
}

func (d delayedLatestReleaseRepo) LatestRelease(ctx context.Context, os otaprotocol.OSType, targetModel string) (store.Release, error) {
	rel, err := d.Repository.LatestRelease(ctx, os, targetModel)
	time.Sleep(d.delay)
	return rel, err
}

// TestReleaseCreateConcurrentRejectsDuplicateVersion proves the fix for the
// check-then-act race in handleCreateRelease (server.go's releaseMu field doc
// + handlers_release.go): LatestRelease (the monotonicity check) and
// CreateRelease (the act it gates) are two separate, individually-locked
// store calls. Without a handler-level lock serializing the whole sequence, N
// concurrent POST /releases requests carrying the IDENTICAL version for the
// same os+target_model could all observe "no conflicting latest release yet"
// before any of them finishes creating one, so MORE THAN ONE release row
// would land for the exact same (os, target_model, version) tuple — silently
// violating endpoints.md §10.1's "version must be strictly greater than the
// latest published release for this target" invariant (a same-version
// resubmission must get 409 VERSION_NOT_MONOTONIC, never 201).
//
// This is a business-invariant TOCTOU race, not a low-level memory race —
// `go test -race` alone does not catch it (every individual store method is
// correctly locked); only asserting the actual outcome (exactly one 201, the
// rest 409) does. delayedLatestReleaseRepo widens the window deterministically
// so the test does not depend on winning a sub-microsecond OS-scheduling race.
func TestReleaseCreateConcurrentRejectsDuplicateVersion(t *testing.T) {
	repo := store.NewMemoryRepository()
	ctx := context.Background()

	const n = 10
	const version = "7.0.0"
	artIDs := make([]string, n)
	for i := 0; i < n; i++ {
		artIDs[i] = fmt.Sprintf("art-concurrent-rel-%d", i)
		if err := repo.CreateArtifact(ctx, store.Artifact{
			ArtifactID:  artIDs[i],
			SHA256:      fmt.Sprintf("sha-%d", i),
			Size:        42,
			OSType:      otaprotocol.OSAndroid,
			TargetModel: "OrangePi5Max",
			Version:     version,
			Verified:    true,
			UploadedAt:  time.Now().UTC(),
		}); err != nil {
			t.Fatalf("seed artifact %d: %v", i, err)
		}
	}

	wrapped := delayedLatestReleaseRepo{Repository: repo, delay: 20 * time.Millisecond}

	var idSeq int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: 15 * time.Minute,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("test-secret-release-concurrency"),
		},
		Repo: wrapped,
		Now:  func() time.Time { return time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC) },
		NewID: func() string {
			n := atomic.AddInt64(&idSeq, 1)
			return "rel-id-" + strconv.FormatInt(n, 10)
		},
	})
	router := srv.Router()

	token, err := srv.signer.Mint("admin@release-concurrency.test",
		[]string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, srv.now())
	if err != nil {
		t.Fatalf("mint admin token: %v", err)
	}

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
			body := []byte(fmt.Sprintf(
				`{"artifact_id":%q,"version":%q,"os":"android","target_model":"OrangePi5Max"}`,
				artIDs[i], version))
			r := httptest.NewRequest(http.MethodPost, "/api/v1/releases", bytes.NewReader(body))
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
	t.Logf("concurrent release create (same version): created=%d conflicts=%d other=%d", created, conflicts, other)
	if other != 0 {
		t.Fatalf("want every response to be 201 or 409, got %d other status codes: %v", other, codes)
	}
	if created != 1 {
		t.Fatalf("want exactly 1 of %d concurrent same-version creates to succeed (201), got %d (business-invariant TOCTOU: duplicate release rows for one (os,target,version))", n, created)
	}

	releases, _, err := repo.ListReleases(ctx, store.ReleaseFilter{OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Limit: 200})
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	sameVersion := 0
	for _, r := range releases {
		if r.Version == version {
			sameVersion++
		}
	}
	if sameVersion != 1 {
		t.Fatalf("want exactly 1 stored release at version %q, got %d: %+v", version, sameVersion, releases)
	}
}
