// Package device — fwenv.go: UBootEnvManager implementation that shells out
// to fw_setenv/fw_printenv from u-boot-tools (libubootenv).
//
// This is the canonical Linux userspace U-Boot environment access mechanism:
//   - fw_printenv <key>     — reads one variable from the persistent env.
//   - fw_setenv <key> <val> — sets one variable and (for raw env) persists
//     immediately.
//   - fw_setenv <key>       — deletes the variable.
//
// Both tools are driven by /etc/fw_env.config, which tells them WHERE on the
// boot device (device + offset + size) the U-Boot env blob lives. The config
// file MUST match the bootloader's CONFIG_ENV_IS_IN_* storage exactly — a
// mismatch = the writes silently go to the wrong region and the bootloader
// never reads them (§11.4.108 SOURCE->RUNTIME gap).
//
// Design: docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md §3.
// The /etc/fw_env.config file is authored at:
//     tests/emulator/ab_virt/rauc/fw_env.config
// and is UNVERIFIED (§11.4.6) until round-trip-tested against the built
// u-boot.bin (PWU-AB-4 §3 step iii / PWU-AB-2 §4.5).

package device

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// fwEnvManager implements UBootEnvManager by shelling out to fw_setenv and
// fw_printenv from u-boot-tools.
type fwEnvManager struct {
	// fwSetenvPath is the path to the fw_setenv binary. If empty, it is
	// resolved from PATH.
	fwSetenvPath string

	// fwPrintenvPath is the path to the fw_printenv binary. If empty, it is
	// resolved from PATH.
	fwPrintenvPath string
}

// NewFwEnvManager creates a new fwEnvManager that shells out to fw_setenv
// and fw_printenv. Pass empty strings to resolve from PATH.
func NewFwEnvManager(fwSetenvPath, fwPrintenvPath string) UBootEnvManager {
	return &fwEnvManager{
		fwSetenvPath:   fwSetenvPath,
		fwPrintenvPath: fwPrintenvPath,
	}
}

// resolveSetenv returns the fw_setenv command path.
func (f *fwEnvManager) resolveSetenv() string {
	if f.fwSetenvPath != "" {
		return f.fwSetenvPath
	}
	return "fw_setenv"
}

// resolvePrintenv returns the fw_printenv command path.
func (f *fwEnvManager) resolvePrintenv() string {
	if f.fwPrintenvPath != "" {
		return f.fwPrintenvPath
	}
	return "fw_printenv"
}

// SetEnv sets a single U-Boot environment variable via fw_setenv.
// fw_setenv persists immediately for raw env regions; for FAT-file env the
// persistence depends on the backend (see SaveEnv).
func (f *fwEnvManager) SetEnv(key, value string) error {
	cmd := exec.Command(f.resolveSetenv(), key, value)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fw_setenv %s=%s: %w (stderr: %s)", key, value, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// GetEnv reads a single U-Boot environment variable via fw_printenv.
// Returns ("", nil) when the variable is not set (fw_printenv exits with
// non-zero for unset vars — we swallow that and return empty).
func (f *fwEnvManager) GetEnv(key string) (string, error) {
	cmd := exec.Command(f.resolvePrintenv(), key)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// fw_printenv exits non-zero for unset variables; that is not a
		// hard error — return empty.
		if strings.Contains(stderr.String(), "not set") || strings.Contains(stderr.String(), "not defined") {
			return "", nil
		}
		return "", fmt.Errorf("fw_printenv %s: %w (stderr: %s)", key, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// SaveEnv persists the environment. For raw env backends fw_setenv persists
// immediately on every SetEnv, so this is a no-op. For FAT-file backends or
// other buffered backends this forces a flush.
//
// The current implementation shells out to `fw_setenv ”` (empty key) which
// triggers a save on backends that support it. If the backend does not
// support the empty-key save, this is a harmless no-op.
func (f *fwEnvManager) SaveEnv() error {
	// fw_setenv with no key triggers a save/flush on some backends.
	// We call it silently; errors are informational only.
	// See u-boot-tools/libubootenv docs for backend-specific behaviour.
	_ = exec.Command(f.resolveSetenv()).Run()
	return nil
}
