package store

import (
	"context"
	"testing"
)

// TestDeviceMetadataIsolation proves CreateDevice/GetDevice/GetDeviceByHardwareID
// do not alias the caller's Metadata map. Device.Metadata is a reference type
// (map[string]string); storing/returning a Device by value still shares the
// underlying map with the caller unless it is cloned. Before this test's fix,
// (a) mutating the caller's own struct's Metadata map AFTER CreateDevice silently
// rewrote the stored record (write-side aliasing), and (b) mutating a map
// returned by GetDevice silently rewrote the stored record too (read-side
// aliasing) — neither requires calling UpdateDevice, so the persistence layer's
// "returns/stores a snapshot" contract was violated. memory_fabric.go's
// cloneStrMap already guards FabricNode.Labels against exactly this; Device
// (and AuditEntry/RollbackRecord, see below) had been missed.
func TestDeviceMetadataIsolation(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	d := Device{DeviceID: "dev-1", HardwareID: "hw-1", Metadata: map[string]string{"site": "lab"}}
	if err := r.CreateDevice(ctx, d); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	// Write-side: mutating the caller's own struct after CreateDevice must not
	// reach the stored record.
	d.Metadata["site"] = "tampered-by-caller"
	got, err := r.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.Metadata["site"] != "lab" {
		t.Fatalf("write-side aliasing: stored Metadata mutated via caller's map after CreateDevice: got %q, want %q",
			got.Metadata["site"], "lab")
	}

	// Read-side: mutating the map returned by GetDevice must not reach the
	// stored record either.
	got.Metadata["site"] = "tampered-by-reader"
	if got2, err := r.GetDevice(ctx, "dev-1"); err != nil || got2.Metadata["site"] != "lab" {
		t.Fatalf("read-side aliasing: stored Metadata mutated via returned map from GetDevice: got %+v err=%v", got2, err)
	}

	// GetDeviceByHardwareID returns the same isolation guarantee.
	byHW, err := r.GetDeviceByHardwareID(ctx, "hw-1")
	if err != nil {
		t.Fatalf("GetDeviceByHardwareID: %v", err)
	}
	byHW.Metadata["site"] = "tampered-via-byhw"
	if got3, _ := r.GetDevice(ctx, "dev-1"); got3.Metadata["site"] != "lab" {
		t.Fatalf("read-side aliasing via GetDeviceByHardwareID: got %+v", got3)
	}

	// UpdateDevice's write-side clone: mutate the caller's map after Update.
	upd := got
	upd.Metadata = map[string]string{"site": "lab2"}
	if err := r.UpdateDevice(ctx, upd); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}
	upd.Metadata["site"] = "tampered-after-update"
	if got4, _ := r.GetDevice(ctx, "dev-1"); got4.Metadata["site"] != "lab2" {
		t.Fatalf("write-side aliasing via UpdateDevice: got %+v", got4)
	}

	// ListDevices must return isolated copies too.
	list, _, err := r.ListDevices(ctx, DeviceFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("ListDevices: %+v err=%v", list, err)
	}
	list[0].Metadata["site"] = "tampered-via-list"
	if got5, _ := r.GetDevice(ctx, "dev-1"); got5.Metadata["site"] != "lab2" {
		t.Fatalf("read-side aliasing via ListDevices: got %+v", got5)
	}

	// AllDevices must return isolated copies too.
	all := r.AllDevices(ctx)
	if len(all) != 1 {
		t.Fatalf("AllDevices: want 1, got %d", len(all))
	}
	all[0].Metadata["site"] = "tampered-via-alldevices"
	if got6, _ := r.GetDevice(ctx, "dev-1"); got6.Metadata["site"] != "lab2" {
		t.Fatalf("read-side aliasing via AllDevices: got %+v", got6)
	}
}

// TestAuditDetailsIsolation proves AppendAudit/ListAudit clone AuditEntry.Details
// on both the write and read side, mirroring TestDeviceMetadataIsolation for the
// audit log's map field.
func TestAuditDetailsIsolation(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	e := AuditEntry{ID: "aud-1", Action: "DEVICE_REGISTER", Details: map[string]string{"model": "OrangePi5Max"}}
	if err := r.AppendAudit(ctx, e); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}

	// Write-side: mutate the caller's struct after Append.
	e.Details["model"] = "tampered-by-caller"
	list, _, err := r.ListAudit(ctx, AuditFilter{})
	if err != nil || len(list) != 1 {
		t.Fatalf("ListAudit: %+v err=%v", list, err)
	}
	if list[0].Details["model"] != "OrangePi5Max" {
		t.Fatalf("write-side aliasing on AuditEntry.Details: got %q want %q", list[0].Details["model"], "OrangePi5Max")
	}

	// Read-side: mutate the returned entry's map.
	list[0].Details["model"] = "tampered-by-reader"
	if list2, _, _ := r.ListAudit(ctx, AuditFilter{}); list2[0].Details["model"] != "OrangePi5Max" {
		t.Fatalf("read-side aliasing on AuditEntry.Details: got %+v", list2[0])
	}
}

// TestRollbackDetailsIsolation proves AppendRollback/ListRollbacks clone
// RollbackRecord.Details on both the write and read side, mirroring
// TestDeviceMetadataIsolation for the rollback/abort log's map field.
func TestRollbackDetailsIsolation(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	rb := RollbackRecord{ID: "rb-1", DeploymentID: "dep-1", Kind: "abort", Details: map[string]string{"error_rate": "0.5"}}
	if err := r.AppendRollback(ctx, rb); err != nil {
		t.Fatalf("AppendRollback: %v", err)
	}

	// Write-side: mutate the caller's struct after Append.
	rb.Details["error_rate"] = "tampered-by-caller"
	list, err := r.ListRollbacks(ctx, "dep-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRollbacks: %+v err=%v", list, err)
	}
	if list[0].Details["error_rate"] != "0.5" {
		t.Fatalf("write-side aliasing on RollbackRecord.Details: got %q want %q", list[0].Details["error_rate"], "0.5")
	}

	// Read-side: mutate the returned record's map.
	list[0].Details["error_rate"] = "tampered-by-reader"
	if list2, _ := r.ListRollbacks(ctx, "dep-1"); list2[0].Details["error_rate"] != "0.5" {
		t.Fatalf("read-side aliasing on RollbackRecord.Details: got %+v", list2[0])
	}
}

// TestUpdateDeviceReindexesHardwareID proves UpdateDevice keeps the devByHW
// secondary index consistent when a device's HardwareID changes. Before this
// test's fix, UpdateDevice overwrote m.devices[d.DeviceID] but never touched
// devByHW: the OLD hardware id kept resolving (stale) to this device via
// GetDeviceByHardwareID, and the NEW hardware id resolved to nothing at all —
// exactly the class of secondary-index staleness bug that UpdateGroup and
// UpdateProject already guard against for their own name indexes
// (grpByName/prjByName) by deleting the old key and setting the new one.
func TestUpdateDeviceReindexesHardwareID(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	d := Device{DeviceID: "dev-1", HardwareID: "HW-OLD"}
	if err := r.CreateDevice(ctx, d); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	d.HardwareID = "HW-NEW"
	if err := r.UpdateDevice(ctx, d); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	if got, err := r.GetDeviceByHardwareID(ctx, "HW-NEW"); err != nil || got.DeviceID != "dev-1" {
		t.Fatalf("GetDeviceByHardwareID(HW-NEW) after HardwareID change: got %+v err=%v", got, err)
	}
	if _, err := r.GetDeviceByHardwareID(ctx, "HW-OLD"); err == nil {
		t.Fatalf("GetDeviceByHardwareID(HW-OLD) should be gone after HardwareID change, but still resolves")
	}

	// A second device cannot steal the now-abandoned old hardware id's
	// binding... but it CAN reuse HW-OLD since it's genuinely free again.
	if err := r.CreateDevice(ctx, Device{DeviceID: "dev-2", HardwareID: "HW-OLD"}); err != nil {
		t.Fatalf("CreateDevice dev-2 reusing freed HW-OLD: %v", err)
	}

	// UpdateDevice must reject stealing a hardware id another device still owns.
	steal := Device{DeviceID: "dev-2", HardwareID: "HW-NEW"}
	if err := r.UpdateDevice(ctx, steal); err == nil {
		t.Fatalf("UpdateDevice want ErrConflict stealing dev-1's HW-NEW, got nil")
	} else if err != ErrConflict {
		t.Fatalf("UpdateDevice want ErrConflict, got %v", err)
	}
	// dev-1's binding must be untouched by the rejected update attempt.
	if got, err := r.GetDeviceByHardwareID(ctx, "HW-NEW"); err != nil || got.DeviceID != "dev-1" {
		t.Fatalf("HW-NEW binding corrupted by rejected UpdateDevice: got %+v err=%v", got, err)
	}
}
