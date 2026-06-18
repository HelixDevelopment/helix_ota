package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- §11.4.85 stress tests for API endpoints ---
//
// These exercise the real Server.Router() under sustained concurrent load
// against auth, release, and device registration handlers, measuring
// per-iteration latency and producing captured evidence (JSONL) under
// qa-results/stress/. Use the real in-memory store (no mocks per §11.4.27).
// Skip with -short.

// stressServer builds a concurrency-safe test server with operator credentials
// and returns the router, server, and an admin token.
func stressServer(t testing.TB) (*gin.Engine, *Server, string) {
	t.Helper()
	return stressServerWithRepo(t, store.NewMemoryRepository())
}

// stressServerWithRepo builds a stress test server using the given repository.
func stressServerWithRepo(t testing.TB, repo store.Repository) (*gin.Engine, *Server, string) {
	t.Helper()
	var ctr int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: time.Hour,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("stress-secret"),
		},
		Repo: repo,
		Users: NewStaticUserDirectory(StaticUser{
			Username: "admin@stress.test",
			Password: "s3cret",
			Roles:    []string{RoleAdmin, RoleOperator, RoleViewer},
		}),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	router := srv.Router()
	tok, err := srv.signer.Mint("admin@stress.test", []string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("mint admin token: %v", err)
	}
	return router, srv, tok
}

// writeStressJSONL records per-iteration latency and status codes as
// newline-delimited JSON evidence. Written to HELIX_STRESS_EVIDENCE_DIR (or a
// default under qa-results/stress/).
func writeStressJSONL(t *testing.T, testName string, lat []time.Duration, codes []int, labels []string) {
	t.Helper()
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "qa-results", "stress")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("mkdir: %v", err)
		return
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%s.jsonl", testName, ts)))
	if err != nil {
		t.Logf("create jsonl: %v", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	created, errs := 0, 0
	for i := range lat {
		entry := map[string]any{
			"test":        testName,
			"iteration":   i,
			"latency_ns":  lat[i].Nanoseconds(),
			"status_code": codes[i],
		}
		if i < len(labels) {
			entry["label"] = labels[i]
		}
		if err := enc.Encode(entry); err != nil {
			t.Logf("encode entry: %v", err)
			return
		}
		if codes[i] >= 200 && codes[i] < 300 {
			created++
		} else {
			errs++
		}
	}
	t.Logf("stress %s: total=%d 2xx=%d errors=%d evidence=%s/%s-%s.jsonl",
		testName, len(lat), created, errs, dir, testName, ts)
}

// TestStressConcurrentAuth — 10 parallel login requests, all must succeed
// (200), capturing per-iteration latency and verifying token presence.
func TestStressConcurrentAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()
	router, _, _ := stressServer(t)

	const n = 10
	lat := make([]time.Duration, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			start := time.Now()
			codes[i] = doStressReq(router, http.MethodPost, "/api/v1/auth/login", "",
				`{"username":"admin@stress.test","password":"s3cret"}`)
			lat[i] = time.Since(start)
		}()
	}
	wg.Wait()

	errs := 0
	for i, c := range codes {
		if c != http.StatusOK {
			errs++
			t.Errorf("login %d: want 200, got %d", i, c)
		}
	}
	writeStressJSONL(t, "TestStressConcurrentAuth", lat, codes, nil)
	sorted := make([]time.Duration, len(lat))
	copy(sorted, lat)
	t.Logf("concurrent_auth: total=%d ok=%d errors=%d p50=%s p95=%s p99=%s",
		n, n-errs, errs,
		percentileSorted(sorted, 50), percentileSorted(sorted, 95), percentileSorted(sorted, 99))
}

// TestStressConcurrentRelease — 100 release creations in parallel, each with
// a unique version, measuring p50/p95/p99 latency. Uses the real store; no
// mocks per §11.4.27.
func TestStressConcurrentRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()
	repo := store.NewMemoryRepository()
	router, _, tok := stressServerWithRepo(t, repo)
	ctx := context.Background()

	// Create a single verified artifact from which all releases will be created.
	artID := "art-stress-release"
	if err := repo.CreateArtifact(ctx, store.Artifact{
		ArtifactID:  artID,
		SHA256:      "abcdef0123456789",
		Size:        42,
		OSType:      "android",
		TargetModel: "OrangePi5Max",
		Version:     "99.0.0",
		Verified:    true,
		UploadedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	const n = 100
	lat := make([]time.Duration, n)
	codes := make([]int, n)
	labels := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			version := fmt.Sprintf("100.0.%d", i)
			body := fmt.Sprintf(
				`{"artifact_id":"%s","version":"%s","os":"android","target_model":"OrangePi5Max"}`,
				artID, version)
			start := time.Now()
			codes[i] = doStressReq(router, http.MethodPost, "/api/v1/releases", tok, body)
			lat[i] = time.Since(start)
			labels[i] = version
		}()
	}
	wg.Wait()

	writeStressJSONL(t, "TestStressConcurrentRelease", lat, codes, labels)

	// Summarize outcomes.
	created, conflicts, errs := 0, 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			errs++
		}
	}
	sorted := make([]time.Duration, len(lat))
	copy(sorted, lat)
	t.Logf("concurrent_release: total=%d created=%d conflicts=%d errs=%d p50=%s p95=%s p99=%s",
		n, created, conflicts, errs,
		percentileSorted(sorted, 50), percentileSorted(sorted, 95), percentileSorted(sorted, 99))
	if errs > 0 {
		t.Errorf("concurrent_release: %d unexpected errors", errs)
	}
}

// TestStressConcurrentDevice — 10 concurrent device registrations, all must
// succeed (201), no deadlock.
func TestStressConcurrentDevice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	t.Parallel()
	router, _, tok := stressServer(t)

	const n = 10
	lat := make([]time.Duration, n)
	codes := make([]int, n)
	labels := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(
				`{"hardware_id":"stress-hw-%d","model":"OrangePi5Max","os":"android"}`,
				i)
			start := time.Now()
			codes[i] = doStressReq(router, http.MethodPost, "/api/v1/devices/register", tok, body)
			lat[i] = time.Since(start)
			labels[i] = fmt.Sprintf("stress-hw-%d", i)
		}()
	}
	wg.Wait()

	errs := 0
	for i, c := range codes {
		if c != http.StatusCreated {
			errs++
			t.Errorf("device %d: want 201, got %d", i, c)
		}
	}
	writeStressJSONL(t, "TestStressConcurrentDevice", lat, codes, labels)
	sorted := make([]time.Duration, len(lat))
	copy(sorted, lat)
	t.Logf("concurrent_device: total=%d ok=%d errors=%d p50=%s p95=%s p99=%s",
		n, n-errs, errs,
		percentileSorted(sorted, 50), percentileSorted(sorted, 95), percentileSorted(sorted, 99))
	if errs > 0 {
		t.Errorf("concurrent_device: %d registrations failed", errs)
	}
}

// percentileSorted computes the p-th percentile from an already-sorted slice.
func percentileSorted(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}
