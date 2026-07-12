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

func TestMaxInflightShedsUnderFlood(t *testing.T) {
	router := newCappedServer(t, 1)
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
		t.Fatalf("flood produced %d unexpected statuses", other)
	}
	if shed == 0 {
		t.Fatalf("cap=1 must shed some with 429, shed=0")
	}
	if served == 0 {
		t.Fatalf("at least one request must be served")
	}
}

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
