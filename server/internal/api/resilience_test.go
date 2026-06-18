package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- §11.4.85 stress + chaos resilience suite (black-box, in-process, no hardware) ---
//
// These exercise the real Server.Router() under sustained load, concurrent
// contention, and injected repository faults, asserting no deadlock/panic, no
// lost updates, graceful 5xx (not crash) under fault, and recovery afterwards.
// Captured latency + error/recovery evidence is written under
// docs/qa/20260608-stress-chaos/.

// faultRepo wraps a Repository and injects failures on demand (chaos). Embedding
// the interface delegates every un-overridden method to the real repo.
type faultRepo struct {
	store.Repository
	failGroups atomic.Bool
}

func (f *faultRepo) ListGroups(ctx context.Context) ([]store.Group, error) {
	if f.failGroups.Load() {
		return nil, errors.New("chaos: injected ListGroups fault")
	}
	return f.Repository.ListGroups(ctx)
}

// newResilienceServer builds a server with a CONCURRENCY-SAFE id generator (the
// shared test env uses a non-atomic counter unsuitable for parallel load).
func newResilienceServer(t testing.TB, repo store.Repository) (*gin.Engine, *Server) {
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
		Repo:   repo,
		Users:  NewStaticUserDirectory(),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	return srv.Router(), srv
}

func resilienceAdminToken(t testing.TB, srv *Server) string {
	t.Helper()
	tok, err := srv.signer.Mint("admin@stress", []string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("mint admin: %v", err)
	}
	return tok
}

func doStressReq(router *gin.Engine, method, path, token, body string) int {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w.Code
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

// writeStressEvidence records the captured latency/error census (§11.4.85 +
// §11.4.5: PASS carries captured evidence).
func writeStressEvidence(t *testing.T, name string, lat []time.Duration, errCount, total int) {
	t.Helper()
	// Runtime stress/chaos evidence goes to a GITIGNORED runs dir so re-runs never
	// mutate the committed 2026-06-08 reference capture (§11.4.11 — runtime logs are
	// not tracked; the historical run.log under 20260608-stress-chaos/ is the
	// committed reference asset). Override with HELIX_STRESS_EVIDENCE_DIR.
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "docs", "qa", "stress-chaos-runs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("evidence dir: %v", err)
		return
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	line := fmt.Sprintf("%s: total=%d errors=%d p50=%s p95=%s p99=%s max=%s\n",
		name, total, errCount, percentile(lat, 50), percentile(lat, 95), percentile(lat, 99),
		func() time.Duration {
			if len(lat) == 0 {
				return 0
			}
			return lat[len(lat)-1]
		}())
	f, err := os.OpenFile(filepath.Join(dir, "run.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Logf("evidence file: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
	t.Log(strings.TrimSpace(line))
}

// TestStressConcurrentGroupCreate: 200 concurrent group creates, unique names ->
// all 201, no deadlock/panic, latency captured.
func TestStressConcurrentGroupCreate(t *testing.T) {
	router, srv := newResilienceServer(t, store.NewMemoryRepository())
	tok := resilienceAdminToken(t, srv)

	const n = 200
	lat := make([]time.Duration, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			start := time.Now()
			codes[i] = doStressReq(router, http.MethodPost, "/api/v1/groups", tok,
				fmt.Sprintf(`{"name":"fleet-%d"}`, i))
			lat[i] = time.Since(start)
		}(i)
	}
	wg.Wait()

	created, errs := 0, 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		} else {
			errs++
		}
	}
	writeStressEvidence(t, "concurrent_group_create", lat, errs, n)
	if created != n {
		t.Fatalf("concurrent group create: want %d created, got %d (errs=%d)", n, created, errs)
	}
}

// TestStressSustainedReads: sustained read load across workers -> 0 unexpected
// errors, latency distribution captured.
func TestStressSustainedReads(t *testing.T) {
	router, srv := newResilienceServer(t, store.NewMemoryRepository())
	tok := resilienceAdminToken(t, srv)
	// Seed a few groups to read.
	for i := 0; i < 5; i++ {
		doStressReq(router, http.MethodPost, "/api/v1/groups", tok, fmt.Sprintf(`{"name":"g-%d"}`, i))
	}

	const workers, perWorker = 16, 150 // 2400 requests
	lat := make([]time.Duration, 0, workers*perWorker)
	var mu sync.Mutex
	var errs int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, perWorker)
			for i := 0; i < perWorker; i++ {
				start := time.Now()
				code := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, "")
				local = append(local, time.Since(start))
				if code != http.StatusOK {
					atomic.AddInt64(&errs, 1)
				}
			}
			mu.Lock()
			lat = append(lat, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	writeStressEvidence(t, "sustained_group_reads", lat, int(errs), workers*perWorker)
	if errs != 0 {
		t.Fatalf("sustained reads: %d unexpected non-200s", errs)
	}
}

// TestStressConcurrentMembershipNoLostUpdates: 60 distinct devices added to ONE
// group concurrently -> final membership has exactly 60 (no lost updates under
// mutex contention).
func TestStressConcurrentMembershipNoLostUpdates(t *testing.T) {
	repo := store.NewMemoryRepository()
	router, srv := newResilienceServer(t, repo)
	tok := resilienceAdminToken(t, srv)

	gw := doStressReq(router, http.MethodPost, "/api/v1/groups", tok, `{"name":"contended"}`)
	if gw != http.StatusCreated {
		t.Fatalf("create group: %d", gw)
	}
	// Resolve the group id + register the devices.
	groups, _ := repo.ListGroups(context.Background())
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
	gid := groups[0].ID
	const n = 60
	for i := 0; i < n; i++ {
		if err := repo.CreateDevice(context.Background(), store.Device{
			DeviceID: fmt.Sprintf("dev-%d", i), HardwareID: fmt.Sprintf("hw-%d", i),
			Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid, RegisteredAt: time.Now(),
		}); err != nil {
			t.Fatalf("register dev-%d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			doStressReq(router, http.MethodPost, "/api/v1/groups/"+gid+"/members", tok,
				fmt.Sprintf(`{"device_ids":["dev-%d"]}`, i))
		}(i)
	}
	wg.Wait()

	members, err := repo.ListGroupMembers(context.Background(), gid)
	if err != nil || len(members) != n {
		t.Fatalf("concurrent membership: want %d members, got %d err=%v", n, len(members), err)
	}
}

// TestDDoSFloodStaysUpAndRecovers (§11.4.27 ddos type): fire a large abusive
// burst at the server and assert it (a) processes every request without
// panic/hang, and (b) is fully responsive immediately after. HONEST FINDING
// captured by this test: the MVP has NO rate-limiting — every request is served
// (no 429s), so the server's only protection under flood is the host + Go's
// scheduler. Recommendation (tracked): add a rate-limit / concurrency-cap
// middleware before public exposure. This test verifies graceful-under-flood +
// recovery; it does NOT assert load-shedding (there is none yet — documenting
// the gap honestly rather than faking a 429).
func TestDDoSFloodStaysUpAndRecovers(t *testing.T) {
	router, srv := newResilienceServer(t, store.NewMemoryRepository())
	tok := resilienceAdminToken(t, srv)

	const total, workers = 6000, 64
	var done, non200, sheds int64 // sheds = 429s, expected 0 today (no rate-limiter)
	var wg sync.WaitGroup
	per := total / workers
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				code := doStressReq(router, http.MethodGet, "/healthz", "", "")
				atomic.AddInt64(&done, 1)
				switch {
				case code == http.StatusTooManyRequests:
					atomic.AddInt64(&sheds, 1)
				case code != http.StatusOK:
					atomic.AddInt64(&non200, 1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// (a) every request completed, none errored unexpectedly (429s are not errors).
	if done != int64(workers*per) {
		t.Fatalf("flood: only %d/%d requests completed", done, workers*per)
	}
	if non200 != 0 {
		t.Fatalf("flood: %d unexpected non-200/non-429 responses", non200)
	}
	// (b) the server is immediately responsive after the burst (recovery), and an
	// authenticated path still works (auth path not starved).
	if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
		t.Fatalf("post-flood healthz want 200, got %d", code)
	}
	if code := doStressReq(router, http.MethodPost, "/api/v1/groups", tok, `{"name":"post-flood"}`); code != http.StatusCreated {
		t.Fatalf("post-flood authed write want 201, got %d", code)
	}

	// Runtime stress/chaos evidence goes to a GITIGNORED runs dir so re-runs never
	// mutate the committed 2026-06-08 reference capture (§11.4.11 — runtime logs are
	// not tracked; the historical run.log under 20260608-stress-chaos/ is the
	// committed reference asset). Override with HELIX_STRESS_EVIDENCE_DIR.
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "docs", "qa", "stress-chaos-runs")
	}
	_ = os.MkdirAll(dir, 0o755)
	if f, err := os.OpenFile(filepath.Join(dir, "run.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		defer f.Close()
		_, _ = f.WriteString(fmt.Sprintf(
			"ddos_flood: %d reqs / %d workers in %s, served=%d shed(429)=%d non200=%d, post-flood 200+201 OK; FINDING: no rate-limiter (recommend adding one)\n",
			workers*per, workers, elapsed, done-sheds-non200, sheds, non200))
	}
	t.Logf("ddos_flood: %d reqs served (0 shed — no rate-limiter today), server responsive post-flood", workers*per)
}

// TestChaosRepoFaultDegradesAndRecovers: inject a repo fault -> GET /groups
// returns 500 (graceful, no panic); clear the fault -> 200 (recovery). Captures
// the recovery transition.
func TestChaosRepoFaultDegradesAndRecovers(t *testing.T) {
	fr := &faultRepo{Repository: store.NewMemoryRepository()}
	router, srv := newResilienceServer(t, fr)
	tok := resilienceAdminToken(t, srv)

	if code := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, ""); code != http.StatusOK {
		t.Fatalf("baseline GET /groups want 200, got %d", code)
	}
	// Inject fault.
	fr.failGroups.Store(true)
	faultCode := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, "")
	if faultCode != http.StatusInternalServerError {
		t.Fatalf("under fault want 500 (graceful), got %d", faultCode)
	}
	// Under sustained fault, the server must keep responding 500 (not panic/hang).
	for i := 0; i < 50; i++ {
		if c := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, ""); c != http.StatusInternalServerError {
			t.Fatalf("sustained fault iter %d want 500, got %d", i, c)
		}
	}
	// Clear fault -> recovery.
	fr.failGroups.Store(false)
	if code := doStressReq(router, http.MethodGet, "/api/v1/groups", tok, ""); code != http.StatusOK {
		t.Fatalf("after fault cleared want 200 (recovery), got %d", code)
	}

	// Runtime stress/chaos evidence goes to a GITIGNORED runs dir so re-runs never
	// mutate the committed 2026-06-08 reference capture (§11.4.11 — runtime logs are
	// not tracked; the historical run.log under 20260608-stress-chaos/ is the
	// committed reference asset). Override with HELIX_STRESS_EVIDENCE_DIR.
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "docs", "qa", "stress-chaos-runs")
	}
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "run.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		defer f.Close()
		_, _ = f.WriteString("chaos_repo_fault: baseline=200 under_fault=500 sustained_fault=50x500 recovered=200\n")
	}
	t.Log("chaos_repo_fault: 200 -> 500 (graceful) -> 50x500 -> 200 (recovered)")
}
