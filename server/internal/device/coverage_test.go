// Package device — coverage gap tests for uncovered simple functions.
package device

import (
	"context"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// slotToPartition — maps slot name to block device path.
// ---------------------------------------------------------------------------

func TestSlotToPartition(t *testing.T) {
	t.Parallel()

	s := NewSlotDevice("/nonexistent/cmdline", "/nonexistent/slot_id", "/dev").(*slotDevice)

	tests := []struct {
		slot string
		want string
	}{
		{"A", filepath.FromSlash("/dev/vda2")},
		{"B", filepath.FromSlash("/dev/vda3")},
		{"", ""},
		{"C", ""},
		{"invalid", ""},
	}

	for _, tc := range tests {
		got := s.slotToPartition(tc.slot)
		if got != tc.want {
			t.Errorf("slotToPartition(%q) = %q, want %q", tc.slot, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ApplyPort.InactiveSlot — delegates to SlotWriter.
// ---------------------------------------------------------------------------

func TestApplyPort_InactiveSlot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		activeSlot   string
		wantInactive string
	}{
		{"A", "B"},
		{"B", "A"},
	}

	for _, tc := range tests {
		t.Run("active="+tc.activeSlot, func(t *testing.T) {
			applier := NewApplyPort(NewDDWriter(tc.activeSlot), newMockEnvManager(), "/dev/vda2")
			slot, err := applier.InactiveSlot()
			if err != nil {
				t.Fatalf("InactiveSlot: %v", err)
			}
			if slot != tc.wantInactive {
				t.Fatalf("expected %q, got %q", tc.wantInactive, slot)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ApplyPortClient getters — DeviceID / DeviceToken.
// ---------------------------------------------------------------------------

func TestApplyPortClient_Getters(t *testing.T) {
	t.Parallel()

	client := NewApplyPortClient("http://example.com")

	// Before registration, getters return empty.
	if id := client.DeviceID(); id != "" {
		t.Fatalf("expected empty DeviceID before registration, got %q", id)
	}
	if tok := client.DeviceToken(); tok != "" {
		t.Fatalf("expected empty DeviceToken before registration, got %q", tok)
	}

	// After registration (simulated), getters return cached values.
	client.deviceID = "dev-001"
	client.deviceToken = "tok-abc"

	if id := client.DeviceID(); id != "dev-001" {
		t.Fatalf("expected dev-001, got %q", id)
	}
	if tok := client.DeviceToken(); tok != "tok-abc" {
		t.Fatalf("expected tok-abc, got %q", tok)
	}
}

// WriteInactiveSlot stub test for the ddWriter to ensure coverage.
func TestDDWriter_WriteInactiveSlot_FromB(t *testing.T) {
	t.Parallel()

	w := NewDDWriter("B")
	slot, err := w.WriteInactiveSlot(context.Background(), "/some/image.img")
	if err != nil {
		t.Fatalf("WriteInactiveSlot: %v", err)
	}
	if slot != "A" {
		t.Fatalf("expected inactive A when active is B, got %q", slot)
	}
	dw := w.(*ddWriter)
	if dw.writtenPath != "/some/image.img" {
		t.Fatalf("expected writtenPath=/some/image.img, got %q", dw.writtenPath)
	}
}
