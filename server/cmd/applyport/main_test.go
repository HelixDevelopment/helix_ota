package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var applyportBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "applyport-smoke-*")
	if err != nil {
		panic(err)
	}
	applyportBin = filepath.Join(tmp, "applyport")
	build := exec.Command("go", "build", "-o", applyportBin, ".")
	build.Stderr = os.Stderr
	if buildErr := build.Run(); buildErr != nil {
		os.RemoveAll(tmp)
		panic("applyport: build failed: " + buildErr.Error())
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Approach (a): in-process run() dispatch — the unknown-command path returns an
// error WITHOUT touching the network, a device, or os.Exit, so it is safe to
// call directly. This exercises run()'s flag construction + subcommand switch.
// ---------------------------------------------------------------------------

// TestRun_UnknownCommand asserts run() rejects an unknown subcommand with an
// error (not a panic, not a silent success) via the real flag-parse + dispatch.
func TestRun_UnknownCommand(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"applyport", "definitely-not-a-command"}

	err := run()
	if err == nil {
		t.Fatal("run() with an unknown command must return an error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' error, got %v", err)
	}
}

// TestRun_CheckCommand exercises the 'check' subcommand dispatch path through
// run(). Without a running server, runCheck will return a register error — that
// is expected and proves the function actually attempts a real check rather than
// silently logging a placeholder.
func TestRun_CheckCommand(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"applyport", "check"}

	err := run()
	if err == nil {
		t.Fatal("run() 'check' must return an error when no server is reachable (was a placeholder no-op)")
	}
}

// TestRun_SubcommandFlagsAreParsed proves that a flag supplied AFTER the
// subcommand — the documented usage form `applyport <cmd> [flags]` — actually
// reaches run(). RED baseline (§11.4.115): the pre-fix code calls flag.Parse()
// (which parses os.Args[1:]) and stops at the subcommand token, silently
// dropping EVERY following flag, so an operator-supplied -server (or -pubkey,
// -username, -password, -insecure) reverts to its default/env value. A dropped
// -pubkey silently DISABLES signature verification the operator asked for.
func TestRun_SubcommandFlagsAreParsed(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	const want = "http://flag-after-subcommand.example/api/v1"
	// runCheck now performs a real network call (OTA-039); without a running
	// server it will return an error — that is fine. What matters is that the
	// -server flag was PARSED and populated from the post-subcommand position.
	os.Args = []string{"applyport", "check", "-server", want}

	_ = run() // error is expected (no server reachable)

	got := flag.CommandLine.Lookup("server").Value.String()
	if got != want {
		t.Fatalf("flag supplied after the subcommand was dropped: -server = %q, want %q "+
			"(flags after `applyport <cmd>` are silently ignored)", got, want)
	}
}

// ---------------------------------------------------------------------------
// Approach (a): pure helper coverage.
// ---------------------------------------------------------------------------

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("HELIX_TEST_KEY", "set-value")
	if got := envOrDefault("HELIX_TEST_KEY", "fallback"); got != "set-value" {
		t.Errorf("set var should win, got %q", got)
	}
	if got := envOrDefault("HELIX_TEST_UNSET_KEY", "fallback"); got != "fallback" {
		t.Errorf("unset var should fall back, got %q", got)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("B1", "1")
	t.Setenv("B2", "true")
	t.Setenv("B3", "no")
	if !envBool("B1") || !envBool("B2") {
		t.Error("'1' and 'true' must parse true")
	}
	if envBool("B3") || envBool("UNSET_BOOL") {
		t.Error("non-truthy / unset must parse false")
	}
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("D_GOOD", "5s")
	t.Setenv("D_BAD", "not-a-duration")
	if got := envDuration("D_GOOD", time.Minute); got != 5*time.Second {
		t.Errorf("good duration should parse, got %v", got)
	}
	if got := envDuration("D_BAD", time.Minute); got != time.Minute {
		t.Errorf("bad duration should fall back, got %v", got)
	}
	if got := envDuration("D_UNSET", 2*time.Hour); got != 2*time.Hour {
		t.Errorf("unset should fall back, got %v", got)
	}
}

func TestBuildVerifier(t *testing.T) {
	// Empty key -> verifier with no key configured (signature disabled).
	v, err := buildVerifier("")
	if err != nil {
		t.Fatalf("empty pubkey should not error, got %v", err)
	}
	if v.KeyConfigured() {
		t.Error("empty pubkey must leave verifier unconfigured")
	}

	// Valid 32-byte ed25519 key -> configured verifier.
	pub, _, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		t.Fatalf("keygen: %v", genErr)
	}
	v2, err := buildVerifier(base64.StdEncoding.EncodeToString(pub))
	if err != nil {
		t.Fatalf("valid pubkey should not error, got %v", err)
	}
	if !v2.KeyConfigured() {
		t.Error("valid pubkey must configure the verifier")
	}

	// Not base64 -> error.
	if _, err := buildVerifier("!!!not-base64!!!"); err == nil {
		t.Error("invalid base64 pubkey must error")
	}
	// Wrong length -> error.
	if _, err := buildVerifier(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("wrong-length pubkey must error")
	}
}

func TestClientOpt(t *testing.T) {
	// Both branches return a non-nil ClientOption; we just exercise them.
	if clientOpt(true) == nil || clientOpt(false) == nil {
		t.Error("clientOpt must always return an option")
	}
}

func TestSha256Hex(t *testing.T) {
	// SHA256 of empty input is a known constant.
	const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := sha256Hex(nil); got != emptyHash {
		t.Errorf("sha256Hex(nil) = %s, want %s", got, emptyHash)
	}
}

func TestDetectCurrentVersion_Default(t *testing.T) {
	// On the dev host /etc/helix-version does not exist -> placeholder "0.0.0".
	if got := detectCurrentVersion(); got == "" {
		t.Error("detectCurrentVersion must never return empty")
	}
}

// ---------------------------------------------------------------------------
// Approach (b): build + exec smoke of the no-args usage path (main->run prints
// usage to stderr and exits 1 — cannot be driven in-process because of os.Exit).
// ---------------------------------------------------------------------------

func TestSmoke_NoArgsUsage(t *testing.T) {
	cmd := exec.Command(applyportBin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("no-args invocation must exit non-zero, got err=%v", err)
	}
	if ee.ExitCode() != 1 {
		t.Errorf("no-args exit code should be 1, got %d", ee.ExitCode())
	}
	usage := stderr.String()
	if !strings.Contains(usage, "Usage: applyport") {
		t.Errorf("expected usage banner on stderr, got %q", usage)
	}
	for _, sub := range []string{"run", "daemon", "check", "confirm", "status"} {
		if !strings.Contains(usage, sub) {
			t.Errorf("usage should list subcommand %q; got %q", sub, usage)
		}
	}
}

// TestSmoke_UnknownCommandExec confirms the unknown-command path also surfaces
// via the real binary (main() wraps run()'s error in log.Fatalf -> exit 1).
func TestSmoke_UnknownCommandExec(t *testing.T) {
	cmd := exec.Command(applyportBin, "bogus-cmd")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unknown command must exit non-zero, got err=%v", err)
	}
	if ee.ExitCode() != 1 {
		t.Errorf("unknown command exit code should be 1, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("expected 'unknown command' on stderr, got %q", stderr.String())
	}
}
