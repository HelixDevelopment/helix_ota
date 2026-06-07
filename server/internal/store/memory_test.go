package store

import (
	"context"
	"errors"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

func TestDeviceCRUDAndConflict(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	d := Device{DeviceID: "dev-1", HardwareID: "hw-1", Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid}
	if err := r.CreateDevice(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := r.GetDevice(ctx, "dev-1")
	if err != nil || got.HardwareID != "hw-1" {
		t.Fatalf("get: %v %+v", err, got)
	}
	byHW, err := r.GetDeviceByHardwareID(ctx, "hw-1")
	if err != nil || byHW.DeviceID != "dev-1" {
		t.Fatalf("by hw: %v %+v", err, byHW)
	}

	// Same hardware id, different device id -> conflict.
	if err := r.CreateDevice(ctx, Device{DeviceID: "dev-2", HardwareID: "hw-1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}

	// Update existing.
	got.CurrentVersion = "1.2.0"
	if err := r.UpdateDevice(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := r.GetDevice(ctx, "dev-1")
	if reread.CurrentVersion != "1.2.0" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update unknown -> not found.
	if err := r.UpdateDevice(ctx, Device{DeviceID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := r.GetDevice(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestReleaseLatestAndList(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	versions := []string{"1.0.0", "1.10.0", "1.2.0"}
	for i, v := range versions {
		err := r.CreateRelease(ctx, Release{
			ReleaseID:   "rel-" + v,
			Version:     v,
			OSType:      otaprotocol.OSAndroid,
			TargetModel: "OrangePi5Max",
			Status:      "published",
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Second),
		})
		if err != nil {
			t.Fatalf("create release: %v", err)
		}
	}

	latest, err := r.LatestRelease(ctx, otaprotocol.OSAndroid, "OrangePi5Max")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	// Dotted-numeric: 1.10.0 is the highest.
	if latest.Version != "1.10.0" {
		t.Fatalf("latest want 1.10.0, got %s", latest.Version)
	}

	if _, err := r.LatestRelease(ctx, otaprotocol.OSAndroid, "Unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found for unknown target, got %v", err)
	}

	// Paginated list: limit 2 yields a next cursor.
	page1, next, err := r.ListReleases(ctx, ReleaseFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page1) != 2 || next == "" {
		t.Fatalf("page1 want 2 items + cursor, got %d items next=%q", len(page1), next)
	}
	page2, next2, _ := r.ListReleases(ctx, ReleaseFilter{Limit: 2, Cursor: next})
	if len(page2) != 1 || next2 != "" {
		t.Fatalf("page2 want 1 item + empty cursor, got %d items next=%q", len(page2), next2)
	}
}

func TestDeploymentAndTelemetry(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	_ = r.CreateRelease(ctx, Release{ReleaseID: "rel-1", Version: "1.1.0", OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max"})
	dep := Deployment{
		DeploymentID: "dep-1",
		ReleaseID:    "rel-1",
		Strategy:     "all-targets",
		Status:       string(otaprotocol.DeploymentActive),
		TargetCount:  3,
		CreatedAt:    time.Now(),
	}
	if err := r.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	active, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "")
	if err != nil || active.DeploymentID != "dep-1" {
		t.Fatalf("active lookup: %v %+v", err, active)
	}
	if list, _ := r.ListActiveDeployments(ctx); len(list) != 1 {
		t.Fatalf("active list want 1, got %d", len(list))
	}

	_ = r.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "d1", DeploymentID: "dep-1", Event: otaprotocol.EventSuccess, Timestamp: time.Now()})
	recs, err := r.TelemetryForDeployment(ctx, "dep-1")
	if err != nil || len(recs) != 1 {
		t.Fatalf("telemetry: %v %d", err, len(recs))
	}
}

func TestIdempotency(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	if _, ok := r.GetIdempotent(ctx, "k"); ok {
		t.Fatalf("unexpected idempotent hit")
	}
	r.PutIdempotent(ctx, "k", "result-1")
	if v, ok := r.GetIdempotent(ctx, "k"); !ok || v != "result-1" {
		t.Fatalf("idempotent miss: %v %q", ok, v)
	}
}

func TestArtifactCRUD(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	if _, err := r.GetArtifact(ctx, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	a := Artifact{ArtifactID: "a-1", SHA256: "deadbeef", Verified: true}
	if err := r.CreateArtifact(ctx, a); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	got, err := r.GetArtifact(ctx, "a-1")
	if err != nil || !got.Verified {
		t.Fatalf("get artifact: %v %+v", err, got)
	}
}
