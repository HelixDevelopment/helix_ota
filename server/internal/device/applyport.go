// Package device provides the in-guest ApplyPort for U-Boot A/B targets.
// This is the Linux/U-Boot analogue of ReflectiveUpdateEngineApplyPort
// from ota-android-agent. It drives the apply sequence (steps b-f) that
// a real guest OTA agent executes: write the new rootfs to the inactive
// slot, arm the U-Boot env (BOOT_ORDER/upgrade_available/bootcount), then
// reboot so the bootloader selects the new slot.
//
// Design: docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md
//
// Status (§11.4.6): SCAFFOLD — the interfaces and struct exist and compile;
// the ApplyPort methods are stubs. The fw_setenv/fw_printenv dependency
// (device/fwenv.go) is authored with the real shell-out; the slot-writer
// and full ApplyPort driver (Step a → f loop) are DESIGNED but NOT wired
// — they wait on the /etc/fw_env.config verification (PWU-AB-4 §3) and
// the RAUC bundle-build (PWU-AB-2). See docs/qa/20260619-pwu-ab4-scaffold
// for the scaffold evidence.

package device

import (
	"context"
	"fmt"
)

// SlotWriter writes a rootfs image to the inactive slot partition.
// The active slot is known from /proc/cmdline (helix_slot=<A|B>) or
// /etc/slot_id; inactive = the other one.
//
// Implementations:
//   - RAUC: rauc install <bundle.raucb> for the dm-verity path (PWU-AB-2).
//     RAUC writes the inactive slot AND owns the slot-state (BOOT_ORDER).
//   - dd-class: a direct partition writer for the pre-verity / no-RAUC-bundle
//     path, mirroring assemble_ab_disk.sh:204-205.
type SlotWriter interface {
	// WriteInactiveSlot writes the rootfs image at imagePath to the
	// currently-inactive slot partition. Returns the slot name ("A"|"B")
	// that was written and any error.
	WriteInactiveSlot(ctx context.Context, imagePath string) (string, error)

	// ActiveSlot returns the currently-active slot name ("A"|"B").
	ActiveSlot() (string, error)

	// InactiveSlot returns the currently-inactive slot name ("A"|"B").
	InactiveSlot() (string, error)
}

// UBootEnvManager reads/writes the U-Boot environment from Linux userspace.
// The canonical Linux tool is fw_setenv/fw_printenv from u-boot-tools
// (libubootenv), driven by /etc/fw_env.config.
//
// The env variables this manager touches:
//   - BOOT_ORDER        — space-separated slot list; HEAD = slot tried first.
//   - upgrade_available — 1 = update on probation; 0 = confirmed-good.
//   - bootcount         — attempt counter (U-Boot increments it on each reboot
//     while upgrade_available != 0).
//   - BOOT_<A|B>_LEFT   — per-slot remaining attempts (RAUC convention).
type UBootEnvManager interface {
	// SetEnv sets a single U-Boot environment variable.
	SetEnv(key, value string) error

	// GetEnv reads a single U-Boot environment variable. Returns the value
	// and any error. Returns ("", nil) when the variable is not set.
	GetEnv(key string) (string, error)

	// SaveEnv explicitly flushes the environment to persistent storage.
	// Some backends (e.g. raw env region) commit on every SetEnv; this is
	// an explicit flush for backends that batch.
	SaveEnv() error
}

// ApplyPort drives the apply sequence for U-Boot A/B targets.
// It is the in-guest analogue of the ota-android-agent ApplyPort interface.
//
// The full sequence (design doc §2, steps a-f):
//
//	(a) Receive apply instruction   -> GET /client/update (protocol client)
//	(b) Write new rootfs            -> SlotWriter.WriteInactiveSlot
//	(c) Arm U-Boot env              -> UBootEnvManager (BOOT_ORDER, upgrade_available, bootcount)
//	(d) Reboot                      -> reboot (guest)
//	(e) Healthy-boot confirm        -> helix-ab-confirm script (clears upgrade_available, bootcount)
//	(f) Report telemetry            -> POST /client/telemetry (protocol client)
//
// Steps (b) and (c) are the ApplyPort's responsibility; steps (d)-(f) are
// orchestrated by the worker (the guest OTA agent).
type ApplyPort struct {
	writer   SlotWriter
	env      UBootEnvManager
	rootPart string // e.g. "/dev/vda2" (A) or "/dev/vda3" (B)
}

// NewApplyPort creates a new ApplyPort for the given SlotWriter and
// UBootEnvManager. The rootPart is the active root device path read from
// /proc/cmdline (root=/dev/vdaX) at construction time.
func NewApplyPort(writer SlotWriter, envManager UBootEnvManager, rootPart string) *ApplyPort {
	return &ApplyPort{
		writer:   writer,
		env:      envManager,
		rootPart: rootPart,
	}
}

// WriteAndArm writes the new rootfs to the inactive slot and arms the U-Boot
// environment so the next boot selects the new slot. This implements steps
// (b) + (c) of the apply sequence.
//
// After a successful return, the caller should reboot the guest (step d).
func (p *ApplyPort) WriteAndArm(ctx context.Context, imagePath string) error {
	// Step (b): write the new rootfs to the inactive slot.
	writtenSlot, err := p.writer.WriteInactiveSlot(ctx, imagePath)
	if err != nil {
		return fmt.Errorf("device apply: write inactive slot: %w", err)
	}

	// Step (c): arm the U-Boot environment so the bootloader tries the
	// new slot first on the next boot.
	// BOOT_ORDER = "<new slot> <current slot>"  — e.g. "B A" or "A B"
	currentSlot, err := p.writer.ActiveSlot()
	if err != nil {
		return fmt.Errorf("device apply: read active slot: %w", err)
	}

	newOrder := fmt.Sprintf("%s %s", writtenSlot, currentSlot)
	if err := p.env.SetEnv("BOOT_ORDER", newOrder); err != nil {
		return fmt.Errorf("device apply: set BOOT_ORDER: %w", err)
	}

	// upgrade_available=1 tells U-Boot to increment bootcount on each
	// reboot while the probation window is open.
	if err := p.env.SetEnv("upgrade_available", "1"); err != nil {
		return fmt.Errorf("device apply: set upgrade_available: %w", err)
	}

	// bootcount=1 initialises the attempt counter. U-Boot increments it
	// on each reboot while upgrade_available != 0.
	if err := p.env.SetEnv("bootcount", "1"); err != nil {
		return fmt.Errorf("device apply: set bootcount: %w", err)
	}

	// Persist the environment so it survives the reboot.
	if err := p.env.SaveEnv(); err != nil {
		return fmt.Errorf("device apply: save env: %w", err)
	}

	return nil
}

// ActiveSlot returns the slot name ("A"|"B") from the ApplyPort's rootPart.
// This is a convenience that delegates to the SlotWriter.
func (p *ApplyPort) ActiveSlot() (string, error) {
	return p.writer.ActiveSlot()
}

// InactiveSlot returns the inactive slot name ("A"|"B").
func (p *ApplyPort) InactiveSlot() (string, error) {
	return p.writer.InactiveSlot()
}
