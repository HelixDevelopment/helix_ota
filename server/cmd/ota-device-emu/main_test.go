package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var emuBin string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ota-device-emu-smoke-*")
	if err != nil {
		panic(err)
	}
	emuBin = filepath.Join(tmp, "ota-device-emu")
	build := exec.Command("go", "build", "-o", emuBin, ".")
	build.Stderr = os.Stderr
	if buildErr := build.Run(); buildErr != nil {
		os.RemoveAll(tmp)
		panic("ota-device-emu: build failed: " + buildErr.Error())
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Approach (b) ONLY: main() is the sole function and it drives flag.CommandLine
// + os.Exit (via fatal()), so there is no in-process seam to call safely. These
// build+exec smoke tests assert main() parses flags and surfaces input errors
// with the right exit codes — no external server needed (all failures happen
// before any HTTP call).
// ---------------------------------------------------------------------------

// TestSmoke_MissingHardwareID asserts the required -hardware-id is enforced:
// deviceemu.New() returns an error which main()->fatal() reports as exit 1.
func TestSmoke_MissingHardwareID(t *testing.T) {
	cmd := exec.Command(emuBin) // no -hardware-id
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("missing -hardware-id must exit non-zero, got err=%v", err)
	}
	if ee.ExitCode() != 1 {
		t.Errorf("missing -hardware-id exit code should be 1, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "ota-device-emu:") {
		t.Errorf("expected a fatal() error prefix on stderr, got %q", stderr.String())
	}
}

// TestSmoke_Help asserts the standard flag -h path prints usage to stderr and
// exits 0. (Go's flag.ExitOnError treats -h as flag.ErrHelp, which prints the
// auto-generated usage and exits 0 — verified empirically, not assumed.)
func TestSmoke_Help(t *testing.T) {
	cmd := exec.Command(emuBin, "-h")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	// -h is the help flag: flag.ErrHelp -> usage printed, exit code 0 (err == nil).
	if err != nil {
		t.Fatalf("-h should exit 0 (flag.ErrHelp convention), got err=%v", err)
	}
	usage := stderr.String()
	if !strings.Contains(usage, "Usage of") {
		t.Errorf("expected a usage banner on stderr, got %q", usage)
	}
	for _, fl := range []string{"-base", "-hardware-id", "-model", "-interval"} {
		if !strings.Contains(usage, fl) {
			t.Errorf("usage should document flag %q; got %q", fl, usage)
		}
	}
}

// TestSmoke_BadFlag asserts an unknown flag is rejected with exit 2.
func TestSmoke_BadFlag(t *testing.T) {
	cmd := exec.Command(emuBin, "-no-such-flag")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unknown flag must exit non-zero, got err=%v", err)
	}
	if ee.ExitCode() != 2 {
		t.Errorf("unknown flag exit code should be 2, got %d", ee.ExitCode())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("expected flag-undefined error, got %q", stderr.String())
	}
}
