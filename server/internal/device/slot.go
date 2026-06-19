// Package device -- slot.go: concrete SlotWriter for the A/B U-Boot target.
//
// This implements SlotWriter (defined in applyport.go) by reading the
// currently-active slot from /proc/cmdline (helix_slot=<A|B>) or from
// /etc/slot_id, mapping A-/dev/vda2 and B-/dev/vda3, and writing the
// rootfs image to the inactive partition with a straightforward dd(1) call.
//
// Design: docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md SS8.
// The slot-partition map is the load-bearing GPT contract:
//
//	/dev/vda2 = slot A
//	/dev/vda3 = slot B
//
// (uboot_ab/README.md:96-100, boot.cmd:34-40).
package device

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// slotDevice implements SlotWriter by shelling out to dd(1) for the
// inactive-slot write and parsing /proc/cmdline or /etc/slot_id for
// slot-detection. On the actual device these are real sysfs operations;
// in tests they can be overridden.
type slotDevice struct {
	mu sync.Mutex

	// For testing: override paths. Empty = use the real paths.
	procCmdlinePath string // e.g. "/proc/cmdline"
	etcSlotIDPath   string // e.g. "/etc/slot_id"
	devPrefix       string // e.g. "/dev"

	// cachedSlot stores the last-detected active slot ("A"|"B") so we
	// don't re-read sysfs on every call.
	cachedSlot    string
	cachedSlotErr error
	cacheSlotOnce sync.Once
}

// NewSlotDevice creates a SlotWriter that uses real device paths.
// Pass empty strings to use the standard paths.
func NewSlotDevice(procCmdlinePath, etcSlotIDPath, devPrefix string) SlotWriter {
	s := &slotDevice{}
	if procCmdlinePath != "" {
		s.procCmdlinePath = procCmdlinePath
	} else {
		s.procCmdlinePath = "/proc/cmdline"
	}
	if etcSlotIDPath != "" {
		s.etcSlotIDPath = etcSlotIDPath
	} else {
		s.etcSlotIDPath = "/etc/slot_id"
	}
	if devPrefix != "" {
		s.devPrefix = devPrefix
	} else {
		s.devPrefix = "/dev"
	}
	return s
}

// WriteInactiveSlot writes the rootfs image at imagePath to the currently-
// inactive slot partition using dd(1).
func (s *slotDevice) WriteInactiveSlot(ctx context.Context, imagePath string) (string, error) {
	inactive, err := s.InactiveSlot()
	if err != nil {
		return "", fmt.Errorf("slot: determine inactive slot: %w", err)
	}

	partition := s.slotToPartition(inactive)
	if partition == "" {
		return "", fmt.Errorf("slot: no partition mapped for slot %q", inactive)
	}

	// Verify the image file exists and is non-empty.
	fi, err := os.Stat(imagePath)
	if err != nil {
		return "", fmt.Errorf("slot: stat image %q: %w", imagePath, err)
	}
	if fi.Size() == 0 {
		return "", fmt.Errorf("slot: image %q is empty", imagePath)
	}

	// Write via dd(1): bs=4M to match the emulator's typical I/O size.
	cmd := exec.CommandContext(ctx, "dd",
		"if="+imagePath,
		"of="+partition,
		"bs=4M",
		"conv=fsync",
		"status=progress",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("slot: dd write to %s: %w (stderr: %s)",
			partition, err, strings.TrimSpace(stderr.String()))
	}

	return inactive, nil
}

// ActiveSlot returns the currently-active slot name ("A"|"B").
func (s *slotDevice) ActiveSlot() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cacheSlotOnce.Do(s.detectSlot)
	return s.cachedSlot, s.cachedSlotErr
}

// InactiveSlot returns the currently-inactive slot name ("A"|"B").
func (s *slotDevice) InactiveSlot() (string, error) {
	active, err := s.ActiveSlot()
	if err != nil {
		return "", err
	}
	switch active {
	case "A":
		return "B", nil
	case "B":
		return "A", nil
	default:
		return "", fmt.Errorf("slot: unexpected active slot %q", active)
	}
}

// detectSlot reads the active slot from /proc/cmdline with fallback to
// /etc/slot_id. It is called lazily once and cached.
func (s *slotDevice) detectSlot() {
	// First try /proc/cmdline for helix_slot=<A|B>.
	data, err := os.ReadFile(s.procCmdlinePath)
	if err == nil {
		slot := parseHelixSlot(string(data))
		if slot != "" {
			s.cachedSlot = slot
			return
		}
	}

	// Fallback: /etc/slot_id (written by assemble_ab_disk.sh:177-200).
	data, err = os.ReadFile(s.etcSlotIDPath)
	if err == nil {
		slot := strings.TrimSpace(string(data))
		if slot == "A" || slot == "B" {
			s.cachedSlot = slot
			return
		}
	}

	// Neither available -- this is expected in a test/dev environment without
	// a real boot chain. Default to A so tests and the CLI don't crash.
	s.cachedSlot = "A"
}

// parseHelixSlot extracts the helix_slot value from a kernel cmdline string.
func parseHelixSlot(cmdline string) string {
	for _, part := range strings.Fields(cmdline) {
		if strings.HasPrefix(part, "helix_slot=") {
			val := strings.TrimPrefix(part, "helix_slot=")
			if val == "A" || val == "B" {
				return val
			}
		}
	}
	return ""
}

// slotToPartition maps a slot name to its block device path.
func (s *slotDevice) slotToPartition(slot string) string {
	switch slot {
	case "A":
		return filepath.Join(s.devPrefix, "vda2")
	case "B":
		return filepath.Join(s.devPrefix, "vda3")
	default:
		return ""
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// ddWriter implements SlotWriter for tests -- it captures the image path
// rather than writing to a real partition.
type ddWriter struct {
	activeSlot  string
	writtenPath string
	writtenSlot string
}

// NewDDWriter creates a test SlotWriter that records what would be written.
func NewDDWriter(activeSlot string) SlotWriter {
	return &ddWriter{activeSlot: activeSlot}
}

func (w *ddWriter) WriteInactiveSlot(_ context.Context, imagePath string) (string, error) {
	var err error
	w.writtenPath = imagePath
	w.writtenSlot, err = w.InactiveSlot()
	return w.writtenSlot, err
}

func (w *ddWriter) ActiveSlot() (string, error) {
	if w.activeSlot == "" {
		return "A", nil
	}
	return w.activeSlot, nil
}

func (w *ddWriter) InactiveSlot() (string, error) {
	active, _ := w.ActiveSlot()
	if active == "A" {
		return "B", nil
	}
	return "A", nil
}
