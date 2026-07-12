package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		t.Fatalf("unexpected statuses: %d", other)
	}
	if shed == 0 {
		t.Fatal("cap=1 must shed some with 429")
	}
	if served == 0 {
		t.Fatal("at least one request must be served")
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

func TestTokenBucketAllowAndRefill(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	tb := newTokenBucket(10.0, 10.0, now)
	for i := 0; i < 10; i++ {
		if !tb.allow(now) {
			t.Fatalf("token %d: expected allow=true", i+1)
		}
	}
	if tb.allow(now) {
		t.Fatal("11th token should be denied")
	}
	future := now.Add(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if !tb.allow(future) {
			t.Fatalf("refilled token %d: expected allow=true", i+1)
		}
	}
	if tb.allow(future) {
		t.Fatal("6th refilled token should be denied")
	}
}

func TestTokenBucketCapacityCap(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	tb := newTokenBucket(100.0, 5.0, now)
	for i := 0; i < 5; i++ {
		if !tb.allow(now) {
			t.Fatalf("token %d: expected allow=true", i+1)
		}
	}
	if tb.allow(now) {
		t.Fatal("6th token should be denied")
	}
	future := now.Add(1 * time.Second)
	for i := 0; i < 5; i++ {
		if !tb.allow(future) {
			t.Fatalf("capped refill token %d: expected allow=true", i+1)
		}
	}
	if tb.allow(future) {
		t.Fatal("tokens should be capped")
	}
}

func newRateLimitServer(t testing.TB, rps int) *gin.Engine {
	t.Helper()
	var ctr int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath: "/api/v1", AccessTokenTTL: time.Hour, DeviceTokenTTL: 24 * time.Hour,
			TokenSecret:  []byte("rl-secret"),
			RateLimitRPS: rps,
		},
		Repo:   store.NewMemoryRepository(),
		Users:  NewStaticUserDirectory(),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	return srv.Router()
}

func TestRateLimitBlocksWhenExceeded(t *testing.T) {
	router := newRateLimitServer(t, 1)
	if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
		t.Fatalf("first request: want 200, got %d", code)
	}
	shed := 0
	for i := 0; i < 10; i++ {
		if doStressReq(router, http.MethodGet, "/healthz", "", "") == http.StatusTooManyRequests {
			shed++
		}
	}
	if shed == 0 {
		t.Fatal("RED: burst under rate=1 must produce at least one 429")
	}
}

func TestRateLimitAllowsBelowLimit(t *testing.T) {
	router := newRateLimitServer(t, 100)
	if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
		t.Fatalf("GREEN: want 200, got %d", code)
	}
}

func TestRateLimitZeroBlocksAll_AntiTautologyRED(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	rl := NewIPRateLimiter(0.0, 1.0)
	defer rl.Stop()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if !rl.Allow(c.ClientIP(), now) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{"code": CodeRateLimited, "message": "rate limit exceeded"},
			})
			return
		}
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request with token: want 200, got %d", w.Code)
	}
	const N = 20
	for i := 0; i < N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("RED: zero refill request %d: want 429, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitNormalAllows_AntiTautologyGREEN(t *testing.T) {
	router := newRateLimitServer(t, 100)
	const N = 10
	for i := 0; i < N; i++ {
		if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
			t.Fatalf("GREEN: rate=100 request %d: want 200, got %d", i+1, code)
		}
	}
}

func TestRateLimit429CarriesRetryAfter(t *testing.T) {
	router := newRateLimitServer(t, 1)
	doStressReq(router, http.MethodGet, "/healthz", "", "")
	w := doStressReqWithHeaders(router, http.MethodGet, "/healthz", "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestRateLimitDisabledByDefault(t *testing.T) {
	router := newRateLimitServer(t, 0)
	for i := 0; i < 10; i++ {
		if code := doStressReq(router, http.MethodGet, "/healthz", "", ""); code != http.StatusOK {
			t.Fatalf("disabled: all should pass, got %d on request %d", code, i+1)
		}
	}
}

func newAuthRateLimitServer(t testing.TB, rpm int) *gin.Engine {
	t.Helper()
	var ctr int64
	repo := store.NewMemoryRepository()
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath:    "/api/v1", AccessTokenTTL: time.Hour, DeviceTokenTTL: 24 * time.Hour,
			TokenSecret:    []byte("auth-rl-secret"),
			AuthRateLimit:  rpm,
			RateLimitRPS:   100,
		},
		Repo:  repo,
		Users: NewStaticUserDirectory(StaticUser{
			Username: "admin@helix.test", Password: "s3cret",
			Roles: []string{RoleAdmin, RoleOperator, RoleViewer},
		}),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    func() time.Time { return time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) },
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	return srv.Router()
}

func TestAuthRateLimitBlocksBruteForce(t *testing.T) {
	router := newAuthRateLimitServer(t, 5)
	for i := 0; i < 5; i++ {
		code := doStressReq(router, http.MethodPost, "/api/v1/auth/login",
			"", `{"username":"admin@helix.test","password":"s3cret"}`)
		if code == http.StatusTooManyRequests {
			t.Fatalf("login %d: should allow within 5/min", i+1)
		}
	}
	code := doStressReq(router, http.MethodPost, "/api/v1/auth/login",
		"", `{"username":"admin@helix.test","password":"s3cret"}`)
	if code != http.StatusTooManyRequests {
		t.Fatalf("RED: 6th login must be 429, got %d", code)
	}
}

func TestAuthRateLimit429RetryAfter(t *testing.T) {
	router := newAuthRateLimitServer(t, 5)
	for i := 0; i < 5; i++ {
		doStressReq(router, http.MethodPost, "/api/v1/auth/login",
			"", `{"username":"admin@helix.test","password":"s3cret"}`)
	}
	w := doStressReqWithHeaders(router, http.MethodPost, "/api/v1/auth/login",
		`{"username":"admin@helix.test","password":"s3cret"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("auth 429 must carry Retry-After")
	}
}

func TestAuthRateLimitDisabled(t *testing.T) {
	router := newAuthRateLimitServer(t, 0)
	for i := 0; i < 20; i++ {
		code := doStressReq(router, http.MethodPost, "/api/v1/auth/login",
			"", `{"username":"admin@helix.test","password":"s3cret"}`)
		if code == http.StatusTooManyRequests {
			t.Fatalf("disabled should never get 429, got on attempt %d", i+1)
		}
	}
}

func TestIPRateLimiterZeroRate_AntiTautologyRED(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	rl := NewIPRateLimiter(0.0, 0.0)
	defer rl.Stop()
	const N = 50
	for i := 0; i < N; i++ {
		if rl.Allow("192.168.1.1", now) {
			t.Fatalf("RED: rate=0 must block ALL; iter %d returned true", i+1)
		}
	}
}

func TestIPRateLimiterNormalRate_AntiTautologyGREEN(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	rl := NewIPRateLimiter(100.0, 100.0)
	defer rl.Stop()
	for i := 0; i < 10; i++ {
		if !rl.Allow("192.168.1.1", now) {
			t.Fatalf("GREEN: rate=100 must allow; iter %d returned false", i+1)
		}
	}
}

func doStressReqWithHeaders(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	router.ServeHTTP(w, r)
	return w
}
