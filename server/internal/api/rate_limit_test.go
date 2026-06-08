package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
	"github.com/gin-gonic/gin"
)

// newCappedServer builds a server with an in-flight cap (the §11.4.27-ddos
// finding's recommended protection) for the shedding test.
func newCappedServer(t testing.TB, limit int64) *gin.Engine {
	t.Helper()
	var ctr int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath: "/api/v1", AccessTokenTTL: time.Hour, DeviceTokenTTL: 24 * time.Hour,
			TokenSecret: []byte("cap-secret"), MaxInflight: limit,
		},
		Repo:   store.NewMemoryRepository(),
		Users:  NewStaticUserDirectory(),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	return srv.Router()
}

// TestMaxInflightShedsUnderFlood: with the cap ENABLED, an abusive concurrent
// burst is partly shed with 429 (load-shedding works), at least one request is
// served, no other status appears, and the server is responsive immediately
// after — turning the DDoS finding's recommendation into a proven control.
func TestMaxInflightShedsUnderFlood(t *testing.T) {
	router := newCappedServer(t, 1) // cap=1 makes shedding deterministic under burst

	const n = 300
	var served, shed, other int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			switch doStressReq(router, http.MethodGet, "/healthz", "", "") {
			case http.StatusOK:
				atomic.AddInt64(&served, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&shed, 1)
			default:
				atomic.AddInt64(&other, 1)
			}
		}()
	}
	wg.Wait()

	if other != 0 {
		t.Fatalf("flood produced %d unexpected statuses (want only 200/429)", other)
	}
	if served+shed != n {
		t.Fatalf("accounting: served=%d shed=%d != %d", served, shed, n)
	}
	if shed == 0 {
		t.Fatalf("cap=1 under %d concurrent requests must shed some with 429, shed=0", n)
	}
	if served == 0 {
		t.Fatalf("at least one request must be served, served=0")
	}
	// Recovery: once the burst drains, a normal request succeeds.
	if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
		t.Fatalf("post-flood healthz want 200, got %d", code)
	}
	t.Logf("max-inflight cap=1: served=%d shed(429)=%d of %d; responsive post-flood", served, shed, n)
}

// TestMaxInflightDisabledByDefault: a zero/absent limit is a no-op — no shedding,
// preserving existing behaviour (the cap is strictly opt-in).
func TestMaxInflightDisabledByDefault(t *testing.T) {
	router := newCappedServer(t, 0)
	const n = 200
	var shed int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if doStressReq(router, http.MethodGet, "/healthz", "", "") == http.StatusTooManyRequests {
				atomic.AddInt64(&shed, 1)
			}
		}()
	}
	wg.Wait()
	if shed != 0 {
		t.Fatalf("cap disabled must never shed, got %d 429s", shed)
	}
}
