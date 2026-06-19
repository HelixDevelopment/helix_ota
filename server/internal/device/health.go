// Package device -- health.go: healthy-boot confirmation marker.
//
// On a confirmed-healthy boot the marker clears upgrade_available and
// bootcount in the U-Boot environment, freezing the counter so the
// just-applied slot is CONFIRMED and no rollback fires (PWU-AB-4 SS4,
// uboot_ab/README.md:76-80).
//
// The marker is the in-guest application code the U-Boot
// docs/README:76-80 require: "the responsibility of some application code
// (typically a Linux application)" and is intentionally NOT in boot.cmd.
//
// Reuse (SS11.4.74): RAUC ships mark-good (rauc status mark-good) which in
// U-Boot bootchooser mode writes exactly these env vars. Where RAUC is the
// slot-writer, use rauc mark-good; this Go code is the fallback for the
// pre-RAUC-bundle / no-verity path.
package device

import (
	"fmt"
)

// HealthMarker confirms a healthy boot by clearing the U-Boot env probation
// counters. It is driven by a systemd oneshot unit or init script.
type HealthMarker struct {
	env UBootEnvManager
}

// NewHealthMarker creates a HealthMarker backed by the given UBootEnvManager.
func NewHealthMarker(env UBootEnvManager) *HealthMarker {
	return &HealthMarker{env: env}
}

// ConfirmHealthy reads the current upgrade_available state; if it is 1, it
// clears upgrade_available and bootcount to freeze the probation counter.
// It is idempotent and crash-safe: re-running it on an already-confirmed
// slot is a no-op.
func (m *HealthMarker) ConfirmHealthy() error {
	current, err := m.env.GetEnv("upgrade_available")
	if err != nil {
		return fmt.Errorf("health: read upgrade_available: %w", err)
	}

	if current != "1" {
		// Already confirmed or never armed -- nothing to do.
		return nil
	}

	if err := m.env.SetEnv("upgrade_available", "0"); err != nil {
		return fmt.Errorf("health: clear upgrade_available: %w", err)
	}

	if err := m.env.SetEnv("bootcount", "0"); err != nil {
		return fmt.Errorf("health: reset bootcount: %w", err)
	}

	if err := m.env.SaveEnv(); err != nil {
		return fmt.Errorf("health: save env after confirm: %w", err)
	}

	return nil
}

// IsArmed returns true when upgrade_available=1 (a slot is on probation).
func (m *HealthMarker) IsArmed() (bool, error) {
	current, err := m.env.GetEnv("upgrade_available")
	if err != nil {
		return false, fmt.Errorf("health: read upgrade_available: %w", err)
	}
	return current == "1", nil
}
