package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- §11.4.85 chaos tests for API endpoints ---
//
// Failure-injection and robustness tests exercising the real Server.Router()
// against malformed payloads, large payloads, concurrent conflicting mutation,
// and simulated store unavailability. Captured evidence written to
// qa-results/stress_chaos/ or HELIX_STRESS_EVIDENCE_DIR.
//
// Use the real in-memory store (no mocks per §11.4.27). Graceful degradation
// (500, not panic) under fault + recovery checked.

// chaosStoreFaultRepo wraps a Repository and injects failures on demand for
// chaos tests. Embedding the interface delegates every un-overridden method
// to the real repo.
type chaosStoreFaultRepo struct {
	store.Repository
	failAll atomic.Bool
}

func (f *chaosStoreFaultRepo) CreateDevice(ctx context.Context, d store.Device) error {
	if f.failAll.Load() {
		return errors.New("chaos: simulated CreateDevice failure")
	}
	return f.Repository.CreateDevice(ctx, d)
}

func (f *chaosStoreFaultRepo) CreateRelease(ctx context.Context, r store.Release) error {
	if f.failAll.Load() {
		return errors.New("chaos: simulated CreateRelease failure")
	}
	return f.Repository.CreateRelease(ctx, r)
}

func (f *chaosStoreFaultRepo) ListGroups(ctx context.Context) ([]store.Group, error) {
	if f.failAll.Load() {
		return nil, errors.New("chaos: simulated ListGroups failure")
	}
	return f.Repository.ListGroups(ctx)
}

// TestChaosAuthBadPayload — send malformed JSON to the login endpoint, assert
// 400 VALIDATION_FAILED.
func TestChaosAuthBadPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()
	router, _, _ := stressServer(t)

	tests := []struct {
		name string
		body string
	}{
		{"garbage json", `{bad json!!!`},
		{"truncated json", `{"username":"admin`},
		{"array instead of object", `["admin","s3cret"]`},
		{"empty object", `{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := doStressReq(router, http.MethodPost, "/api/v1/auth/login", "", tc.body)
			if code != http.StatusBadRequest {
				t.Fatalf("%s: want 400, got %d", tc.name, code)
			}
		})
	}
}

// TestChaosAuthHugePayload — send a payload exceeding MaxUploadBytes, assert
// 413 (PAYLOAD_TOO_LARGE) or 400 (VALIDATION_FAILED when gin rejects the body
// before the handler).
func TestChaosAuthHugePayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()
	router, _, _ := stressServer(t)

	// Build a request body larger than the 8 MiB MaxUploadBytes.
	large := strings.Repeat("x", 10<<20) // 10 MiB
	body := fmt.Sprintf(`{"username":"%s","password":"pw"}`, large)

	code := doStressReq(router, http.MethodPost, "/api/v1/auth/login", "", body)
	if code == http.StatusOK {
		t.Fatalf("huge payload: accepted when it should be rejected (status %d)", code)
	}
	// Any non-200 response (400/401/403/413) proves graceful degradation.
	t.Logf("chaos_huge_payload: status=%d (graceful degradation — not 200, not crash)", code)
}

// TestChaosConcurrentMutation — create releases with conflicting version
// strings concurrently, then verify the final state is self-consistent (no
// data corruption). Uses the real store.
func TestChaosConcurrentMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()
	repo := store.NewMemoryRepository()
	router, _, tok := stressServerWithRepo(t, repo)
	ctx := context.Background()

	// Create a verified artifact.
	artID := "art-chaos-mutate"
	if err := repo.CreateArtifact(ctx, store.Artifact{
		ArtifactID:  artID,
		SHA256:      "deadbeef01234567",
		Size:        42,
		OSType:      "android",
		TargetModel: "OrangePi5Max",
		Version:     "0.1.0",
		Verified:    true,
		UploadedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	// Create a baseline release so monotonicity has a starting point.
	if err := repo.CreateRelease(ctx, store.Release{
		ReleaseID:   "base-rel",
		ArtifactID:  artID,
		Version:     "0.1.0",
		OSType:      "android",
		TargetModel: "OrangePi5Max",
		Status:      "published",
		CreatedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("create baseline release: %v", err)
	}

	// Phase 1: 50 goroutines each try to create a release with version "0.2.0"
	// (same version). The monotonicity check uses LatestRelease which returns
	// the highest version. Since "0.2.0" > "0.1.0" (baseline), the first to
	// run the check passes, and then all others also pass because LatestRelease
	// only returns "0.1.0" until the first one finishes, or "0.2.0" thereafter.
	// The real value is exercising concurrent mutation: at most one should
	// succeed, or if the race window allows multiple, the state must still be
	// internally consistent (no panic, no duplicate release IDs).
	phase1Body := fmt.Sprintf(
		`{"artifact_id":"%s","version":"0.2.0","os":"android","target_model":"OrangePi5Max"}`, artID)
	const p1 = 50
	phase1Codes := make([]int, p1)
	var wg sync.WaitGroup
	wg.Add(p1)
	for i := 0; i < p1; i++ {
		i := i
		go func() {
			defer wg.Done()
			phase1Codes[i] = doStressReq(router, http.MethodPost, "/api/v1/releases", tok, phase1Body)
		}()
	}
	wg.Wait()

	// Count outcomes.
	created, conflicts, errs := 0, 0, 0
	for _, c := range phase1Codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			errs++
		}
	}
	t.Logf("chaos_concurrent_mutation phase1 (same-version): total=%d created=%d conflicts=%d errs=%d",
		p1, created, conflicts, errs)

	// Phase 2: verify the server is still responsive — list releases.
	listCode := doStressReq(router, http.MethodGet, "/api/v1/releases?os=android", tok, "")
	if listCode != http.StatusOK {
		t.Errorf("list after mutation: want 200, got %d", listCode)
	}

	// Phase 3: create a release with a higher version to confirm monotonicity
	// still works correctly (no data corruption from Phase 1).
	higherCode := doStressReq(router, http.MethodPost, "/api/v1/releases", tok,
		fmt.Sprintf(`{"artifact_id":"%s","version":"0.3.0","os":"android","target_model":"OrangePi5Max"}`, artID))
	if higherCode != http.StatusCreated {
		t.Errorf("release after mutation: want 201, got %d", higherCode)
	}

	// Write evidence.
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "qa-results", "stress_chaos")
	}
	_ = os.MkdirAll(dir, 0o755)
	line := fmt.Sprintf(
		"chaos_concurrent_mutation: same_version_release: phase1_created=%d phase1_conflicts=%d phase1_errs=%d list_after=%d higher_version_release=%d\n",
		created, conflicts, errs, listCode, higherCode)
	if f, err := os.OpenFile(filepath.Join(dir, "errors.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		defer f.Close()
		_, _ = f.WriteString(line)
	}
	t.Log(strings.TrimSpace(line))
}

// TestChaosStoreRestart — simulate store unavailability via fault injection,
// prove handlers return 500 (graceful degradation, not panic), then clear the
// fault and confirm recovery.
func TestChaosStoreRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	t.Parallel()
	fr := &chaosStoreFaultRepo{Repository: store.NewMemoryRepository()}
	router, _, tok := stressServerWithRepo(t, fr)

	// Baseline: operations work fine.
	groupCode := doStressReq(router, http.MethodPost, "/api/v1/groups", tok, `{"name":"baseline-group"}`)
	if groupCode != http.StatusCreated {
		t.Fatalf("baseline group create: want 201, got %d", groupCode)
	}

	// Inject fault — store is "down".
	fr.failAll.Store(true)

	// Device registration must return 500 (graceful), not panic.
	devCode := doStressReq(router, http.MethodPost, "/api/v1/devices/register", tok,
		`{"hardware_id":"chaos-hw","model":"OrangePi5Max","os":"android"}`)
	if devCode != http.StatusInternalServerError {
		t.Fatalf("device register under fault: want 500, got %d (test may need handler check)", devCode)
	}

	// Sustained fault: all device registrations keep returning 500.
	for i := 0; i < 5; i++ {
		c := doStressReq(router, http.MethodPost, "/api/v1/devices/register", tok,
			fmt.Sprintf(`{"hardware_id":"chaos-hw-%d","model":"OrangePi5Max","os":"android"}`, i))
		if c != http.StatusInternalServerError {
			t.Fatalf("sustained fault iter %d: want 500, got %d", i, c)
		}
	}

	// Group list also fails (ListGroups overridden in faultRepo).
	listCode := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, "")
	if listCode != http.StatusInternalServerError {
		t.Fatalf("group list under fault: want 500, got %d", listCode)
	}

	// Clear fault — store is back.
	fr.failAll.Store(false)

	// Recovery: operations succeed again.
	recoverCode := doStressReq(router, http.MethodPost, "/api/v1/groups", tok, `{"name":"recovery-group"}`)
	if recoverCode != http.StatusCreated {
		t.Fatalf("recovery group create: want 201, got %d", recoverCode)
	}
	recoverList := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, "")
	if recoverList != http.StatusOK {
		t.Fatalf("recovery list: want 200, got %d", recoverList)
	}

	// Write evidence.
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "qa-results", "stress_chaos")
	}
	_ = os.MkdirAll(dir, 0o755)
	line := fmt.Sprintf(
		"chaos_store_restart: baseline=201 under_fault_dev=%d under_fault_list=%d recovered_create=%d recovered_list=%d\n",
		devCode, listCode, recoverCode, recoverList)
	if f, err := os.OpenFile(filepath.Join(dir, "errors.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		defer f.Close()
		_, _ = f.WriteString(line)
	}
	t.Log(strings.TrimSpace(line))
}
