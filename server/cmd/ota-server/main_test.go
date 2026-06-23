package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var otaServerBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ota-server-smoke-*")
	if err != nil {
		panic(err)
	}
	otaServerBin = filepath.Join(tmp, "ota-server")
	build := exec.Command("go", "build", "-o", otaServerBin, ".")
	build.Stderr = os.Stderr
	if buildErr := build.Run(); buildErr != nil {
		os.RemoveAll(tmp)
		panic("ota-server: build failed: " + buildErr.Error())
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Approach (a): pure helper coverage (the only in-process-safe seam — main()
// itself blocks on ListenAndServe and cannot be driven without a kill).
// ---------------------------------------------------------------------------

func TestGetEnvDefault(t *testing.T) {
	t.Setenv("HELIX_TEST_ADMIN", "admin-set")
	if got := getEnvDefault("HELIX_TEST_ADMIN", "fallback"); got != "admin-set" {
		t.Errorf("set var should win, got %q", got)
	}
	if got := getEnvDefault("HELIX_TEST_ADMIN_UNSET", "fallback"); got != "fallback" {
		t.Errorf("unset var should fall back, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Approach (b): build + exec smoke of the config-error startup path. A malformed
// HELIX_POLL_INTERVAL makes config.Load() fail, so main() log.Fatalf's (exit 1)
// BEFORE binding any TCP port — a deterministic, non-blocking smoke of main().
// ---------------------------------------------------------------------------

func TestSmoke_BadConfigExits(t *testing.T) {
	cmd := exec.Command(otaServerBin)
	cmd.Env = append(os.Environ(), "HELIX_POLL_INTERVAL=not-a-duration")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("bad-config startup must exit non-zero (not block/panic), got err=%v", err)
	}
	if ee.ExitCode() != 1 {
		t.Errorf("config-error exit code should be 1, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "config") {
		t.Errorf("expected a config error on stderr, got %q", stderr.String())
	}
}

// NOTE (honest §11.4.6 boundary): the ota-server happy path is NOT smoke-tested
// here. main() has no --help/--version flag and immediately calls
// http.Server.ListenAndServe (or the HTTP/3 transport.Start), which blocks
// forever; driving it would require binding a real port and then killing the
// process, which is a server-boot integration concern covered by the
// internal/api + internal/transport test suites and the e2e stream — not a
// lightweight unit smoke. Only the deterministic config-error exit path is
// asserted at the binary level above.
