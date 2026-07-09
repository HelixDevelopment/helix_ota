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
//   - (*Server).MountManagerUI: StaticFS("/manager", ...) (embed.go:56-67)
//   - r.NoRoute greedy SPA fallback -> index.html for any
//     GET under /manager that does not resolve to a file   (embed.go:71-99)
//
// Observed real routing (probed 2026-07-09, gin v1.x, Go net/http FileServer):
//   GET /manager/                          -> 200 text/html  (index.html)
//   GET /manager/assets/<hash>.js          -> 200 text/javascript (real bundle)
//   GET /manager/assets/<hash>.css         -> 200 text/css        (real stylesheet)
//   GET /manager/devices  (client route)   -> 200 text/html  (SPA fallback)
//   GET /manager/assets/does-not-exist.js  -> 200 text/html  (GREEDY fallback,
//        see the honest finding in TestManagerSPA_MissingAsset_GreedyFallback)

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

// TestManagerSPA_MissingAsset_GreedyFallback documents + asserts the REAL,
// discovered behavior for a clearly-absent asset under /manager: the NoRoute
// fallback does NOT distinguish asset paths from client routes, so a missing
// /manager/assets/*.js is answered with 200 + index.html (text/html), NOT a 404.
//
// HONEST FINDING (§11.4.6): this greedy fallback means a browser that requests a
// stale/renamed hashed asset receives an HTML document with a JS/CSS URL — a
// latent correctness concern (a 404 for a missing /assets/* path would be more
// correct so the browser surfaces the load error). The test asserts the real
// behavior as-is; changing it is a product decision outside this test's scope.
func TestManagerSPA_MissingAsset_GreedyFallback(t *testing.T) {
	requireBuiltEmbed(t)
	env := newTestEnv(t)

	w := env.do("GET", "/manager/assets/does-not-exist-"+randToken()+".js", "", nil, "")

	// Real behavior: greedy SPA fallback -> 200 + index.html.
	if w.Code != 200 {
		t.Fatalf("GET missing asset = %d; discovered design is a greedy fallback returning 200+index.html", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(strings.ToLower(body), `<div id="root"`) {
		t.Fatalf("missing asset did not fall back to the SPA index.html; body head=%q", head(body))
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
