package api

import "github.com/gin-gonic/gin"

// Security response headers (Item O). Three tiers, because the JSON API and the
// embedded manager SPA have different needs:
//
//   - Tier A — securityHeadersMiddleware — set on EVERY response (API, SPA,
//     probes, and error/shed paths). Origin-isolation + anti-clickjacking +
//     MIME-sniff defence + a deny-all Permissions-Policy. HSTS is emitted ONLY
//     when TLS is configured OR the operator has declared a trusted
//     TLS-terminating reverse-proxy topology (cfg.TrustTLSProxy; see
//     tlsEnabled below) — never over genuinely plaintext HTTP.
//   - Tier B — apiSecurityHeadersMiddleware — applied to the API base-path group
//     only: a maximally-strict CSP for JSON (needs no resources at all) plus
//     `Cache-Control: no-store` (never cache tokens / device lists / audit logs).
//     NOT applied to the SPA, so hashed assets keep their cacheable headers.
//   - Tier C — spaDocumentCSP — set on the served manager index.html document in
//     MountManagerUI (embed.go). A `default-src 'self'` CSP tuned against the
//     REAL built Vite/React bundle (see docs/qa/20260710-server-security-headers/
//     EVIDENCE.md for the static analysis that derived each directive).
//
// Reference: OWASP Secure Headers Project recipe (2026-06-30), tailored to this
// single-binary API+SPA per docs/research/operator_brief_security_headers_ddos_20260710/BRIEF.md.
// COEP `require-corp` is deliberately OMITTED (high breakage risk, no benefit).

// permissionsPolicyDenyAll denies every powerful browser feature the control
// plane never uses. The empty allow-list `()` for each feature disables it for
// all origins (deny-all).
const permissionsPolicyDenyAll = "accelerometer=(), autoplay=(), camera=(), " +
	"cross-origin-isolated=(), display-capture=(), encrypted-media=(), " +
	"fullscreen=(), geolocation=(), gyroscope=(), keyboard-map=(), " +
	"magnetometer=(), microphone=(), midi=(), payment=(), picture-in-picture=(), " +
	"publickey-credentials-get=(), screen-wake-lock=(), sync-xhr=(), usb=(), " +
	"web-share=(), xr-spatial-tracking=(), clipboard-read=(), clipboard-write=(), " +
	"hid=(), idle-detection=(), serial=()"

// hstsValue is the Strict-Transport-Security policy: force HTTPS for two years,
// including subdomains. `preload` is intentionally omitted (a hard-to-reverse
// commitment; add only with explicit intent).
const hstsValue = "max-age=63072000; includeSubDomains"

// apiCSP is the Tier-B CSP for JSON API responses: a JSON body needs no
// resources at all, so lock everything down.
const apiCSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'"

// spaCSPBase is the Tier-C document CSP for the embedded manager UI, WITHOUT the
// `upgrade-insecure-requests` directive. Every directive is justified against the
// real built bundle (docs/qa/20260710-server-security-headers/EVIDENCE.md):
//
//	default-src 'self'         baseline: only same-origin resources
//	script-src  'self'         external module bundle is same-origin; the bundle
//	                           contains no eval/new Function/WebAssembly and
//	                           index.html carries no inline script → no hash /
//	                           'unsafe-inline' / 'unsafe-eval' needed
//	style-src   'self' 'unsafe-inline'
//	                           REQUIRED: the bundle injects <style> elements at
//	                           runtime (React 19 style hoisting + a CSS-in-JS
//	                           injector: createElement("style"), .cssText). Without
//	                           'unsafe-inline' those injected styles are blocked and
//	                           the UI renders unstyled/broken.
//	img-src     'self' data:   same-origin assets + inline data: images (safe
//	                           superset; base64 images/icons)
//	font-src    'self'         no @font-face / web fonts (Tailwind system stack)
//	connect-src 'self'         the SPA calls its own API same-origin (axios
//	                           baseURL is the serving origin in the single-binary
//	                           topology); no websockets
//	object-src  'none'         no plugins
//	base-uri 'self'; form-action 'self'; frame-ancestors 'none'
//	                           lock <base>, form targets, and framing (anti-
//	                           clickjacking)
const spaCSPBase = "default-src 'self'; script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; " +
	"connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; " +
	"frame-ancestors 'none'"

// spaDocumentCSP returns the Tier-C SPA document CSP, appending
// `upgrade-insecure-requests` ONLY when TLS is configured. Over plaintext HTTP
// on a non-localhost host, `upgrade-insecure-requests` would upgrade the
// same-origin /manager/assets/* subresource loads to https and white-screen the
// UI, so it is gated exactly like HSTS.
func spaDocumentCSP(tlsEnabled bool) string {
	if tlsEnabled {
		return spaCSPBase + "; upgrade-insecure-requests"
	}
	return spaCSPBase
}

// securityHeadersMiddleware sets the Tier-A headers on EVERY response. It runs
// early in the chain (before the in-flight limiter and the handlers) so the
// headers are present even on 429-shed and error responses. HSTS is emitted only
// when TLS is configured.
func securityHeadersMiddleware(tlsEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", permissionsPolicyDenyAll)
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		if tlsEnabled {
			h.Set("Strict-Transport-Security", hstsValue)
		}
		c.Next()
	}
}

// apiSecurityHeadersMiddleware adds the Tier-B API-only headers (strict JSON CSP
// + no-store). Applied to the API base-path group, NOT to the /manager SPA path
// (hashed assets must stay cacheable).
func apiSecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", apiCSP)
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}

// tlsEnabled reports whether the client-facing connection is genuinely HTTPS,
// gating the TLS-only response headers (HSTS and the SPA CSP's
// upgrade-insecure-requests) so they are never emitted on a truly-plaintext
// listener. It is true in either of two cases:
//
//   - the control plane terminates TLS itself (both TLSCertFile and TLSKeyFile
//     supplied — the original, still-default-safe gate); OR
//   - the operator has explicitly declared, via cfg.TrustTLSProxy, that this
//     process sits behind a trusted TLS-terminating reverse proxy (nginx /
//     cloud load balancer) that receives the real HTTPS connection and
//     forwards plain HTTP here (docs/research/security_headers_adversarial_
//     review_20260710/ finding I1 — without this gate, that common production
//     topology never has a local cert/key configured, so HSTS was silently
//     never sent even though the real client<->proxy hop is HTTPS).
//
// cfg.TrustTLSProxy is an operator-set boolean assertion, never derived from
// any request header (e.g. X-Forwarded-Proto) — trusting a client-supplied
// header unconditionally would let a client spoof HSTS emission over an
// actually-plaintext connection. Default (TrustTLSProxy=false) preserves the
// original cert/key-only gate byte-for-byte.
func (s *Server) tlsEnabled() bool {
	return (s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "") || s.cfg.TrustTLSProxy
}
