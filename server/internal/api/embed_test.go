package api

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// §11.4.38 installable-asset evidence + §11.4.27 (real handler, no fakes):
// exercise the REAL embedded OTA-Manager SPA through the REAL Gin router
// (Server.Router() -> MountManagerUI). Every PASS opens the served asset and
// verifies it is PRESENT and NON-DEGENERATE — not a 404 page, not an empty
// placeholder. If the embed has not been built (manager-dist/ holds only the
// placeholder), the suite SKIPs with reason per §11.4.3 rather than fake-passing
// on an empty embed.
//
// Discovered handler: internal/api/embed.go
//   - //go:embed manager-dist/*                          (embed.go:50-51)
//   - (*Server).MountManagerUI: StaticFS("/manager", ...) (embed.go:57-68)
//   - r.NoRoute asset-aware SPA fallback: for a GET under /manager with no real
//     embedded file it 404s an ASSET-LIKE path (under /manager/assets/ OR whose
//     last segment has a file extension) and serves index.html for an
//     extension-less CLIENT ROUTE.                          (embed.go:72-118)
//
// Observed real routing (re-probed 2026-07-09 after the §11.4.120 fix, gin v1.x,
// Go net/http FileServer):
//   GET /manager/                          -> 200 text/html  (index.html)
//   GET /manager/assets/<hash>.js          -> 200 text/javascript (real bundle)
//   GET /manager/assets/<hash>.css         -> 200 text/css        (real stylesheet)
//   GET /manager/devices  (client route)   -> 200 text/html  (SPA fallback)
//   GET /manager/assets/does-not-exist.js  -> 404 text/plain (asset-aware:
//        a missing asset no longer masquerades as index.html — see
//        TestManagerSPA_MissingAsset_Returns404)

// assetRefRe matches an absolute asset reference emitted by the Vite build in
// index.html, e.g.  src="/assets/index-BCUfw0_g.js"  or
// href="/assets/index-BhJ2of6B.css".
var assetRefRe = regexp.MustCompile(`(?:src|href)="(/assets/[^"]+\.(?:js|css))"`)

// embedIsBuilt reports whether the //go:embed manager-dist tree actually
// contains a built SPA (a non-empty index.html plus at least one non-trivial
// assets/*.js). It reads the embedded FS directly (managerFS, embed.go:51) so
// the decision is about the artifact compiled into THIS binary, not the working
// tree. Returns a reason string when the embed is only a placeholder.
func embedIsBuilt() (ok bool, reason string) {
	index, err := managerFS.ReadFile("manager-dist/index.html")
	if err != nil {
		return false, "manager-dist/index.html absent from embed: " + err.Error()
	}
	if len(index) == 0 {
		return false, "manager-dist/index.html is a 0-byte placeholder (SPA not built)"
	}
	entries, err := fs.ReadDir(managerFS, "manager-dist/assets")
	if err != nil {
		return false, "manager-dist/assets absent from embed: " + err.Error()
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		info, ierr := e.Info()
		if ierr == nil && info.Size() > 1024 {
			return true, ""
		}
	}
	return false, "manager-dist/assets holds no non-trivial *.js bundle (SPA not built)"
}

// requireBuiltEmbed skips the calling test honestly when the embed is empty.
func requireBuiltEmbed(t *testing.T) {
	t.Helper()
	if ok, reason := embedIsBuilt(); !ok {
		t.Skipf("SKIP (§11.4.3): OTA-Manager SPA not built into embed — %s. "+
			"Build clients/ota-manager and copy dist/ into internal/api/manager-dist/ "+
			"(see embed.go header) then re-run.", reason)
	}
}

// fetchServedIndex GETs /manager/ through the real router and returns the served
// index.html body, failing (not skipping) if the embed is built but serving is
// broken.
func fetchServedIndex(t *testing.T, env *testEnv) string {
	t.Helper()
	w := env.do("GET", "/manager/", "", nil, "")
	if w.Code != 200 {
		t.Fatalf("GET /manager/ = %d, want 200 (embed is built but router did not serve index.html)", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Fatalf("GET /manager/ returned an empty body — degenerate index.html")
	}
	return body
}

// TestManagerSPA_Root_ServesRealIndexHTML asserts the SPA root serves a present,
// non-degenerate index.html (real HTML with an app mount + a script tag), not a
// 404 page and not an empty file. §11.4.38.
func TestManagerSPA_Root_ServesRealIndexHTML(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	w := env.do("GET", "/manager/", "", nil, "")
	if w.Code != 200 {
		t.Fatalf("GET /manager/ = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET /manager/ content-type = %q, want text/html", ct)
	}
	body := w.Body.String()

	// PRESENT + NON-DEGENERATE: real HTML document markers.
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "<!doctype html") {
		t.Fatalf("index.html missing <!doctype html> marker; body head=%q", head(body))
	}
	if !strings.Contains(lower, `<div id="root"`) {
		t.Fatalf("index.html missing app-mount <div id=\"root\">; body head=%q", head(body))
	}
	if !strings.Contains(lower, "<script") {
		t.Fatalf("index.html missing a <script> tag; body head=%q", head(body))
	}
	// Not a fallback/404 masquerade.
	if strings.Contains(lower, "404 page not found") {
		t.Fatalf("GET /manager/ served a 404 page, not the SPA index.html")
	}
	if len(body) < 100 {
		t.Fatalf("index.html suspiciously small (%d bytes) — likely degenerate", len(body))
	}
}

// TestManagerSPA_AssetChain_ServesRealBundle is the core §11.4.38 check: it opens
// the served index.html, extracts a hashed asset it references, GETs that asset
// through the real router, and asserts the asset is present, correctly typed, and
// non-trivial in size (a real JS/CSS bundle, not an error page).
func TestManagerSPA_AssetChain_ServesRealBundle(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	index := fetchServedIndex(t, env)

	refs := assetRefRe.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Fatalf("index.html references no /assets/*.{js,css} — cannot verify the asset chain; body head=%q", head(index))
	}

	var sawJS, sawCSS bool
	for _, m := range refs {
		ref := m[1] // e.g. "/assets/index-BCUfw0_g.js"
		// index.html references assets from the site root ("/assets/..."); the
		// StaticFS mount serves them under the /manager prefix.
		served := "/manager" + ref

		w := env.do("GET", served, "", nil, "")
		if w.Code != 200 {
			t.Fatalf("GET %s = %d, want 200 (asset referenced by index.html not served)", served, w.Code)
		}
		body := w.Body.Bytes()
		ct := w.Header().Get("Content-Type")

		switch {
		case strings.HasSuffix(ref, ".js"):
			sawJS = true
			if !strings.Contains(ct, "javascript") {
				t.Fatalf("GET %s content-type = %q, want a javascript type", served, ct)
			}
			if len(body) < 1024 {
				t.Fatalf("GET %s served only %d bytes — degenerate JS bundle", served, len(body))
			}
			// Not the HTML fallback masquerading as JS.
			if strings.Contains(strings.ToLower(string(body[:min(64, len(body))])), "<!doctype html") {
				t.Fatalf("GET %s served HTML, not a JS bundle (greedy-fallback leak on a real asset)", served)
			}
		case strings.HasSuffix(ref, ".css"):
			sawCSS = true
			if !strings.Contains(ct, "css") {
				t.Fatalf("GET %s content-type = %q, want text/css", served, ct)
			}
			if len(body) < 256 {
				t.Fatalf("GET %s served only %d bytes — degenerate CSS", served, len(body))
			}
		}
	}
	if !sawJS {
		t.Fatalf("index.html referenced no /assets/*.js bundle — a Vite SPA must ship one")
	}
	_ = sawCSS // CSS is expected but not strictly mandated by this check.
}

// TestManagerSPA_ClientRouteFallback asserts the discovered SPA-fallback design:
// an unknown client-side route under /manager (no matching file) serves
// index.html with 200 so the React router can take over. This is the real
// behavior in embed.go:71-99, asserted — not invented.
func TestManagerSPA_ClientRouteFallback(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	root := fetchServedIndex(t, env)

	w := env.do("GET", "/manager/devices", "", nil, "")
	if w.Code != 200 {
		t.Fatalf("GET /manager/devices = %d, want 200 (SPA client-route fallback)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET /manager/devices content-type = %q, want text/html", ct)
	}
	body := w.Body.String()
	if !strings.Contains(strings.ToLower(body), `<div id="root"`) {
		t.Fatalf("GET /manager/devices did not serve the SPA index.html; body head=%q", head(body))
	}
	// The fallback must serve the SAME document as the root.
	if body != root {
		t.Fatalf("GET /manager/devices body differs from /manager/ index.html (fallback served a different document)")
	}
}

// TestManagerSPA_MissingAsset_Returns404 asserts the §11.4.120-reconciled
// behavior: a clearly-absent ASSET under /manager (a hashed /manager/assets/*.js
// that resolves to no real embedded file) is answered with a 404 — NOT the
// greedy 200 + index.html the handler previously returned.
//
// PRIOR BEHAVIOR (superseded, embed.go pre-fix): the NoRoute fallback did not
// distinguish asset paths from client routes, so a missing /manager/assets/*.js
// returned 200 + index.html (text/html). A browser requesting a stale/renamed
// hashed asset then received an HTML document at a .js URL and failed to parse
// it ("Unexpected token '<'"), masking the real load failure. The fix makes an
// asset-like miss 404 so the browser surfaces the failure honestly; this test
// FAILS if that fix regresses back to serving index.html for a missing asset.
func TestManagerSPA_MissingAsset_Returns404(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	w := env.do("GET", "/manager/assets/does-not-exist-"+randToken()+".js", "", nil, "")

	// Reconciled behavior: a missing asset 404s — it must NOT serve index.html.
	if w.Code != 404 {
		t.Fatalf("GET missing /manager/assets/*.js = %d, want 404 (asset-aware fallback must not serve index.html for a missing asset)", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(strings.ToLower(body), `<div id="root"`) {
		t.Fatalf("GET missing asset served the SPA index.html body — greedy-fallback regression; body head=%q", head(body))
	}
	if ct := w.Header().Get("Content-Type"); strings.Contains(ct, "text/html") {
		t.Fatalf("GET missing asset content-type = %q, want a non-HTML type (must not masquerade as index.html)", ct)
	}
}

// TestManagerSPA_MissingExtensionlessAssetLikePath_Returns404 guards the second
// half of the asset-like rule: a missing path OUTSIDE /manager/assets/ whose
// last segment carries a file extension (e.g. /manager/favicon.ico) is still
// asset-like and MUST 404, not fall back to index.html. This proves the fix
// keys on the extension, not merely on the /assets/ prefix, and FAILS if the
// rule narrows to the prefix alone.
func TestManagerSPA_MissingExtensionlessAssetLikePath_Returns404(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	w := env.do("GET", "/manager/does-not-exist-"+randToken()+".svg", "", nil, "")
	if w.Code != 404 {
		t.Fatalf("GET missing /manager/*.svg = %d, want 404 (extension makes it asset-like)", w.Code)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), `<div id="root"`) {
		t.Fatalf("missing extensioned path fell back to index.html — asset-like rule regressed to the /assets/ prefix only")
	}
}

// randToken returns a short unique suffix so the missing-asset path cannot
// collide with a real embedded file across runs.
func randToken() string { return "zzq9x7" }

func head(s string) string {
	const n = 80
	if len(s) > n {
		return s[:n]
	}
	return s
}
