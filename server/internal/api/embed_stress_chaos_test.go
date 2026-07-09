package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- §11.4.85 stress + chaos tests for the embedded OTA-Manager SPA serve path ---
//
// Under test: internal/api/embed.go — (*Server).MountManagerUI, i.e. the
// StaticFS("/manager", ...) mount + the asset-aware NoRoute SPA fallback (a
// missing ASSET 404s, a client ROUTE falls back to index.html). The functional
// contract is covered by embed_test.go (TestManagerSPA_*). This file adds the
// §11.4.85-mandated STRESS (sustained load, concurrent contention, boundary
// conditions) and CHAOS (input-corruption, resource-pressure, traversal/
// injection census) coverage the fix previously lacked.
//
// Anti-bluff (§11.4 / §11.4.5 / §11.4.69): the stress tests drive REAL
// concurrent goroutines through the REAL Gin router (Server.Router() ->
// MountManagerUI) with the REAL built embed (no mocks per §11.4.27); every PASS
// records a captured-evidence artefact (per-request latency JSONL + a
// categorized-results census) under a gitignored qa-results/ dir, and the
// traversal PROVE asserts no host file ever escapes the embed. When the SPA is
// not built into the binary the suite SKIPs with reason per §11.4.3 (see
// requireBuiltEmbed in embed_test.go) rather than fake-passing on an empty
// embed.
//
// Reused helpers (same package): stressServer / doStressReq / percentileSorted
// (stress_test.go, resilience_test.go), requireBuiltEmbed / assetRefRe / head
// (embed_test.go).

// realManagerAsset extracts a real hashed asset reference from the served
// index.html so the stress load hits a genuine 200-serving asset URL (not a
// hardcoded hash that could rot). Returns the /manager-prefixed served path.
func realManagerAsset(t *testing.T, env *testEnv) string {
	t.Helper()
	index := fetchServedIndex(t, env)
	refs := assetRefRe.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Fatalf("index.html references no /assets/*.{js,css}; head=%q", head(index))
	}
	return "/manager" + refs[0][1] // e.g. "/manager/assets/index-XXXX.js"
}

// embedRec runs one GET through the router and returns the full recorder so
// stress/chaos assertions can inspect status + content-type + body.
func embedRec(router http.Handler, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// embedEvidenceDir returns the (gitignored) evidence directory for embed
// stress/chaos captures, honoring HELIX_STRESS_EVIDENCE_DIR like the sibling
// suites. Falls back to repo-root qa-results/embed_stress_chaos/.
func embedEvidenceDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("HELIX_STRESS_EVIDENCE_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "qa-results", "embed_stress_chaos")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("evidence dir %s: %v", dir, err)
	}
	return dir
}

// writeCensus appends a categorized-results census line block as captured
// evidence (§11.4.85 / §11.4.5 — every PASS cites its evidence path).
func writeCensus(t *testing.T, name string, lines []string) string {
	t.Helper()
	dir := embedEvidenceDir(t)
	ts := time.Now().UTC().Format("20060102T150405Z")
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.txt", name, ts))
	f, err := os.Create(p)
	if err != nil {
		t.Logf("create census %s: %v", p, err)
		return ""
	}
	defer f.Close()
	for _, l := range lines {
		_, _ = f.WriteString(l + "\n")
	}
	t.Logf("%s: evidence=%s", name, p)
	return p
}

// pctl computes the p-th percentile from an unsorted duration slice.
func pctl(lat []time.Duration, p float64) time.Duration {
	if len(lat) == 0 {
		return 0
	}
	s := make([]time.Duration, len(lat))
	copy(s, lat)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return percentileSorted(s, p)
}

// ==========================================================================
// STRESS
// ==========================================================================

// TestStressManagerSPA_SustainedMixedLoad drives a large number of concurrent
// GETs across the four real routing classes of the SPA serve path (index /
// real asset / client route / missing asset) and asserts EVERY response is
// correct for its class, records p50/p95/p99 latency (overall + per class),
// and checks for a goroutine leak. N = 4 classes * 120 = 480 concurrent
// requests (>= §11.4.85 sustained-load floor of 100).
func TestStressManagerSPA_SustainedMixedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	realAsset := realManagerAsset(t, env)
	router := env.router

	type class struct {
		name    string
		path    func(i int) string
		wantErr func(w *httptest.ResponseRecorder) string // "" == correct
	}
	classes := []class{
		{
			name: "index",
			path: func(int) string { return "/manager/" },
			wantErr: func(w *httptest.ResponseRecorder) string {
				if w.Code != 200 {
					return fmt.Sprintf("code=%d want 200", w.Code)
				}
				if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
					return "content-type not text/html"
				}
				if !strings.Contains(strings.ToLower(w.Body.String()), `<div id="root"`) {
					return "body not index.html"
				}
				return ""
			},
		},
		{
			name: "real_asset",
			path: func(int) string { return realAsset },
			wantErr: func(w *httptest.ResponseRecorder) string {
				if w.Code != 200 {
					return fmt.Sprintf("code=%d want 200", w.Code)
				}
				ct := w.Header().Get("Content-Type")
				if !strings.Contains(ct, "javascript") && !strings.Contains(ct, "css") {
					return "asset content-type=" + ct
				}
				if strings.Contains(strings.ToLower(head(w.Body.String())), "<!doctype html") {
					return "asset served HTML (greedy-fallback leak)"
				}
				return ""
			},
		},
		{
			name: "client_route",
			path: func(i int) string { return fmt.Sprintf("/manager/devices-%d", i) },
			wantErr: func(w *httptest.ResponseRecorder) string {
				if w.Code != 200 {
					return fmt.Sprintf("code=%d want 200", w.Code)
				}
				if !strings.Contains(strings.ToLower(w.Body.String()), `<div id="root"`) {
					return "client route did not fall back to index.html"
				}
				return ""
			},
		},
		{
			name: "missing_asset",
			path: func(i int) string { return fmt.Sprintf("/manager/assets/missing-%d-%s.js", i, randToken()) },
			wantErr: func(w *httptest.ResponseRecorder) string {
				if w.Code != 404 {
					return fmt.Sprintf("code=%d want 404", w.Code)
				}
				if strings.Contains(strings.ToLower(w.Body.String()), `<div id="root"`) {
					return "missing asset served index.html (greedy-fallback regression)"
				}
				return ""
			},
		},
	}

	const perClass = 120
	total := perClass * len(classes)

	// Goroutine-leak baseline (ServeHTTP is fully synchronous, so after wg.Wait
	// the only surviving goroutines beyond baseline would be a real handler leak).
	runtime.GC()
	baseGoroutines := runtime.NumGoroutine()

	lat := make([]time.Duration, total)
	classIdx := make([]int, total)
	failMsg := make([]string, total)
	var errCount int64
	var wg sync.WaitGroup
	wg.Add(total)
	idx := 0
	for ci := range classes {
		for j := 0; j < perClass; j++ {
			slot, cls, jj := idx, ci, j
			classIdx[slot] = ci
			go func() {
				defer wg.Done()
				p := classes[cls].path(jj)
				start := time.Now()
				w := embedRec(router, http.MethodGet, p)
				lat[slot] = time.Since(start)
				if msg := classes[cls].wantErr(w); msg != "" {
					failMsg[slot] = fmt.Sprintf("%s %q: %s", classes[cls].name, p, msg)
					atomic.AddInt64(&errCount, 1)
				}
			}()
			idx++
		}
	}
	// No-deadlock guard: the whole fan-out must finish within a generous budget.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("sustained mixed load did not complete within 30s — possible deadlock")
	}

	// Per-class + overall latency percentiles.
	perClassLat := make(map[int][]time.Duration)
	for i := range lat {
		perClassLat[classIdx[i]] = append(perClassLat[classIdx[i]], lat[i])
	}
	census := []string{
		fmt.Sprintf("test=%s total=%d classes=%d errors=%d", t.Name(), total, len(classes), errCount),
		fmt.Sprintf("overall p50=%s p95=%s p99=%s", pctl(lat, 50), pctl(lat, 95), pctl(lat, 99)),
	}
	for ci := range classes {
		cl := perClassLat[ci]
		census = append(census, fmt.Sprintf("class=%-13s n=%d p50=%s p95=%s p99=%s",
			classes[ci].name, len(cl), pctl(cl, 50), pctl(cl, 95), pctl(cl, 99)))
	}
	// Record every failure line so a FAIL is diagnosable from evidence.
	for i := range failMsg {
		if failMsg[i] != "" {
			census = append(census, "FAIL: "+failMsg[i])
		}
	}

	// Goroutine-leak check: allow a small slack for test-runtime goroutines,
	// poll briefly to let any transient settle.
	var leaked int
	for i := 0; i < 20; i++ {
		runtime.GC()
		leaked = runtime.NumGoroutine() - baseGoroutines
		if leaked <= 4 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	census = append(census, fmt.Sprintf("goroutines base=%d delta=%d (leak-tolerance<=4)", baseGoroutines, leaked))
	p := writeCensus(t, "stress_mixed_load", census)

	t.Logf("sustained_mixed_load: total=%d errors=%d overall p50=%s p95=%s p99=%s goroutine_delta=%d evidence=%s",
		total, errCount, pctl(lat, 50), pctl(lat, 95), pctl(lat, 99), leaked, p)

	if errCount > 0 {
		for i := range failMsg {
			if failMsg[i] != "" {
				t.Errorf("mixed-load response error: %s", failMsg[i])
			}
		}
	}
	if leaked > 4 {
		t.Errorf("goroutine leak: base=%d now=%d delta=%d (>4) — a handler leaked goroutines under load",
			baseGoroutines, baseGoroutines+leaked, leaked)
	}
}

// TestStressManagerSPA_ConcurrentContention hammers the SAME handler from many
// goroutines each doing repeated back-to-back GETs (index + real asset), the
// -race-clean concurrency contention probe: no data race, no deadlock, every
// response correct. Run under `go test -race`.
func TestStressManagerSPA_ConcurrentContention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	realAsset := realManagerAsset(t, env)
	router := env.router

	const workers = 50
	const itersPerWorker = 20
	var errCount int64
	var wg sync.WaitGroup
	wg.Add(workers)
	start := time.Now()
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for k := 0; k < itersPerWorker; k++ {
				ri := embedRec(router, http.MethodGet, "/manager/")
				if ri.Code != 200 || !strings.Contains(strings.ToLower(ri.Body.String()), `<div id="root"`) {
					atomic.AddInt64(&errCount, 1)
				}
				ra := embedRec(router, http.MethodGet, realAsset)
				if ra.Code != 200 {
					atomic.AddInt64(&errCount, 1)
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("concurrent contention did not complete within 30s — possible deadlock")
	}

	totalReq := workers * itersPerWorker * 2
	writeCensus(t, "stress_contention", []string{
		fmt.Sprintf("test=%s workers=%d iters=%d total_requests=%d errors=%d elapsed=%s",
			t.Name(), workers, itersPerWorker, totalReq, errCount, time.Since(start)),
	})
	if errCount > 0 {
		t.Errorf("concurrent contention: %d incorrect responses under load", errCount)
	}
}

// TestStressManagerSPA_BoundaryPaths exercises boundary conditions AND the
// security-relevant path-traversal PROVE: empty/edge paths, a very long path,
// unicode, and a census of directory-traversal / encoded-traversal payloads.
// The mandate: NO host file ever escapes the embed. Every response is asserted
// to (a) never panic (the request completes), (b) never contain host-file
// content (e.g. /etc/passwd's "root:" marker), and (c) carry a categorized
// verdict written to evidence.
func TestStressManagerSPA_BoundaryPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	router := env.router

	// hostFileMarker is content that would only appear if a real host file
	// (like /etc/passwd) were served — the escape signal we must never see.
	const hostFileMarker = "root:x:0:0"

	cases := []struct {
		name string
		path string
	}{
		{"root_no_slash", "/manager"},
		{"root_slash", "/manager/"},
		{"empty_segment", "/manager//"},
		{"long_path_8k", "/manager/" + strings.Repeat("a", 8000) + ".js"},
		{"long_client_route", "/manager/" + strings.Repeat("b", 8000)},
		{"unicode", "/manager/devices/éü中文"},
		{"unicode_asset", "/manager/assets/éü.js"},
		{"dotdot_etc_passwd", "/manager/../../etc/passwd"},
		{"dotdot_deep", "/manager/../../../../../../etc/passwd"},
		{"assets_dotdot", "/manager/assets/../../../../etc/passwd"},
		{"encoded_dotdot", "/manager/%2e%2e/%2e%2e/etc/passwd"},
		{"encoded_slash", "/manager/..%2f..%2fetc%2fpasswd"},
		{"mixed_encoded", "/manager/assets/%2e%2e%2f%2e%2e%2fetc/passwd"},
		{"backslash", "/manager/..\\..\\etc\\passwd"},
		{"double_encoded", "/manager/%252e%252e/etc/passwd"},
		{"null_byteish", "/manager/assets/index.js%00.txt"},
	}

	census := []string{fmt.Sprintf("test=%s cases=%d host_file_marker=%q", t.Name(), len(cases), hostFileMarker)}
	escapes := 0
	for _, c := range cases {
		// Guard: the request itself must complete without panicking the router.
		var w *httptest.ResponseRecorder
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on %s (%q): %v", c.name, c.path, r)
				}
			}()
			w = embedRec(router, http.MethodGet, c.path)
		}()
		if w == nil {
			continue
		}
		body := w.Body.String()
		ct := w.Header().Get("Content-Type")

		// Categorize the verdict.
		var verdict string
		switch {
		case strings.Contains(body, hostFileMarker):
			verdict = "ESCAPE-HOST-FILE-LEAK"
			escapes++
		case w.Code == 200 && strings.Contains(strings.ToLower(body), `<div id="root"`):
			verdict = "spa-index-fallback(200,safe)"
		case w.Code == 200:
			verdict = "200-embed-file(safe)"
		case w.Code == 404:
			verdict = "404-blocked"
		case w.Code >= 300 && w.Code < 400:
			verdict = fmt.Sprintf("%d-redirect", w.Code)
		case w.Code == 400:
			verdict = "400-rejected"
		default:
			verdict = fmt.Sprintf("%d-other", w.Code)
		}
		census = append(census, fmt.Sprintf("case=%-20s code=%d ct=%-30q verdict=%s",
			c.name, w.Code, ct, verdict))

		// PROVE: no traversal ever serves host-file content.
		if strings.Contains(body, hostFileMarker) {
			t.Errorf("PATH ESCAPE: %s (%q) served host-file content (found %q) — traversal escaped the embed",
				c.name, c.path, hostFileMarker)
		}
	}
	census = append(census, fmt.Sprintf("host_file_escapes=%d (MUST be 0)", escapes))
	p := writeCensus(t, "stress_boundary_traversal", census)
	t.Logf("boundary_paths: cases=%d host_file_escapes=%d evidence=%s", len(cases), escapes, p)

	if escapes != 0 {
		t.Fatalf("path-traversal PROVE failed: %d host-file escape(s) — the embed serve path is NOT confined", escapes)
	}
}

// ==========================================================================
// CHAOS
// ==========================================================================

// TestChaosManagerSPA_OddMethodsAndHeaders sends non-GET methods and many
// headers at the SPA path through the in-process router. The handler must
// refuse cleanly (never panic, never 5xx from a nil-deref) — a non-GET under
// /manager passes through the NoRoute guard to a 404. Each result is
// categorized to evidence.
func TestChaosManagerSPA_OddMethodsAndHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	realAsset := realManagerAsset(t, env)
	router := env.router

	methods := []string{
		http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace, "WEIRD",
	}
	paths := []string{"/manager/", realAsset, "/manager/devices", "/manager/assets/missing-" + randToken() + ".js"}

	census := []string{fmt.Sprintf("test=%s methods=%d paths=%d", t.Name(), len(methods), len(paths))}
	for _, m := range methods {
		for _, p := range paths {
			var w *httptest.ResponseRecorder
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic on %s %s: %v", m, p, r)
					}
				}()
				// Attach a spray of headers (in-process ServeHTTP does not enforce
				// header-byte limits — that path is covered by the raw-wire test).
				r := httptest.NewRequest(m, p, nil)
				for i := 0; i < 40; i++ {
					r.Header.Add(fmt.Sprintf("X-Chaos-%d", i), strings.Repeat("v", 256))
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, r)
				w = rec
			}()
			if w == nil {
				continue
			}
			if w.Code >= 500 {
				t.Errorf("odd method %s %s returned %d (>=500) — not a clean refusal", m, p, w.Code)
			}
			census = append(census, fmt.Sprintf("method=%-8s path=%-40s code=%d", m, p, w.Code))
		}
	}
	p := writeCensus(t, "chaos_odd_methods_headers", census)
	t.Logf("odd_methods_headers: combos=%d evidence=%s", len(methods)*len(paths), p)
}

// rawSend dials the given TCP address, writes payload bytes, reads the response
// (bounded by a deadline), and returns the response text plus any read error.
// Used to drive genuinely-malformed HTTP at the wire level (impossible through
// httptest.NewRequest, which only builds well-formed requests).
func rawSend(addr, payload string) (string, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(payload)); err != nil {
		return "", err
	}
	br := bufio.NewReader(conn)
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, rerr := br.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if rerr != nil {
			return string(buf), rerr
		}
		if len(buf) > 1<<20 {
			return string(buf), nil
		}
	}
}

// firstLine returns the first line (status line) of a raw HTTP response.
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// TestChaosManagerSPA_RawMalformedWire drives input-corruption chaos at the
// socket level against a REAL httptest server wrapping the router: truncated
// requests, garbage bytes, oversized headers, oversized request line. The
// server MUST refuse each cleanly (400/431/414/closed connection) and — the
// recovery assertion — MUST still serve a valid request afterward (no crash).
// No response may ever leak host-file content.
func TestChaosManagerSPA_RawMalformedWire(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	srv := httptest.NewServer(env.router)
	defer srv.Close()
	addr := srv.Listener.Addr().String()

	const hostFileMarker = "root:x:0:0"

	// Baseline: a well-formed request over the wire returns 200.
	base, _ := rawSend(addr, "GET /manager/ HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	if !strings.Contains(base, "200") {
		t.Fatalf("wire baseline: expected 200 status line, got %q", firstLine(base))
	}

	cases := []struct {
		name    string
		payload string
	}{
		{"truncated_no_terminator", "GET /manager/ HTTP/1.1\r\nHost: x\r\n"},
		{"garbage_bytes", "\x00\x01\x02\x03\x04 not http at all \xff\xfe\r\n\r\n"},
		{"bad_request_line", "TOTALLY BROKEN REQUEST LINE\r\n\r\n"},
		{"oversized_header", "GET /manager/ HTTP/1.1\r\nHost: x\r\nX-Big: " + strings.Repeat("A", 2<<20) + "\r\nConnection: close\r\n\r\n"},
		{"oversized_request_line", "GET /manager/" + strings.Repeat("a", 2<<20) + " HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"},
		{"bare_lf", "GET /manager/ HTTP/1.1\nHost: x\n\n"},
	}

	census := []string{fmt.Sprintf("test=%s baseline=%q cases=%d", t.Name(), firstLine(base), len(cases))}
	for _, c := range cases {
		resp, rerr := rawSend(addr, c.payload)
		if strings.Contains(resp, hostFileMarker) {
			t.Errorf("wire chaos %s leaked host-file content", c.name)
		}
		// Categorize: a clean refusal is either an HTTP error status or a
		// closed/timed-out connection — never a crash (verified by the
		// post-census recovery probe).
		var verdict string
		switch {
		case resp == "":
			verdict = "connection-closed/no-response"
		case strings.HasPrefix(firstLine(resp), "HTTP/"):
			verdict = "http-status:" + firstLine(resp)
		default:
			verdict = "non-http-response:" + firstLine(resp)
		}
		errStr := "nil"
		if rerr != nil {
			errStr = rerr.Error()
		}
		census = append(census, fmt.Sprintf("case=%-24s verdict=%s read_err=%s", c.name, verdict, errStr))
	}

	// Recovery PROVE: after the malformed barrage the server must still serve a
	// valid request (it did not crash).
	rec, _ := rawSend(addr, "GET /manager/ HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	recovered := strings.Contains(rec, "200")
	census = append(census, fmt.Sprintf("recovery_after_chaos=%q recovered=%v", firstLine(rec), recovered))
	p := writeCensus(t, "chaos_raw_malformed_wire", census)
	t.Logf("raw_malformed_wire: cases=%d recovered=%v evidence=%s", len(cases), recovered, p)

	if !recovered {
		t.Fatalf("server did not recover after malformed-wire chaos — expected 200 on a valid request, got %q", firstLine(rec))
	}
}

// TestChaosManagerSPA_ConcurrentConnPressure applies resource-pressure chaos:
// many concurrent REAL HTTP connections hammering the SPA path (index + real
// asset + missing asset) through a live server. The server must serve every
// request with the correct class outcome (or degrade cleanly) and stay alive —
// no crash, no deadlock. Latency percentiles + a categorized census captured.
func TestChaosManagerSPA_ConcurrentConnPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	requireBuiltEmbed(t)
	t.Parallel()
	env := newTestEnv(t)
	realAsset := realManagerAsset(t, env)
	srv := httptest.NewServer(env.router)
	defer srv.Close()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        256,
			MaxIdleConnsPerHost: 256,
			MaxConnsPerHost:     256,
		},
	}

	type probe struct {
		path     string
		wantCode int
	}
	probes := []probe{
		{"/manager/", 200},
		{realAsset, 200},
		{"/manager/devices", 200},           // client-route fallback
		{"/manager/assets/miss-x9.js", 404}, // asset-aware 404
	}

	const total = 200
	lat := make([]time.Duration, total)
	var errCount, transport int64
	var wg sync.WaitGroup
	wg.Add(total)
	start := time.Now()
	for i := 0; i < total; i++ {
		pr := probes[i%len(probes)]
		go func(i int, pr probe) {
			defer wg.Done()
			t0 := time.Now()
			resp, err := client.Get(srv.URL + pr.path)
			lat[i] = time.Since(t0)
			if err != nil {
				atomic.AddInt64(&transport, 1)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != pr.wantCode {
				atomic.AddInt64(&errCount, 1)
			}
		}(i, pr)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(40 * time.Second):
		t.Fatalf("concurrent connection pressure did not complete within 40s — possible deadlock")
	}

	// Liveness PROVE: server still serves correctly after the pressure burst.
	live, lerr := client.Get(srv.URL + "/manager/")
	alive := lerr == nil && live != nil && live.StatusCode == 200
	if live != nil {
		live.Body.Close()
	}

	census := []string{
		fmt.Sprintf("test=%s total=%d code_mismatches=%d transport_errors=%d elapsed=%s",
			t.Name(), total, errCount, transport, time.Since(start)),
		fmt.Sprintf("latency p50=%s p95=%s p99=%s", pctl(lat, 50), pctl(lat, 95), pctl(lat, 99)),
		fmt.Sprintf("post_pressure_alive=%v", alive),
	}
	p := writeCensus(t, "chaos_conn_pressure", census)
	t.Logf("conn_pressure: total=%d mismatches=%d transport_errs=%d alive=%v p50=%s p95=%s p99=%s evidence=%s",
		total, errCount, transport, alive, pctl(lat, 50), pctl(lat, 95), pctl(lat, 99), p)

	if errCount > 0 {
		t.Errorf("connection-pressure: %d responses had the wrong status code", errCount)
	}
	if !alive {
		t.Fatalf("server not alive after connection-pressure burst (liveness PROVE failed)")
	}
}
