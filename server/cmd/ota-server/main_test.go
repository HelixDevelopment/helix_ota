package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
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
// Approach (a): pure helper coverage.
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
// Approach (b): build + exec smoke of the config-error startup path.
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

// ---------------------------------------------------------------------------
// OTA-032: graceful SIGTERM drain.
//
// RED (before the signal-handling fix): starting the server and sending
// SIGTERM would kill it immediately — no drain, in-flight requests dropped,
// exit code -1 (signalled).  The pre-fix ListenAndServe blocked forever so the
// happy-path was never smoke-testable at the process level; the fact that no
// graceful-stop test existed IS the RED.
//
// GREEN (this test): the server catches SIGTERM, drains for up to 30 s
// (returning 503 to new requests), shuts down cleanly, and exits 0.
// ---------------------------------------------------------------------------

func TestGracefulShutdown_SIGTERM_ExitsCleanly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not supported on Windows")
	}

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cmd := exec.Command(otaServerBin)
	cmd.Env = append(os.Environ(),
		"HELIX_PORT="+fmt.Sprint(port),
		"HELIX_POLL_INTERVAL=5s",
		"HELIX_API_BASE_PATH=/api/v1",
		"HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET=1",
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	// Wait for the health endpoint to become reachable.
	healthURL := fmt.Sprintf("http://%s/healthz", addr)
	if !waitForHTTP(t, healthURL, 5*time.Second) {
		killAndWait(cmd)
		t.Fatalf("server did not become healthy within 5s\nstderr:\n%s", stderr.String())
	}

	// Send SIGTERM to trigger graceful drain.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		killAndWait(cmd)
		t.Fatalf("send SIGTERM: %v", err)
	}

	// Wait for the process to exit (drain timeout is 30 s, 35 s is safe).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("server should exit cleanly after SIGTERM drain, got: %v", err)
		}
		out := stderr.String()
		if !strings.Contains(out, "draining") {
			t.Errorf("expected drain log message, got stderr:\n%s", out)
		}
		if !strings.Contains(out, "stopped") {
			t.Errorf("expected 'stopped' log message, got stderr:\n%s", out)
		}
	case <-time.After(35 * time.Second):
		killAndWait(cmd)
		t.Fatal("server did not exit within 35 s after SIGTERM (drain hung?)")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// freePort returns a free TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitForHTTP polls url until it returns 200 or the deadline is reached.
func waitForHTTP(t *testing.T, url string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// killAndWait sends SIGKILL and waits for the process to exit.
func killAndWait(cmd *exec.Cmd) {
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()
}
