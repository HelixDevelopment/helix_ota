// Package server_stress_test -- stress tests for the helix_ota server (§11.4.85).
//
// Exercises the real Server.Router() under sustained concurrent load against
// auth, group, release, and device endpoints, recording per-iteration latency
// distribution as captured evidence.  Uses the real in-memory store (no mocks
// per §11.4.27).
package server_stress_test

import (
	"context"
	"encoding/json"
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

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func evidenceDir() string {
	if d := os.Getenv("HELIX_STRESS_EVIDENCE_DIR"); d != "" {
		return d
	}
	return "qa-results/stress_chaos"
}

func writeEvidenceJSON(t *testing.T, name string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("WARNING: marshal evidence: %v", err)
		return
	}
	dir := evidenceDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("WARNING: mkdir evidence dir %s: %v", dir, err)
		return
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.json", name, ts))
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Logf("WARNING: write evidence %s: %v", p, err)
	}
}

func percentiles(durations []time.Duration) (p50, p95, p99 time.Duration) {
	n := len(durations)
	if n == 0 {
		return 0, 0, 0
	}
	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p50 = sorted[n*50/100]
	p95 = sorted[n*95/100]
	p99 = sorted[n*99/100]
	return
}

// login retrieves an admin token via the login endpoint.
func login(router *gin.Engine) string {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"username":"admin@helix.test","password":"s3cret"}`))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		return ""
	}
	var resp struct {
		Token string `json:"access_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token
}

// testRouter builds a concurrency-safe test server and returns its router.
func testRouter(t testing.TB) *gin.Engine {
	t.Helper()
	var ctr int64
	srv := api.NewServer(api.Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: time.Hour,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("stress-test-secret"),
		},
		Repo: store.NewMemoryRepository(),
		Users: api.NewStaticUserDirectory(api.StaticUser{
			Username: "admin@helix.test",
			Password: "s3cret",
			Roles:    []string{api.RoleAdmin, api.RoleOperator, api.RoleViewer},
		}),
		Health:  health.New(func(context.Context) bool { return true }),
		Now:     time.Now,
		NewID:   func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
		Rollout: nil,
	})
	return srv.Router()
}

// doReq executes an HTTP request against the router and returns the status code.
func doReq(router *gin.Engine, method, path, token, body string) int {
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

// ---------------------------------------------------------------------------
// TestStressSustainedGroupCreate
//
// N=200 consecutive group creates, recording per-iteration latency.
// ---------------------------------------------------------------------------

func TestStressSustainedGroupCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in short mode")
	}
	router := testRouter(t)
	tok := login(router)
	if tok == "" {
		t.Fatal("failed to obtain admin token")
	}

	const N = 200
	durations := make([]time.Duration, N)
	var failed int32

	for i := 0; i < N; i++ {
		start := time.Now()
		code := doReq(router, http.MethodPost, "/api/v1/groups", tok,
			fmt.Sprintf(`{"name":"group-%d"}`, i))
		durations[i] = time.Since(start)
		if code != http.StatusCreated {
			atomic.AddInt32(&failed, 1)
		}
	}

	p50, p95, p99 := percentiles(durations)
	record := map[string]interface{}{
		"test":    "TestStressSustainedGroupCreate",
		"N":       N,
		"failed":  failed,
		"p50_ns":  p50.Nanoseconds(),
		"p95_ns":  p95.Nanoseconds(),
		"p99_ns":  p99.Nanoseconds(),
	}
	writeEvidenceJSON(t, "sustained_group_create", record)
	t.Logf("Sustained group create N=%d: failed=%d p50=%v p95=%v p99=%v",
		N, failed, p50, p95, p99)
	if failed != 0 {
		t.Fatalf("%d group creates failed", failed)
	}
}

// ---------------------------------------------------------------------------
// TestStressConcurrentAuth
//
// 50 concurrent login requests, all must return 200.
// ---------------------------------------------------------------------------

func TestStressConcurrentAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in short mode")
	}
	router := testRouter(t)

	const n = 50
	durations := make([]time.Duration, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			start := time.Now()
			codes[i] = doReq(router, http.MethodPost, "/api/v1/auth/login", "",
				`{"username":"admin@helix.test","password":"s3cret"}`)
			durations[i] = time.Since(start)
		}()
	}
	wg.Wait()

	var failed int
	for _, c := range codes {
		if c != http.StatusOK {
			failed++
		}
	}
	p50, p95, p99 := percentiles(durations)
	record := map[string]interface{}{
		"test":   "TestStressConcurrentAuth",
		"n":      n,
		"failed": failed,
		"p50_ns": p50.Nanoseconds(),
		"p95_ns": p95.Nanoseconds(),
		"p99_ns": p99.Nanoseconds(),
	}
	writeEvidenceJSON(t, "concurrent_auth", record)
	t.Logf("Concurrent auth N=%d: failed=%d p50=%v p95=%v p99=%v", n, failed, p50, p95, p99)
	if failed != 0 {
		t.Fatalf("%d auth requests failed", failed)
	}
}

// ---------------------------------------------------------------------------
// TestStressConcurrentDeviceRegistration
//
// 50 concurrent device registrations, all must return 201.
// ---------------------------------------------------------------------------

func TestStressConcurrentDeviceRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in short mode")
	}
	router := testRouter(t)
	tok := login(router)
	if tok == "" {
		t.Fatal("failed to obtain admin token")
	}

	const n = 50
	durations := make([]time.Duration, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			start := time.Now()
			codes[i] = doReq(router, http.MethodPost, "/api/v1/devices/register", tok,
				fmt.Sprintf(`{"hardware_id":"hw-%d","model":"OrangePi5Max","os":"android"}`, i))
			durations[i] = time.Since(start)
		}()
	}
	wg.Wait()

	var failed int
	for _, c := range codes {
		if c != http.StatusCreated {
			failed++
		}
	}
	p50, p95, p99 := percentiles(durations)
	record := map[string]interface{}{
		"test":   "TestStressConcurrentDeviceRegistration",
		"n":      n,
		"failed": failed,
		"p50_ns": p50.Nanoseconds(),
		"p95_ns": p95.Nanoseconds(),
		"p99_ns": p99.Nanoseconds(),
	}
	writeEvidenceJSON(t, "concurrent_device_reg", record)
	t.Logf("Concurrent device reg N=%d: failed=%d p50=%v p95=%v p99=%v", n, failed, p50, p95, p99)
	if failed != 0 {
		t.Fatalf("%d device registrations failed", failed)
	}
}

// ---------------------------------------------------------------------------
// TestStressBoundaryAuthPayloads
//
// Boundary conditions for login: empty username, empty password, very long
// values. Must all gracefully reject (no panic, no 200).
// ---------------------------------------------------------------------------

func TestStressBoundaryAuthPayloads(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in short mode")
	}
	router := testRouter(t)

	type bc struct {
		name string
		body string
	}
	cases := []bc{
		{"empty-username", `{"username":"","password":"s3cret"}`},
		{"empty-password", `{"username":"admin@helix.test","password":""}`},
		{"both-empty", `{"username":"","password":""}`},
		{"huge-username", fmt.Sprintf(`{"username":"%s","password":"s3cret"}`, string(make([]byte, 10000)))},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			code := doReq(router, http.MethodPost, "/api/v1/auth/login", "", c.body)
			if code == http.StatusOK {
				t.Fatalf("%s: login succeeded with boundary input", c.name)
			}
		})
	}
}
