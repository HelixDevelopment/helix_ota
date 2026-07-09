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
// Method — RETENTION-SCALES-WITH-LOAD discriminator (§11.4.6/§11.4.107(13);
// corrects the earlier per-equal-batch-delta design flagged by the session
// whole-branch review as unable to detect a *steady* linear leak). Drive
// warmup, then measure retained live heap (post-GC HeapAlloc, relative to a
// warmed baseline) at TWO very different cumulative request counts — a small
// phase (N requests) and a large phase (~LARGE_MULT×N requests) — of real GET
// traffic (idempotent list reads over a fixed, already-seeded dataset, so no
// legitimate store growth is expected once seeding is done) through the real
// Gin router (newResilienceServer / real Server.Router(), real in-memory
// store, no mocks per §11.4.27).
//
// The load-invariant signal a steady leak produces: retained live heap that
// SCALES with the number of requests served. A handler that leaks L bytes
// per request retains ~N·L after the small phase and ~LARGE_MULT·N·L after
// the large phase (≈ LARGE_MULT× more) — retention tracks request count. A
// healthy handler's post-GC live heap PLATEAUS after warmup (pools/buckets
// saturate) and does NOT scale with request count, so retainedLarge ≈
// retainedSmall. The verdict is therefore a pure comparison —
// classifyHeapGrowth: FAIL only when retainedLarge both exceeds an absolute
// noise floor AND exceeds a bounded multiple of the small-phase retention
// (retention scaled up with load). This catches the sub-floor-per-batch
// steady leak the old delta test missed, because the leak's signature is the
// SCALING across 8× the requests, not the size of any single equal batch.
// The classifier is self-validated (§11.4.107(10)) by TestMemoryGrowthClassifier
// (golden-good flat / golden-bad scaling) AND proven on a REAL injected steady
// leak (§11.4.115 RED) by TestMemoryGrowthClassifier_DetectsInjectedSteadyLeak.
// Reuses the same doStressReq / newResilienceServer / resilienceAdminToken
// helpers as resilience_test.go and the same goroutine-leak check shape as
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
	const smallN = 1500 // >= §5 gap-1's "N>=1000 iterations" ask; >= §11.4.85's 100-req sustained-load floor
	const largeMult = 8 // large phase serves ~8x the small phase's requests; a steady leak scales retention ~8x

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

	const noiseFloor = 512 * 1024 // 512 KiB absolute floor below which retention is treated as post-GC noise

	// Warmed baseline: post-GC live heap after warmup, BEFORE the measured phases.
	var mBase, mSmall, mLarge runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&mBase)

	// Small phase: smallN requests, then post-GC retained heap vs the baseline.
	runBatch(smallN)
	runtime.GC()
	runtime.ReadMemStats(&mSmall)

	// Large phase: (largeMult-1)*smallN MORE requests (cumulative largeMult*smallN
	// since the baseline), then post-GC retained heap vs the SAME baseline.
	runBatch(smallN * (largeMult - 1))
	runtime.GC()
	runtime.ReadMemStats(&mLarge)

	retainedSmall := int64(mSmall.HeapAlloc) - int64(mBase.HeapAlloc)
	retainedLarge := int64(mLarge.HeapAlloc) - int64(mBase.HeapAlloc)

	// Verdict via the pure, self-validated classifier (§11.4.107(10)): a leak
	// is reported only when retention BOTH exceeds the absolute noise floor AND
	// scaled up with the 8x request count (retainedLarge > ref * scaleNum/scaleDen).
	const scaleNum, scaleDen = 3, 1
	leak, reason := classifyHeapGrowth(retainedSmall, retainedLarge, noiseFloor, scaleNum, scaleDen)

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
		fmt.Sprintf("test=%s warmup=%d small_n=%d large_mult=%d endpoints=groups,devices,releases", t.Name(), warmup, smallN, largeMult),
		fmt.Sprintf("HeapAlloc bytes: baseline=%d after_small=%d after_large=%d", mBase.HeapAlloc, mSmall.HeapAlloc, mLarge.HeapAlloc),
		fmt.Sprintf("HeapObjects: baseline=%d after_large=%d NumGC=%d", mBase.HeapObjects, mLarge.HeapObjects, mLarge.NumGC),
		fmt.Sprintf("retained-vs-baseline bytes: small_phase(%d reqs)=%d large_phase(%d reqs)=%d", smallN, retainedSmall, smallN*largeMult, retainedLarge),
		fmt.Sprintf("verdict leak=%v scale=%d/%d noise_floor=%d reason=%q", leak, scaleNum, scaleDen, noiseFloor, reason),
		fmt.Sprintf("goroutines base=%d delta=%d (leak-tolerance<=4)", baseGoroutines, leaked),
	}
	p := writeMemoryCensus(t, "memory_sustained_api_load", census)

	t.Logf("memory_sustained_api_load: retainedSmall=%d retainedLarge=%d leak=%v reason=%q goroutine_delta=%d evidence=%s",
		retainedSmall, retainedLarge, leak, reason, leaked, p)

	if leak {
		t.Errorf("heap retention scaled with load across %dx requests: retainedSmall=%d retainedLarge=%d (%s) — possible memory leak in the core API handlers",
			largeMult, retainedSmall, retainedLarge, reason)
	}
	if leaked > 4 {
		t.Errorf("goroutine leak: base=%d now=%d delta=%d (>4) — a handler leaked goroutines under sustained API load",
			baseGoroutines, baseGoroutines+leaked, leaked)
	}
}

// classifyHeapGrowth is the pure verdict for the retention-scales-with-load
// leak discriminator, extracted so it is unit-self-validatable independently
// of the slow real-router measurement (§11.4.107(10)). retainedSmall /
// retainedLarge are post-GC live-heap bytes retained (vs a warmed baseline)
// after the small and large request phases. A leak is reported ONLY when
// retention BOTH exceeds the absolute noiseFloor AND scaled up beyond
// scaleNum/scaleDen of the small-phase retention (retention tracked request
// count) — the load-invariant signature of a steady per-request leak. A
// healthy handler plateaus (retainedLarge ~ retainedSmall) or stays below the
// floor, so it is not flagged. This REPLACES the earlier per-equal-batch delta
// threshold, which could not see a steady linear leak (each equal batch
// retained ~equal bytes, so the asserted batch never exceeded a multiple of
// the calibration batches — the reference was contaminated by the leak itself).
func classifyHeapGrowth(retainedSmall, retainedLarge, noiseFloor int64, scaleNum, scaleDen int64) (leak bool, reason string) {
	if retainedLarge <= noiseFloor {
		return false, fmt.Sprintf("retainedLarge=%d <= noiseFloor=%d (flat/below floor — retention did not scale)", retainedLarge, noiseFloor)
	}
	ref := retainedSmall
	if ref < noiseFloor {
		ref = noiseFloor // never divide noise-by-noise; anchor the ratio at the floor
	}
	if retainedLarge*scaleDen > ref*scaleNum {
		return true, fmt.Sprintf("retainedLarge=%d > ref=%d * %d/%d (retention scaled with request count → leak)", retainedLarge, ref, scaleNum, scaleDen)
	}
	return false, fmt.Sprintf("retainedLarge=%d <= ref=%d * %d/%d (retention did not scale with load)", retainedLarge, ref, scaleNum, scaleDen)
}

// TestMemoryGrowthClassifier self-validates the leak discriminator (§11.4.107(10)):
// golden-good inputs (flat/plateau, one-time growth, GC-freed) must NOT be
// flagged; golden-bad inputs (retention scaling with load) MUST be flagged. A
// classifier that passes its golden-bad or fails its golden-good is itself a
// §11.4 bluff, so this runs as a fast unit test independent of the slow
// real-router measurement.
func TestMemoryGrowthClassifier(t *testing.T) {
	const floor = 512 * 1024
	const sn, sd = 3, 1
	kb := int64(1024)
	cases := []struct {
		name         string
		small, large int64
		wantLeak     bool
	}{
		{"flat-below-floor", 120 * kb, 140 * kb, false},             // healthy plateau, below floor
		{"one-time-growth-no-scaling", 800 * kb, 900 * kb, false},   // grew once above floor but did not scale
		{"negative-small-flat-large", -50 * kb, 100 * kb, false},    // GC freed in small phase; large below floor
		{"steady-leak-scales-8x", 800 * kb, 6400 * kb, true},        // retention ~8x with 8x load → leak
		{"subfloor-small-scaling-large", 300 * kb, 2400 * kb, true}, // small below floor, large scales far past it
		{"exactly-at-3x-ref-not-leak", 800 * kb, 2400 * kb, false},  // == ref*3, strictly-greater test → not a leak
		{"just-over-3x-ref-leak", 800 * kb, 2400*kb + 1, true},      // one byte over the 3x bound → leak
	}
	for _, c := range cases {
		leak, reason := classifyHeapGrowth(c.small, c.large, floor, sn, sd)
		if leak != c.wantLeak {
			t.Errorf("%s: classifyHeapGrowth(small=%d,large=%d) leak=%v want=%v (%s)", c.name, c.small, c.large, leak, c.wantLeak, reason)
		}
	}
	// Explicit §11.4.107(10) self-validation gate.
	goodLeak, _ := classifyHeapGrowth(120*kb, 140*kb, floor, sn, sd)
	badLeak, _ := classifyHeapGrowth(800*kb, 6400*kb, floor, sn, sd)
	if goodLeak || !badLeak {
		t.Fatalf("analyzer self-validation FAILED: golden-good flagged=%v (want false), golden-bad flagged=%v (want true)", goodLeak, badLeak)
	}
}

// TestMemoryGrowthClassifier_DetectsInjectedSteadyLeak proves the discriminator
// on a REAL steady leak (§11.4.115 RED): it retains perIter bytes on every
// iteration (the shape of a handler that appends to a package global per
// request), grows the sink with the iteration count across a small then a large
// phase, and asserts the SAME classifier the real test uses flags it. This is
// the end-to-end proof that detection — not merely the assertion wiring — works.
func TestMemoryGrowthClassifier_DetectsInjectedSteadyLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping injected-leak RED in short mode")
	}
	const noiseFloor = 512 * 1024
	const perIter = 4096
	const smallN = 2000
	const largeMult = 8
	sink := make([][]byte, 0, smallN*largeMult)
	leakIters := func(n int) {
		for i := 0; i < n; i++ {
			buf := make([]byte, perIter)
			for j := range buf {
				buf[j] = byte(i) // touch every byte so it is genuinely live, not zero-page-elided
			}
			sink = append(sink, buf)
		}
	}
	var b0, bs, bl runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&b0)
	leakIters(smallN)
	runtime.GC()
	runtime.ReadMemStats(&bs)
	leakIters(smallN * (largeMult - 1))
	runtime.GC()
	runtime.ReadMemStats(&bl)
	retainedSmall := int64(bs.HeapAlloc) - int64(b0.HeapAlloc)
	retainedLarge := int64(bl.HeapAlloc) - int64(b0.HeapAlloc)
	leak, reason := classifyHeapGrowth(retainedSmall, retainedLarge, noiseFloor, 3, 1)
	t.Logf("injected steady leak (%d B/iter): retainedSmall=%d retainedLarge=%d → leak=%v (%s)", perIter, retainedSmall, retainedLarge, leak, reason)
	if !leak {
		t.Errorf("classifier FAILED to detect an injected steady %d B/iter leak: retainedSmall=%d retainedLarge=%d (%s) — the leak discriminator is a bluff",
			perIter, retainedSmall, retainedLarge, reason)
	}
	runtime.KeepAlive(sink)
}
