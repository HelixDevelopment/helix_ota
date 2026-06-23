package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain builds the loadtest binary once so the build+exec smoke tests below
// can run it as an external process — exactly how an operator invokes it.
var loadtestBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "loadtest-smoke-*")
	if err != nil {
		panic(err)
	}
	loadtestBin = filepath.Join(tmp, "loadtest")
	build := exec.Command("go", "build", "-o", loadtestBin, ".")
	build.Stderr = os.Stderr
	if buildErr := build.Run(); buildErr != nil {
		os.RemoveAll(tmp)
		panic("loadtest: build failed: " + buildErr.Error())
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Approach (b): build + exec smoke tests of main() — real exit codes / output.
// ---------------------------------------------------------------------------

// TestSmoke_Selftest exercises main() end-to-end via -selftest (no external
// deps: it spins up its own in-process 200-OK server) and asserts a clean exit
// plus a real measured JSON report on stdout.
func TestSmoke_Selftest(t *testing.T) {
	cmd := exec.Command(loadtestBin, "-selftest", "-duration", "300ms", "-concurrency", "4")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("selftest run should exit 0, got err=%v", err)
	}
	var rep report
	if jErr := json.Unmarshal(out, &rep); jErr != nil {
		t.Fatalf("selftest stdout must be a JSON report, got parse error %v; stdout=%q", jErr, string(out))
	}
	if rep.TotalRequests <= 0 {
		t.Errorf("selftest must measure >0 real requests, got %d", rep.TotalRequests)
	}
	if rep.RPS <= 0 {
		t.Errorf("selftest must report >0 RPS, got %f", rep.RPS)
	}
}

// TestSmoke_BadConcurrency asserts an invalid flag value yields exit code 2 and
// a usage-style error on stderr — not a panic, not a silent 0.
func TestSmoke_BadConcurrency(t *testing.T) {
	cmd := exec.Command(loadtestBin, "-concurrency", "0")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit, got err=%v", err)
	}
	if ee.ExitCode() != 2 {
		t.Errorf("expected exit code 2 for bad -concurrency, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "concurrency") {
		t.Errorf("expected concurrency error on stderr, got %q", stderr.String())
	}
}

// TestSmoke_BadFlag asserts an unknown flag is rejected (flag pkg exits 2).
func TestSmoke_BadFlag(t *testing.T) {
	cmd := exec.Command(loadtestBin, "-no-such-flag")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit for unknown flag, got err=%v", err)
	}
	if ee.ExitCode() != 2 {
		t.Errorf("expected exit code 2 for unknown flag, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("expected flag-undefined error, got %q", stderr.String())
	}
}

// ---------------------------------------------------------------------------
// Approach (a): in-process unit tests of the measurement logic — these are what
// move the package coverage off 0%.
// ---------------------------------------------------------------------------

// TestRun_AgainstRealServer drives run() against a throwaway httptest server and
// asserts the measured report is internally consistent (real round-trips).
func TestRun_AgainstRealServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	rep := run(srv.URL+"/healthz", 4, 200*time.Millisecond, 5*time.Second)
	if rep.TotalRequests <= 0 {
		t.Fatalf("run must record >0 requests, got %d", rep.TotalRequests)
	}
	if rep.Non2xx != 0 {
		t.Errorf("200-only server should yield 0 non-2xx, got %d", rep.Non2xx)
	}
	if rep.P50Ms < 0 || rep.P99Ms < rep.P50Ms {
		t.Errorf("percentiles inconsistent: p50=%f p99=%f", rep.P50Ms, rep.P99Ms)
	}
}

// TestRun_Non2xxCounted asserts a 500-returning server is counted as non-2xx.
func TestRun_Non2xxCounted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	rep := run(srv.URL, 2, 150*time.Millisecond, 5*time.Second)
	if rep.TotalRequests <= 0 {
		t.Fatalf("expected requests, got %d", rep.TotalRequests)
	}
	if rep.Non2xx <= 0 {
		t.Errorf("500 server must produce non-2xx > 0, got %d", rep.Non2xx)
	}
}

// TestPercentile verifies the nearest-rank percentile on a known input.
func TestPercentile(t *testing.T) {
	in := []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("empty slice percentile must be 0, got %v", got)
	}
	if got := percentile(in, 0); got != 1 {
		t.Errorf("p0 must be min(1), got %v", got)
	}
	if got := percentile(in, 1); got != 10 {
		t.Errorf("p1.0 must be max(10), got %v", got)
	}
	if got := percentile(in, 0.5); got < 4 || got > 6 {
		t.Errorf("p50 of 1..10 should be mid-range, got %v", got)
	}
}

// TestSummarize checks summary math on a fixed latency set.
func TestSummarize(t *testing.T) {
	lats := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}
	rep := summarize("t", 1, 1*time.Second, 3, 0, 0, lats)
	if rep.MinMs != 1 || rep.MaxMs != 3 {
		t.Errorf("min/max wrong: min=%f max=%f", rep.MinMs, rep.MaxMs)
	}
	if rep.MeanMs != 2 {
		t.Errorf("mean of 1,2,3 ms should be 2, got %f", rep.MeanMs)
	}
	if rep.RPS != 3 {
		t.Errorf("3 reqs / 1s should be 3 RPS, got %f", rep.RPS)
	}
}

// TestSummarize_NoLatencies asserts a graceful zero-report when no latencies.
func TestSummarize_NoLatencies(t *testing.T) {
	rep := summarize("t", 1, 1*time.Second, 0, 0, 0, nil)
	if rep.P50Ms != 0 || rep.MaxMs != 0 {
		t.Errorf("empty latencies must yield zero percentiles, got %+v", rep)
	}
}

// TestMs verifies the nanosecond->millisecond conversion.
func TestMs(t *testing.T) {
	if got := ms(1500 * time.Microsecond); got != 1.5 {
		t.Errorf("ms(1500us) = %f, want 1.5", got)
	}
}

// TestDoRequest_Drain verifies doRequest measures a real round-trip and drains.
func TestDoRequest_Drain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload-body-to-drain"))
	}))
	defer srv.Close()

	res := doRequest(context.Background(), srv.Client(), srv.URL)
	if res.err {
		t.Fatalf("doRequest against live server should not error")
	}
	if res.status != http.StatusOK {
		t.Errorf("expected 200, got %d", res.status)
	}
	if res.latency <= 0 {
		t.Errorf("latency must be measured >0, got %v", res.latency)
	}
}
