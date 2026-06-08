package store

import (
	"context"
	"errors"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// runRepositoryContract exercises the behavioural contract every Repository
// implementation MUST satisfy. It is run against the in-memory repository here
// and against the pgx/PostgreSQL repository in the (containerised) integration
// test, proving behavioural parity between the two.
//
// The caller MUST pass a freshly-emptied repository.
func runRepositoryContract(t *testing.T, repo Repository) {
	t.Helper()
	ctx := context.Background()
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	// --- devices ---
	d1 := Device{
		DeviceID: "dev-1", HardwareID: "HW-1", Model: "OrangePi5Max",
		OSType: otaprotocol.OSAndroid, OSVersion: "15", CurrentVersion: "1.0.0",
		Group: "g1", Metadata: map[string]string{"site": "lab"}, RegisteredAt: ts,
		HealthOK: true, ActiveSlot: "a",
	}
	if err := repo.CreateDevice(ctx, d1); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	got, err := repo.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.HardwareID != "HW-1" || got.Group != "g1" || got.CurrentVersion != "1.0.0" ||
		got.Metadata["site"] != "lab" || !got.HealthOK || got.ActiveSlot != "a" {
		t.Fatalf("GetDevice round-trip mismatch: %+v", got)
	}
	if byHW, err := repo.GetDeviceByHardwareID(ctx, "HW-1"); err != nil || byHW.DeviceID != "dev-1" {
		t.Fatalf("GetDeviceByHardwareID: %+v err=%v", byHW, err)
	}
	if _, err := repo.GetDevice(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDevice unknown want ErrNotFound, got %v", err)
	}
	// Duplicate hardware id bound to a DIFFERENT device id -> conflict.
	if err := repo.CreateDevice(ctx, Device{DeviceID: "dev-2", HardwareID: "HW-1",
		OSType: otaprotocol.OSAndroid, RegisteredAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate hardware id want ErrConflict, got %v", err)
	}
	// Update existing.
	d1.CurrentVersion = "1.1.0"
	d1.UpdateState = "success"
	if err := repo.UpdateDevice(ctx, d1); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}
	if got, _ := repo.GetDevice(ctx, "dev-1"); got.CurrentVersion != "1.1.0" || got.UpdateState != "success" {
		t.Fatalf("UpdateDevice not applied: %+v", got)
	}
	if err := repo.UpdateDevice(ctx, Device{DeviceID: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateDevice unknown want ErrNotFound, got %v", err)
	}

	// --- artifacts ---
	a1 := Artifact{
		ArtifactID: "art-1", SHA256: "abc123", Size: 4096, OSType: otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max", Version: "1.1.0", StorageRef: "s3://x/art-1",
		Verified: true, UploadedAt: ts, Signature: "sig",
		PayloadProperties: otaprotocol.PayloadProperties{
			FileHash: "fh", FileSize: 4096, MetadataHash: "mh", MetadataSize: 64,
		},
	}
	if err := repo.CreateArtifact(ctx, a1); err != nil {
		t.Fatalf("CreateArtifact: %v", err)
	}
	gotA, err := repo.GetArtifact(ctx, "art-1")
	if err != nil || gotA.SHA256 != "abc123" || !gotA.Verified ||
		gotA.PayloadProperties.FileHash != "fh" || gotA.PayloadProperties.MetadataSize != 64 {
		t.Fatalf("GetArtifact round-trip mismatch: %+v err=%v", gotA, err)
	}
	if _, err := repo.GetArtifact(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetArtifact unknown want ErrNotFound, got %v", err)
	}

	// --- releases ---
	r1 := Release{ReleaseID: "rel-1", ArtifactID: "art-1", Version: "1.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published", CreatedAt: ts}
	r2 := Release{ReleaseID: "rel-2", ArtifactID: "art-1", Version: "1.2.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published", CreatedAt: ts}
	if err := repo.CreateRelease(ctx, r1); err != nil {
		t.Fatalf("CreateRelease r1: %v", err)
	}
	if err := repo.CreateRelease(ctx, r2); err != nil {
		t.Fatalf("CreateRelease r2: %v", err)
	}
	if got, err := repo.GetRelease(ctx, "rel-1"); err != nil || got.Version != "1.0.0" {
		t.Fatalf("GetRelease: %+v err=%v", got, err)
	}
	// LatestRelease uses the dotted comparator: 1.2.0 > 1.0.0.
	if latest, err := repo.LatestRelease(ctx, otaprotocol.OSAndroid, "OrangePi5Max"); err != nil || latest.Version != "1.2.0" {
		t.Fatalf("LatestRelease want 1.2.0, got %+v err=%v", latest, err)
	}
	if _, err := repo.LatestRelease(ctx, otaprotocol.OSAndroid, "NoSuchBoard"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LatestRelease unknown want ErrNotFound, got %v", err)
	}
	// List in insertion order.
	list, next, err := repo.ListReleases(ctx, ReleaseFilter{OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max"})
	if err != nil || len(list) != 2 || list[0].ReleaseID != "rel-1" || list[1].ReleaseID != "rel-2" || next != "" {
		t.Fatalf("ListReleases all: %+v next=%q err=%v", list, next, err)
	}
	// Paging with limit 1.
	p1, n1, err := repo.ListReleases(ctx, ReleaseFilter{Limit: 1})
	if err != nil || len(p1) != 1 || p1[0].ReleaseID != "rel-1" || n1 == "" {
		t.Fatalf("ListReleases page1: %+v next=%q err=%v", p1, n1, err)
	}
	p2, n2, err := repo.ListReleases(ctx, ReleaseFilter{Limit: 1, Cursor: n1})
	if err != nil || len(p2) != 1 || p2[0].ReleaseID != "rel-2" || n2 != "" {
		t.Fatalf("ListReleases page2: %+v next=%q err=%v", p2, n2, err)
	}

	// --- deployments ---
	dep := Deployment{DeploymentID: "dep-1", ReleaseID: "rel-1", Strategy: "all-targets",
		Group: "g1", Status: string(otaprotocol.DeploymentActive), TargetCount: 3, CreatedAt: ts}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if got, err := repo.GetDeployment(ctx, "dep-1"); err != nil || got.TargetCount != 3 {
		t.Fatalf("GetDeployment: %+v err=%v", got, err)
	}
	if act, err := repo.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "g1"); err != nil || act.DeploymentID != "dep-1" {
		t.Fatalf("ActiveDeploymentForTarget match: %+v err=%v", act, err)
	}
	// A different, non-empty group does not match.
	if _, err := repo.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ActiveDeploymentForTarget other-group want ErrNotFound, got %v", err)
	}
	if all, err := repo.ListActiveDeployments(ctx); err != nil || len(all) != 1 || all[0].DeploymentID != "dep-1" {
		t.Fatalf("ListActiveDeployments: %+v err=%v", all, err)
	}

	// --- telemetry ---
	if err := repo.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "dev-1", DeploymentID: "dep-1",
		Event: otaprotocol.EventDownloadStarted, Version: "1.1.0", Timestamp: ts, ReceivedAt: ts}); err != nil {
		t.Fatalf("AppendTelemetry 1: %v", err)
	}
	if err := repo.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "dev-1", DeploymentID: "dep-1",
		Event: otaprotocol.EventSuccess, Version: "1.1.0", ErrorCode: "", Detail: "ok", Timestamp: ts, ReceivedAt: ts}); err != nil {
		t.Fatalf("AppendTelemetry 2: %v", err)
	}
	evs, err := repo.TelemetryForDeployment(ctx, "dep-1")
	if err != nil || len(evs) != 2 || evs[0].Event != otaprotocol.EventDownloadStarted || evs[1].Event != otaprotocol.EventSuccess {
		t.Fatalf("TelemetryForDeployment: %+v err=%v", evs, err)
	}
	if evs, err := repo.TelemetryForDeployment(ctx, "no-dep"); err != nil || len(evs) != 0 {
		t.Fatalf("TelemetryForDeployment empty: %+v err=%v", evs, err)
	}

	// --- idempotency ---
	if _, ok := repo.GetIdempotent(ctx, "k1"); ok {
		t.Fatalf("GetIdempotent before put should be absent")
	}
	repo.PutIdempotent(ctx, "k1", "res-1")
	if id, ok := repo.GetIdempotent(ctx, "k1"); !ok || id != "res-1" {
		t.Fatalf("GetIdempotent after put: id=%q ok=%v", id, ok)
	}
	// A second put for the same key does not overwrite.
	repo.PutIdempotent(ctx, "k1", "res-2")
	if id, _ := repo.GetIdempotent(ctx, "k1"); id != "res-1" {
		t.Fatalf("PutIdempotent must not overwrite: got %q", id)
	}
}

// TestMemoryRepositoryContract runs the shared contract against the in-memory
// repository (always, no infra needed).
func TestMemoryRepositoryContract(t *testing.T) {
	runRepositoryContract(t, NewMemoryRepository())
}
