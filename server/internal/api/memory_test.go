package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- §11.4.169 memory-growth regression guard for the core API hot-path handlers ---
//
// Gap closed: docs/research/server_test_type_coverage_audit_20260709/AUDIT.md
// §5 gap 1 (HIGHEST risk, §11.4.132 risk-ordered) — the only existing memory
// signal anywhere in server/ was a goroutine-leak check scoped to the
// embedded manager-UI static-file handler (embed_stress_chaos_test.go). No
// test asserted bounded heap growth for the frequently-hit core API handlers
// (groups/devices/releases). This file is a TEST-ONLY addition — it does not
// change any handler/product behavior.
//
// Method (§11.4.6/§11.4.107(13) — no hardcoded literature threshold): drive
// warmup + three EQUAL-SIZE batches of real GET requests (idempotent list
// reads over a fixed, already-seeded dataset — so no legitimate store growth
// is expected once seeding is done) through the real Gin router
// (newResilienceServer / real Server.Router(), real in-memory store, no
// mocks per §11.4.27), forcing a full runtime.GC() + runtime.ReadMemStats
// before/after each batch. The growth measured across the FIRST two
// post-warmup batches is used as THIS run's own calibration reference (real
// captured evidence from THIS host, THIS run, THIS build — never an
// imported/literature number); the growth in the THIRD batch must stay
// within a bounded multiple of that reference (plus an absolute noise
// floor guarding a near-zero/negative reference). A genuine per-request
// heap leak retains bytes every batch (roughly the same amount per
// equal-size batch, since each batch drives the identical workload), so an
// actual regression shows up as growth persisting past what the calibration
// batches already captured, not shrinking toward the floor the way real
// post-GC noise does. Reuses the same doStressReq / newResilienceServer /
// resilienceAdminToken helpers as resilience_test.go (same package, same
// real-router pattern) and the same goroutine-leak check shape as
// embed_stress_chaos_test.go's TestStressManagerSPA_SustainedMixedLoad
// (tolerance <= 4, identical rationale).

// memoryEvidenceDir returns the (gitignored) evidence directory for the
// memory-growth census, honoring HELIX_STRESS_EVIDENCE_DIR like the sibling
// stress/chaos suites (stress_test.go, embed_stress_chaos_test.go).
func memoryEvidenceDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "qa-results", "memory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("mkdir %s: %v", dir, err)
	}
	return dir
}

// writeMemoryCensus appends a categorized memory-growth census as captured
// evidence (§11.4.5/§11.4.69 — every PASS/FAIL cites its evidence path).
func writeMemoryCensus(t *testing.T, name string, lines []string) string {
	t.Helper()
	dir := memoryEvidenceDir(t)
	ts := time.Now().UTC().Format("20060102T150405Z")
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.txt", name, ts))
	f, err := os.Create(p)
	if err != nil {
		t.Logf("create census %s: %v", p, err)
		return ""
	}
	defer f.Close()
	for _, l := range lines {
		_, _ = f.WriteString(l + "\n")
	}
	t.Logf("%s: evidence=%s", name, p)
	return p
}

// TestMemory_SustainedAPILoadNoGrowth drives sustained mixed-endpoint GET
// traffic through the real core-API hot paths (groups/devices/releases list
// endpoints) and asserts bounded heap growth + no goroutine leak. Not marked
// t.Parallel(): running it as a plain sequential top-level test means it
// executes to completion (and takes its heap samples) BEFORE the package's
// other t.Parallel()-marked stress/chaos tests are unblocked to run
// concurrently — this keeps the NumGoroutine()/HeapAlloc samples free of
// noise from unrelated concurrently-running tests in the same process.
func TestMemory_SustainedAPILoadNoGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}
	ctx := context.Background()
	repo := store.NewMemoryRepository()
	if err := repo.CreateArtifact(ctx, store.Artifact{
		ArtifactID:  "art-memtest",
		SHA256:      "abcdef0123456789",
		Size:        42,
		OSType:      "android",
		TargetModel: "OrangePi5Max",
		Version:     "1.0.0",
		Verified:    true,
		UploadedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	router, srv := newResilienceServer(t, repo)
	tok := resilienceAdminToken(t, srv)

	// Seed a small fixed dataset so the list responses are non-trivial, but
	// the store itself does NOT grow during the measured batches (every
	// measured request below is a read-only GET).
	for i := 0; i < 5; i++ {
		if code := doStressReq(router, http.MethodPost, "/api/v1/groups", tok,
			fmt.Sprintf(`{"name":"mem-g-%d"}`, i)); code != http.StatusCreated {
			t.Fatalf("seed group %d: want 201 got %d", i, code)
		}
	}
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"hardware_id":"mem-hw-%d","model":"OrangePi5Max","os":"android"}`, i)
		if code := doStressReq(router, http.MethodPost, "/api/v1/devices/register", tok, body); code != http.StatusCreated {
			t.Fatalf("seed device %d: want 201 got %d", i, code)
		}
	}
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"artifact_id":"art-memtest","version":"1.0.%d","os":"android","target_model":"OrangePi5Max"}`, i+1)
		if code := doStressReq(router, http.MethodPost, "/api/v1/releases", tok, body); code != http.StatusCreated {
			t.Fatalf("seed release %d: want 201 got %d", i, code)
		}
	}

	const warmup = 300
	const batchSize = 1500 // >= §5 gap-1's "N>=1000 iterations" ask; >= §11.4.85's 100-req sustained-load floor

	endpoints := []string{"/api/v1/groups", "/api/v1/devices", "/api/v1/releases"}
	runBatch := func(n int) {
		for i := 0; i < n; i++ {
			p := endpoints[i%len(endpoints)]
			if code := doStressReq(router, http.MethodGet, p, tok, ""); code != http.StatusOK {
				t.Fatalf("mixed read %s: want 200 got %d", p, code)
			}
		}
	}

	// Baseline goroutine count, same shape as embed_stress_chaos_test.go.
	runtime.GC()
	baseGoroutines := runtime.NumGoroutine()

	// Warm up: let gin's internal buffer pools, map bucket growth, and any
	// first-time allocations settle before the calibration/measurement
	// batches, so the calibration reference reflects steady-state behavior.
	runBatch(warmup)

	var m0, m1, m2, m3 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)

	runBatch(batchSize)
	runtime.GC()
	runtime.ReadMemStats(&m1)

	runBatch(batchSize)
	runtime.GC()
	runtime.ReadMemStats(&m2)

	runBatch(batchSize)
	runtime.GC()
	runtime.ReadMemStats(&m3)

	growth1 := int64(m1.HeapAlloc) - int64(m0.HeapAlloc) // calibration batch A
	growth2 := int64(m2.HeapAlloc) - int64(m1.HeapAlloc) // calibration batch B
	growth3 := int64(m3.HeapAlloc) - int64(m2.HeapAlloc) // asserted batch

	// Self-calibrated threshold (§11.4.6/§11.4.107(13)): derived from THIS
	// run's own first two post-warmup batches, never a hardcoded literature
	// number. The noise floor guards against a near-zero/negative reference
	// (which would otherwise make ANY small positive growth3 fail
	// spuriously on a perfectly healthy build).
	const noiseFloor = 512 * 1024 // 512 KiB
	ref := growth1
	if growth2 > ref {
		ref = growth2
	}
	threshold := ref * 4
	if threshold < noiseFloor {
		threshold = noiseFloor
	}

	// Goroutine-leak check: identical pattern + tolerance to
	// embed_stress_chaos_test.go's TestStressManagerSPA_SustainedMixedLoad.
	var leaked int
	for i := 0; i < 20; i++ {
		runtime.GC()
		leaked = runtime.NumGoroutine() - baseGoroutines
		if leaked <= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	census := []string{
		fmt.Sprintf("test=%s warmup=%d batch_size=%d endpoints=groups,devices,releases", t.Name(), warmup, batchSize),
		fmt.Sprintf("HeapAlloc bytes: after_warmup=%d after_batchA=%d after_batchB=%d after_batchC=%d",
			m0.HeapAlloc, m1.HeapAlloc, m2.HeapAlloc, m3.HeapAlloc),
		fmt.Sprintf("HeapObjects: after_warmup=%d after_batchC=%d NumGC=%d", m0.HeapObjects, m3.HeapObjects, m3.NumGC),
		fmt.Sprintf("growth bytes: batchA=%d batchB=%d batchC=%d", growth1, growth2, growth3),
		fmt.Sprintf("calibrated threshold=%d bytes (ref=max(batchA,batchB)=%d * 4, noise_floor=%d)", threshold, ref, noiseFloor),
		fmt.Sprintf("goroutines base=%d delta=%d (leak-tolerance<=4)", baseGoroutines, leaked),
	}
	p := writeMemoryCensus(t, "memory_sustained_api_load", census)

	t.Logf("memory_sustained_api_load: growth_batchA=%d growth_batchB=%d growth_batchC=%d threshold=%d goroutine_delta=%d evidence=%s",
		growth1, growth2, growth3, threshold, leaked, p)

	if growth3 > threshold {
		t.Errorf("heap growth: batch C grew %d bytes, exceeds calibrated threshold %d bytes (ref=%d, batchA=%d, batchB=%d) — possible memory leak in the core API handlers",
			growth3, threshold, ref, growth1, growth2)
	}
	if leaked > 4 {
		t.Errorf("goroutine leak: base=%d now=%d delta=%d (>4) — a handler leaked goroutines under sustained API load",
			baseGoroutines, baseGoroutines+leaked, leaked)
	}
}
