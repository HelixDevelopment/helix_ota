package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func maxInflightMiddleware(limit int64) gin.HandlerFunc {
	// Applied globally in Router() via compositionMiddleware chain — covers every
	// endpoint including artifact download (handleGetArtifact, handleClientUpdate)
	// and artifact upload (handleUploadArtifact). The HELIX_MAX_INFLIGHT env var
	// (default 1000) protects against connection-flood DoS by shedding excess
	// requests with 429 RATE_LIMITED. Set to 0 to disable.
	if limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	sem := make(chan struct{}, limit)
	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			respondError(c, http.StatusTooManyRequests, CodeRateLimited,
				"server at capacity ("+strconv.FormatInt(limit, 10)+" in-flight); retry shortly")
			c.Abort()
		}
	}
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	rate       float64
	capacity   float64
	mu         sync.Mutex
}

func newTokenBucket(rate, capacity float64, now time.Time) *tokenBucket {
	return &tokenBucket{tokens: capacity, lastRefill: now, rate: rate, capacity: capacity}
}

func (tb *tokenBucket) allow(now time.Time) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed > 0 {
		tb.tokens += elapsed * tb.rate
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
	}
	tb.lastRefill = now
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

type IPRateLimiter struct {
	buckets  map[string]*tokenBucket
	rate     float64
	capacity float64
	mu       sync.Mutex
	done     chan struct{}
}

func NewIPRateLimiter(rate, capacity float64) *IPRateLimiter {
	rl := &IPRateLimiter{
		buckets: make(map[string]*tokenBucket), rate: rate, capacity: capacity,
		done: make(chan struct{}),
	}
	go rl.reapLoop()
	return rl
}

func (rl *IPRateLimiter) Allow(ip string, now time.Time) bool {
	rl.mu.Lock()
	tb, ok := rl.buckets[ip]
	if !ok {
		tb = newTokenBucket(rl.rate, rl.capacity, now)
		rl.buckets[ip] = tb
	}
	rl.mu.Unlock()
	return tb.allow(now)
}

func (rl *IPRateLimiter) Stop() { close(rl.done) }

func (rl *IPRateLimiter) reapLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.reap()
		case <-rl.done:
			return
		}
	}
}

func (rl *IPRateLimiter) reap() {
	cutoff := time.Now().Add(-10 * time.Minute)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, tb := range rl.buckets {
		tb.mu.Lock()
		if tb.lastRefill.Before(cutoff) {
			delete(rl.buckets, ip)
		}
		tb.mu.Unlock()
	}
}

func rateLimitMiddleware(rps int) gin.HandlerFunc {
	if rps <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	rl := NewIPRateLimiter(float64(rps), float64(rps))
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.Allow(ip, time.Now()) {
			c.Header("Retry-After", "1")
			respondError(c, http.StatusTooManyRequests, CodeRateLimited,
				fmt.Sprintf("rate limit exceeded (%d req/s per IP); retry shortly", rps))
			c.Abort()
			return
		}
		c.Next()
	}
}

func authRateLimitMiddleware(rpm int) gin.HandlerFunc {
	if rpm <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	rate := float64(rpm) / 60.0
	rl := NewIPRateLimiter(rate, float64(rpm))
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.Allow(ip, time.Now()) {
			c.Header("Retry-After", "60")
			respondError(c, http.StatusTooManyRequests, CodeRateLimited,
				fmt.Sprintf("too many login attempts (%d/min per IP); retry after %d seconds",
					rpm, int(math.Ceil(60.0/float64(rpm)))))
			c.Abort()
			return
		}
		c.Next()
	}
}
