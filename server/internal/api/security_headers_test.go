package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// Item O — security response headers. Most of these tests exercise the REAL
// wired router (Server.Router() -> the global chain, the API group, and
// MountManagerUI), not a hand-rolled stand-in, so a PASS proves the headers
// reach real responses. Two are hand-composed minimal chains kept as fast
// isolation smoke checks (TestSecurityHeaders_TierA_OnShed429); their
// real-router siblings (_RealRouter suffix) are the SR-review I2/I3 gap
// closures — see docs/qa/20260710-server-header-test-seams/EVIDENCE.md.
// Evidence: docs/qa/20260710-server-security-headers/EVIDENCE.md.

// tierAExpect is the Tier-A header set every response must carry (HSTS excepted —
// it is TLS-gated and asserted separately).
var tierAExpect = map[string]string{
	"X-Content-Type-Options":            "nosniff",
	"X-Frame-Options":                   "DENY",
	"Referrer-Policy":                   "no-referrer",
	"Cross-Origin-Opener-Policy":        "same-origin",
	"Cross-Origin-Resource-Policy":      "same-origin",
	"Permissions-Policy":                permissionsPolicyDenyAll,
	"X-Permitted-Cross-Domain-Policies": "none",
}

// assertTierA fails the test if any Tier-A header is missing or wrong.
func assertTierA(t *testing.T, where string, h http.Header) {
	t.Helper()
	for name, want := range tierAExpect {
		if got := h.Get(name); got != want {
			t.Errorf("%s: header %q = %q, want %q", where, name, got, want)
		}
	}
}

// newSecHeadersRouter builds a real server router with the given TLS config so
// the HSTS TLS-gating can be exercised both ways.
func newSecHeadersRouter(t testing.TB, tlsCert, tlsKey string) *gin.Engine {
	t.Helper()
	return newSecHeadersRouterTrustProxy(t, tlsCert, tlsKey, false)
}

// newSecHeadersRouterTrustProxy builds a real server router with the given TLS
// config AND TrustTLSProxy setting, so the I1 config-gated trusted-proxy HSTS
// path (docs/qa/20260710-i1-hsts-trust-proxy/EVIDENCE.md) can be exercised
// independently of local cert/key configuration.
func newSecHeadersRouterTrustProxy(t testing.TB, tlsCert, tlsKey string, trustProxy bool) *gin.Engine {
	t.Helper()
	var ctr int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath: "/api/v1", AccessTokenTTL: time.Hour, DeviceTokenTTL: 24 * time.Hour,
			TokenSecret: []byte("sec-headers-secret"),
			TLSCertFile: tlsCert, TLSKeyFile: tlsKey,
			TrustTLSProxy: trustProxy,
		},
		Repo:   store.NewMemoryRepository(),
		Users:  NewStaticUserDirectory(),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	return srv.Router()
}

// getReq runs a GET against a router and returns the recorder.
func getReq(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// TestSecurityHeaders_TierA_OnEveryResponseClass asserts the global Tier-A
// headers are present on a 200 probe, a 404, a 200 API response, and a 401 API
// error — the "every response" contract, including error paths.
func TestSecurityHeaders_TierA_OnEveryResponseClass(t *testing.T) {
	env := newTestEnv(t)

	// 200 operational probe.
	assertTierA(t, "GET /healthz (200)", getReq(env.router, "/healthz").Header())

	// 404 (NoRoute) — global middleware still runs.
	w404 := getReq(env.router, "/definitely-not-a-route")
	if w404.Code != http.StatusNotFound {
		t.Fatalf("GET unknown route = %d, want 404", w404.Code)
	}
	assertTierA(t, "GET unknown (404)", w404.Header())

	// 401 API error path (protected route, no token).
	w401 := getReq(env.router, "/api/v1/devices")
	if w401.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/devices (no token) = %d, want 401", w401.Code)
	}
	assertTierA(t, "GET /api/v1/devices (401)", w401.Header())

	// 200 API success path (login).
	w200 := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "admin@helix.test", "password": "s3cret",
	})
	if w200.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/auth/login = %d, want 200", w200.Code)
	}
	assertTierA(t, "POST /api/v1/auth/login (200)", w200.Header())
}

// TestSecurityHeaders_TierA_OnShed429 proves the Tier-A headers survive the
// in-flight-limiter 429 shed on a hand-composed two-middleware chain (kept as
// a fast, minimal-dependency smoke check of the ordering property in
// isolation). It does NOT exercise the real Server.Router() wiring — see
// TestSecurityHeaders_TierA_OnShed429_RealRouter below for the
// production-wiring proof (SR-review I2).
func TestSecurityHeaders_TierA_OnShed429(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	r := gin.New()
	r.Use(securityHeadersMiddleware(false), maxInflightMiddleware(1))
	r.GET("/block", func(c *gin.Context) {
		started <- struct{}{}
		<-release
		c.String(http.StatusOK, "ok")
	})

	// Request 1 enters the handler and holds the single in-flight slot.
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/block", nil))
	}()
	<-started

	// Request 2 is shed with 429 before reaching the handler.
	shed := getReq(r, "/block")
	if shed.Code != http.StatusTooManyRequests {
		t.Fatalf("second concurrent request = %d, want 429 (shed)", shed.Code)
	}
	assertTierA(t, "429 shed", shed.Header())
	if ra := shed.Header().Get("Retry-After"); ra != "1" {
		t.Errorf("429 shed Retry-After = %q, want 1", ra)
	}
}

// TestSecurityHeaders_TierA_OnShed429_RealRouter closes SR-review I2: it
// proves the Tier-A headers survive an in-flight-limiter 429 shed against the
// REAL production wiring — Server.Router() built with the actual
// r.Use(recoveryMiddleware(), requestIDMiddleware(), securityHeadersMiddleware(...),
// maxInflightMiddleware(...), compressionMiddleware()) chain from server.go,
// with Config.MaxInflight set so the shed is real (not the newTestEnv default
// of 0, under which maxInflightMiddleware is a permanent no-op; config.Load()
// defaults to 1000 with the same effect).
//
// No production source file is touched to make this deterministic. gin's
// RouterGroup.handle (routergroup.go:88-90) calls combineHandlers
// (routergroup.go:241-248), which concatenates the group's CURRENTLY
// -REGISTERED Handlers slice — already populated by Router()'s r.Use(...)
// call, in the exact production order — with whatever handler is attached to
// the group next. So a GET route attached directly to the *gin.Engine
// returned by srv.Router() (this test, after Router() has already run its
// Use() call) inherits the identical middleware chain every real production
// route gets. This is the "test-only seam": a test-file-only route
// registered via gin's own public API on the already-built engine, not a
// change to server.go — so a future reordering of r.Use(...) in server.go
// changes what THIS test exercises too, closing the I2 gap.
func TestSecurityHeaders_TierA_OnShed429_RealRouter(t *testing.T) {
	var ctr int64
	srv := NewServer(Options{
		Config: config.Config{
			APIBasePath: "/api/v1", AccessTokenTTL: time.Hour, DeviceTokenTTL: 24 * time.Hour,
			TokenSecret: []byte("sec-headers-secret"),
			MaxInflight: 1, // real shed (newTestEnv defaults 0=no-op; Load() defaults 1000).
		},
		Repo:   store.NewMemoryRepository(),
		Users:  NewStaticUserDirectory(),
		Health: health.New(func(context.Context) bool { return true }),
		Now:    time.Now,
		NewID:  func() string { return fmt.Sprintf("id-%d", atomic.AddInt64(&ctr, 1)) },
	})
	r := srv.Router()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	// Test-only blocking route attached to the real, already-built engine (see
	// doc comment above) — inherits the production chain, not a hand-rolled one.
	r.GET("/__red_review_i2_block", func(c *gin.Context) {
		started <- struct{}{}
		<-release
		c.String(http.StatusOK, "ok")
	})

	// Request 1 enters the handler and holds the single in-flight slot.
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/__red_review_i2_block", nil))
	}()
	<-started

	// Request 2 is shed with 429 before reaching the handler, via the REAL
	// production maxInflightMiddleware(1) wired by Router().
	shed := getReq(r, "/__red_review_i2_block")
	if shed.Code != http.StatusTooManyRequests {
		t.Fatalf("second concurrent request (real router) = %d, want 429 (shed)", shed.Code)
	}
	assertTierA(t, "429 shed (real Server.Router())", shed.Header())
	if ra := shed.Header().Get("Retry-After"); ra != "1" {
		t.Errorf("429 shed (real router) Retry-After = %q, want 1", ra)
	}
}

// TestSecurityHeaders_TierA_OnPanic500_RealRouter closes SR-review I3: it
// proves the Tier-A headers survive a genuine panic -> recoveryMiddleware ->
// 500 response against the REAL production wiring (Server.Router()), not a
// hand-rolled chain. Uses the identical test-only-route technique as
// TestSecurityHeaders_TierA_OnShed429_RealRouter above (see that test's doc
// comment for why this exercises the production middleware chain and is not
// a change to server.go): recoveryMiddleware's deferred recover() sits
// outermost in the real chain (registered first in server.go's r.Use(...)),
// so it catches a panic from any handler registered afterward — including one
// attached directly to the already-built engine — exactly as it would a panic
// in a real production handler.
func TestSecurityHeaders_TierA_OnPanic500_RealRouter(t *testing.T) {
	r := newSecHeadersRouter(t, "", "") // plain HTTP; TLS-gating is irrelevant here.
	r.GET("/__red_review_i3_panic", func(c *gin.Context) {
		panic("deliberate test panic to exercise recoveryMiddleware")
	})

	w := getReq(r, "/__red_review_i3_panic")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET /__red_review_i3_panic = %d, want 500 (recovered panic)", w.Code)
	}
	assertTierA(t, "panic -> 500 (real Server.Router())", w.Header())
}

// TestSecurityHeaders_HSTS_TLSGated proves HSTS is present ONLY when TLS is
// configured and ABSENT otherwise (the §1.3.4 gating from the brief).
func TestSecurityHeaders_HSTS_TLSGated(t *testing.T) {
	// Plain-HTTP server: no HSTS.
	plain := newSecHeadersRouter(t, "", "")
	if got := getReq(plain, "/healthz").Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("plain HTTP: HSTS = %q, want absent (empty)", got)
	}

	// TLS-configured server: HSTS present with the expected policy.
	tlsR := newSecHeadersRouter(t, "/tmp/cert.pem", "/tmp/key.pem")
	if got := getReq(tlsR, "/healthz").Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Errorf("TLS configured: HSTS = %q, want %q", got, hstsValue)
	}
}

// TestSecurityHeaders_HSTS_TrustTLSProxy closes the I1 security-review finding
// (docs/research/security_headers_adversarial_review_20260710/): in the common
// production topology the control plane sits BEHIND a trusted TLS-terminating
// reverse proxy (nginx / cloud LB) and receives plain HTTP, so no local
// cert/key is ever configured on the app itself. Without a config-gated
// trust flag, tlsEnabled() was cert/key-only and HSTS was silently NEVER sent
// even though the real client<->proxy connection is genuinely HTTPS. Proves
// all three cases against the REAL router (srv.Router()):
//
//	(a) default (TrustTLSProxy unset/false) + no local TLS -> NO HSTS
//	    (current/original behavior preserved byte-for-byte);
//	(b) TrustTLSProxy=true + no local TLS -> HSTS IS emitted (the fix);
//	(c) local TLS configured (TrustTLSProxy irrelevant) -> HSTS emitted
//	    (unchanged pre-existing path).
func TestSecurityHeaders_HSTS_TrustTLSProxy(t *testing.T) {
	// (a) Safe default: flag unset, no local TLS -> HSTS absent.
	defaultOff := newSecHeadersRouterTrustProxy(t, "", "", false)
	if got := getReq(defaultOff, "/healthz").Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("default (TrustTLSProxy=false), no local TLS: HSTS = %q, want absent (empty)", got)
	}

	// (b) The fix: trust flag ON, no local TLS -> HSTS present.
	trustProxyOn := newSecHeadersRouterTrustProxy(t, "", "", true)
	if got := getReq(trustProxyOn, "/healthz").Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Errorf("TrustTLSProxy=true, no local TLS: HSTS = %q, want %q", got, hstsValue)
	}

	// (c) Unchanged path: local TLS configured -> HSTS present regardless of the
	// (here false, i.e. default) trust-flag value.
	localTLS := newSecHeadersRouterTrustProxy(t, "/tmp/cert.pem", "/tmp/key.pem", false)
	if got := getReq(localTLS, "/healthz").Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Errorf("local TLS configured: HSTS = %q, want %q", got, hstsValue)
	}
}

// TestSecurityHeaders_TierB_ScopedToAPI asserts the strict JSON CSP +
// Cache-Control: no-store are present on API responses and ABSENT on the SPA
// path (hashed assets must stay cacheable, and the SPA document uses the Tier-C
// CSP, not the API CSP).
func TestSecurityHeaders_TierB_ScopedToAPI(t *testing.T) {
	env := newTestEnv(t)

	// API response carries Tier B.
	api := getReq(env.router, "/api/v1/devices") // 401, still in the v1 group
	if got := api.Header().Get("Content-Security-Policy"); got != apiCSP {
		t.Errorf("API CSP = %q, want %q", got, apiCSP)
	}
	if got := api.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("API Cache-Control = %q, want no-store", got)
	}

	// Health probe is NOT in the API group: no Tier-B API CSP, no no-store.
	hz := getReq(env.router, "/healthz")
	if got := hz.Header().Get("Content-Security-Policy"); got == apiCSP {
		t.Errorf("/healthz carries the API CSP %q — Tier B leaked outside the API group", got)
	}
	if got := hz.Header().Get("Cache-Control"); strings.Contains(got, "no-store") {
		t.Errorf("/healthz Cache-Control = %q, must not carry API no-store", got)
	}
}

// TestSecurityHeaders_TierB_NotOnSPAAssets asserts the SPA path does NOT get the
// API Tier-B headers (no no-store on hashed assets; the SPA document carries the
// Tier-C CSP, not the API CSP). Skips if the SPA is not built into the embed.
func TestSecurityHeaders_TierB_NotOnSPAAssets(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	// The SPA document uses the Tier-C CSP, never the API CSP.
	doc := getReq(env.router, "/manager/")
	if got := doc.Header().Get("Content-Security-Policy"); got == apiCSP {
		t.Errorf("/manager/ carries the API CSP — Tier B/C confusion")
	}
	if got := doc.Header().Get("Cache-Control"); strings.Contains(got, "no-store") {
		t.Errorf("/manager/ Cache-Control = %q, must not carry API no-store", got)
	}

	// A hashed asset must not be forced no-store (defeats content-hash caching).
	index := fetchServedIndex(t, env)
	refs := assetRefRe.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Fatalf("index.html references no assets to check")
	}
	asset := getReq(env.router, "/manager"+refs[0][1])
	if asset.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", "/manager"+refs[0][1], asset.Code)
	}
	if got := asset.Header().Get("Cache-Control"); strings.Contains(got, "no-store") {
		t.Errorf("asset %s Cache-Control = %q, must not be no-store", refs[0][1], got)
	}
}

// TestSecurityHeaders_TierC_SPADocumentCSP asserts the SPA index.html document
// (served at /manager/ and via the client-route fallback) carries the Tier-C
// CSP, and that over plain HTTP it does NOT include upgrade-insecure-requests.
func TestSecurityHeaders_TierC_SPADocumentCSP(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t) // plain HTTP (no TLS)

	wantCSP := spaDocumentCSP(false)

	// Direct document at /manager/.
	root := getReq(env.router, "/manager/")
	if root.Code != http.StatusOK {
		t.Fatalf("GET /manager/ = %d, want 200", root.Code)
	}
	if got := root.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("GET /manager/ CSP = %q, want %q", got, wantCSP)
	}

	// Client-route fallback document at /manager/devices.
	route := getReq(env.router, "/manager/devices")
	if route.Code != http.StatusOK {
		t.Fatalf("GET /manager/devices = %d, want 200", route.Code)
	}
	if got := route.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("GET /manager/devices CSP = %q, want %q", got, wantCSP)
	}

	// Plain HTTP: upgrade-insecure-requests MUST be absent (would white-screen a
	// non-localhost HTTP deployment by upgrading same-origin asset loads).
	if strings.Contains(root.Header().Get("Content-Security-Policy"), "upgrade-insecure-requests") {
		t.Errorf("plain HTTP SPA CSP contains upgrade-insecure-requests; must be TLS-gated")
	}
}

// TestSecurityHeaders_TierC_UpgradeInsecureWhenTLS asserts the SPA CSP DOES carry
// upgrade-insecure-requests when TLS is configured.
func TestSecurityHeaders_TierC_UpgradeInsecureWhenTLS(t *testing.T) {
	requireBuiltEmbed(t)
	tlsR := newSecHeadersRouter(t, "/tmp/cert.pem", "/tmp/key.pem")

	root := getReq(tlsR, "/manager/")
	if root.Code != http.StatusOK {
		t.Fatalf("GET /manager/ (TLS) = %d, want 200", root.Code)
	}
	csp := root.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "upgrade-insecure-requests") {
		t.Errorf("TLS SPA CSP = %q, want it to contain upgrade-insecure-requests", csp)
	}
	if csp != spaDocumentCSP(true) {
		t.Errorf("TLS SPA CSP = %q, want %q", csp, spaDocumentCSP(true))
	}
}

// TestSPACSP_CompatibleWithRealBundle is the §11.4.170/§11.4.123 gate: it proves
// the Tier-C CSP is compatible with the REAL built manager UI — i.e. it does NOT
// white-screen it. It ties the served index.html + the derived bundle
// requirements (EVIDENCE.md static analysis) to the emitted CSP:
//
//   - every subresource index.html references is same-origin (/assets/*) → the
//     script-src/style-src/img-src 'self' directives serve them (no cross-origin
//     source needed);
//   - the CSP grants style-src 'unsafe-inline' — REQUIRED, because the bundle
//     injects <style> elements at runtime (React 19 style hoisting + a CSS-in-JS
//     injector). Without it the UI renders unstyled;
//   - the CSP does NOT grant 'unsafe-eval' — the bundle contains no
//     eval/new Function/WebAssembly, so the strictest script policy is compatible
//     (a tightness proof: we allow exactly what is needed, nothing more);
//   - index.html carries no inline <script> body — so script-src 'self' with no
//     hash/nonce/'unsafe-inline' is sufficient for scripts.
func TestSPACSP_CompatibleWithRealBundle(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	root := getReq(env.router, "/manager/")
	if root.Code != http.StatusOK {
		t.Fatalf("GET /manager/ = %d, want 200", root.Code)
	}
	csp := root.Header().Get("Content-Security-Policy")
	index := root.Body.String()

	// (1) All referenced subresources are same-origin (/assets/*): 'self' covers.
	refs := assetRefRe.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Fatalf("index.html references no /assets/* subresources to validate against the CSP")
	}
	sawJS := false
	for _, m := range refs {
		ref := m[1]
		if !strings.HasPrefix(ref, "/assets/") {
			t.Errorf("index.html references a non-same-origin/self-incompatible subresource %q; "+
				"CSP 'self' would block it", ref)
		}
		if strings.HasSuffix(ref, ".js") {
			sawJS = true
		}
	}
	if !sawJS {
		t.Fatalf("index.html references no /assets/*.js — cannot validate script-src compatibility")
	}

	// (2) style-src MUST allow 'unsafe-inline' (runtime <style> injection).
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("SPA CSP = %q; MUST contain \"style-src 'self' 'unsafe-inline'\" — the real bundle "+
			"injects <style> elements at runtime and would render unstyled without it", csp)
	}

	// (3) Tightness: the bundle needs no eval, so the CSP must not weaken to it.
	if strings.Contains(csp, "unsafe-eval") {
		t.Errorf("SPA CSP = %q contains 'unsafe-eval'; the bundle needs none (no eval/new Function/wasm) "+
			"— do not weaken script-src", csp)
	}

	// (4) index.html carries no inline <script> body (only external module src),
	// so script-src 'self' (no hash/nonce/'unsafe-inline') is sufficient.
	if hasInlineScript(index) {
		t.Errorf("index.html contains an inline <script> body; script-src 'self' would block it " +
			"— the CSP or the build must be reconciled")
	}
}

// hasInlineScript reports whether the HTML contains a <script> element with an
// inline body (i.e. a <script> whose opening tag has no src= attribute and is
// followed by non-empty content). External module scripts (<script ... src=...>)
// are NOT inline and return false.
func hasInlineScript(html string) bool {
	lower := strings.ToLower(html)
	for i := 0; ; {
		open := strings.Index(lower[i:], "<script")
		if open < 0 {
			return false
		}
		open += i
		tagEnd := strings.Index(lower[open:], ">")
		if tagEnd < 0 {
			return false
		}
		tagEnd += open
		openTag := lower[open : tagEnd+1]
		close := strings.Index(lower[tagEnd:], "</script>")
		body := ""
		if close >= 0 {
			body = strings.TrimSpace(html[tagEnd+1 : tagEnd+close])
		}
		// An inline script has a non-empty body and no src attribute.
		if !strings.Contains(openTag, "src=") && body != "" {
			return true
		}
		i = tagEnd + 1
	}
}
