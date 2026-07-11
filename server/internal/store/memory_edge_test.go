package store

import (
	"context"
	"errors"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// TestCloneStrMapNilInput exercises the nil-branch of cloneStrMap through
// GetFabricNode with a nil-Labels node, and verifies the non-nil path returns
// a properly cloned map (mutating the caller's copy does not corrupt the repo).
func TestCloneStrMapNilInput(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	// Create a node with nil Labels — exercises cloneStrMap(nil) → nil.
	if err := r.CreateFabricNode(ctx, FabricNode{
		NodeID: "node-nil-labels", Kind: "dev-mac", Arch: "arm64",
		LastSeenAt: ts, CreatedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricNode (nil labels): %v", err)
	}
	n, err := r.GetFabricNode(ctx, "node-nil-labels")
	if err != nil {
		t.Fatalf("GetFabricNode (nil labels): %v", err)
	}
	if n.Labels != nil {
		t.Fatalf("expected nil Labels, got %v", n.Labels)
	}

	// Non-nil path — CreateFabricNode also calls cloneStrMap on the input.
	if err := r.CreateFabricNode(ctx, FabricNode{
		NodeID: "node-with-labels", Kind: "ci-linux-kvm", Arch: "x86_64",
		Labels:     map[string]string{"pool": "runner-1"},
		LastSeenAt: ts, CreatedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricNode (with labels): %v", err)
	}
	n2, err := r.GetFabricNode(ctx, "node-with-labels")
	if err != nil {
		t.Fatalf("GetFabricNode (with labels): %v", err)
	}
	if n2.Labels["pool"] != "runner-1" {
		t.Fatalf("expected pool=runner-1, got %v", n2.Labels)
	}
	// Verify the returned map is a clone — mutating the copy must not affect the repo.
	n2.Labels["pool"] = "mutated"
	n3, _ := r.GetFabricNode(ctx, "node-with-labels")
	if n3.Labels["pool"] != "runner-1" {
		t.Fatalf("cloneStrMap returned a shared reference: %v", n3.Labels)
	}
}

// TestReleaseFabricLeaseEdgeCases covers the ErrNotFound path (unknown lease id),
// the idempotent re-release path (already-released lease), and the target-status
// cleanup path (releasing the only held lease restores "idle").
func TestReleaseFabricLeaseEdgeCases(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRepository()

	_ = r.CreateFabricTarget(ctx, FabricTarget{
		TargetID: "tgt", Tier: "T2", Tech: "cuttlefish",
		Exclusive: true, Status: "idle", CreatedAt: ts,
	})

	// 1. ReleaseFabricLease on an unknown lease id → ErrNotFound.
	if err := r.ReleaseFabricLease(ctx, "nonexistent-lease", ts); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Release unknown lease: want ErrNotFound, got %v", err)
	}

	if err := r.AcquireFabricLease(ctx, FabricLease{
		LeaseID: "l1", TargetID: "tgt", Owner: "A", AcquiredAt: ts,
	}); err != nil {
		t.Fatalf("AcquireFabricLease: %v", err)
	}

	// Release — returns nil on a held lease.
	if err := r.ReleaseFabricLease(ctx, "l1", ts.Add(5*time.Minute)); err != nil {
		t.Fatalf("ReleaseFabricLease first release: %v", err)
	}

	// 2. ReleaseFabricLease on an already-released lease → idempotent (no error).
	if err := r.ReleaseFabricLease(ctx, "l1", ts.Add(10*time.Minute)); err != nil {
		t.Fatalf("ReleaseFabricLease second release (idempotent): %v", err)
	}

	// 3. Verify the target status was restored to "idle" after releasing the only held lease.
	tgt, err := r.GetFabricTarget(ctx, "tgt")
	if err != nil {
		t.Fatalf("GetFabricTarget: %v", err)
	}
	if tgt.Status != "idle" {
		t.Fatalf("target status: want idle, got %q", tgt.Status)
	}
}

// TestReleaseFabricLeaseWithOtherActiveLease proves that when a second active lease
// still holds the same non-exclusive target, releasing the first does NOT change the
// target status back to "idle" (the stillHeld=true branch).
func TestReleaseFabricLeaseWithOtherActiveLease(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRepository()

	_ = r.CreateFabricTarget(ctx, FabricTarget{
		TargetID: "tgt-non-excl", Tier: "T0", Tech: "ota-device-emulator",
		Exclusive: false, Status: "idle", CreatedAt: ts,
	})

	if err := r.AcquireFabricLease(ctx, FabricLease{LeaseID: "l1", TargetID: "tgt-non-excl", Owner: "A", AcquiredAt: ts}); err != nil {
		t.Fatalf("first lease: %v", err)
	}
	// HB-2 reconcile (operator decision 2026-07-11): AcquireFabricLease now
	// enforces one active lease per target UNIFORMLY, so it no longer permits a
	// 2nd concurrent lease even on a non-exclusive target. Inject the 2nd active
	// lease DIRECTLY into the backing map (same pattern as
	// TestFabricLeaseExclusivityGuard's mutation half) to set up the "two active
	// leases" state whose stillHeld=true release branch this test exercises.
	r.mu.Lock()
	r.fabLeases["l2"] = FabricLease{LeaseID: "l2", TargetID: "tgt-non-excl", Owner: "B", AcquiredAt: ts}
	r.mu.Unlock()

	// Release l1 — l2 is still active, so the target stays "leased".
	_ = r.ReleaseFabricLease(ctx, "l1", ts.Add(5*time.Minute))

	tgt, _ := r.GetFabricTarget(ctx, "tgt-non-excl")
	if tgt.Status != "leased" {
		t.Fatalf("target status: want leased (other lease active), got %q", tgt.Status)
	}
}

// TestGetFabricRunNotFound covers the not-found branch of GetFabricRun.
func TestGetFabricRunNotFound(t *testing.T) {
	r := NewMemoryRepository()
	_, err := r.GetFabricRun(context.Background(), "nonexistent-run")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFabricRun unknown: want ErrNotFound, got %v", err)
	}
}

// TestActiveDeploymentForTargetGroupEdgeCases exercises the group-mismatch branch,
// the empty-group skip (matches any deployment regardless of its group), and the
// deployment-with-empty-group match (empty group always matches).
func TestActiveDeploymentForTargetGroupEdgeCases(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	_ = r.CreateRelease(ctx, Release{
		ReleaseID: "rel-1", Version: "1.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
		Status: "published",
	})

	// Active deployment scoped to "canary" group.
	_ = r.CreateDeployment(ctx, Deployment{
		DeploymentID: "dep-g", ReleaseID: "rel-1",
		Strategy: "rolling", Status: string(otaprotocol.DeploymentActive),
		Group: "canary",
	})

	// Query group mismatch — different group, no match.
	if _, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "stable"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("group mismatch: want ErrNotFound, got %v", err)
	}

	// Empty query group — matches even a group-scoped deployment (skips group check).
	dep, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "")
	if err != nil {
		t.Fatalf("empty group should match: %v", err)
	}
	if dep.DeploymentID != "dep-g" {
		t.Fatalf("expected dep-g, got %s", dep.DeploymentID)
	}

	// Matching group explicitly — should match.
	dep2, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "canary")
	if err != nil || dep2.DeploymentID != "dep-g" {
		t.Fatalf("matching group: %+v err=%v", dep2, err)
	}

	// Deployment with empty group always matches regardless of query group.
	_ = r.CreateRelease(ctx, Release{
		ReleaseID: "rel-2", Version: "2.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
		Status: "published",
	})
	_ = r.CreateDeployment(ctx, Deployment{
		DeploymentID: "dep-ng", ReleaseID: "rel-2",
		Strategy: "immediate", Status: string(otaprotocol.DeploymentActive),
	})
	dep3, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "any-group")
	if err != nil {
		t.Fatalf("empty group deployment should match any query group: %v", err)
	}
	if dep3.DeploymentID != "dep-ng" {
		t.Fatalf("expected dep-ng, got %s", dep3.DeploymentID)
	}
}

// TestRemoveProjectAccessThirdBranch covers the ErrNotFound path in
// RemoveProjectAccess when a caller exists in prjAccess but has no access
// entry for the specific project (the third ErrNotFound return).
func TestRemoveProjectAccessThirdBranch(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// Create a project and grant access so the caller has an entry in prjAccess.
	_ = r.CreateProject(ctx, Project{ProjectID: "proj-1", Name: "A"})
	_ = r.CreateProject(ctx, Project{ProjectID: "proj-2", Name: "B"})
	_ = r.SetProjectAccess(ctx, ProjectAccess{
		ProjectID: "proj-1", CallerID: "user-1", Role: ProjectRoleOperator,
	})

	// user-1 has access to proj-1 but NOT to proj-2.
	// Removing user-1 from proj-2 should return ErrNotFound because
	// user-1 exists in prjAccess but has no entry for proj-2.
	if err := r.RemoveProjectAccess(ctx, "user-1", "proj-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Remove caller from project they have no access to: want ErrNotFound, got %v", err)
	}
}

// TestListReleasesFilterByOSTypeAndModel covers the OSType and TargetModel filter
// branches in ListReleases.
func TestListReleasesFilterByOSTypeAndModel(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	_ = r.CreateRelease(ctx, Release{ReleaseID: "r1", Version: "1.0.0", OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published"})
	_ = r.CreateRelease(ctx, Release{ReleaseID: "r2", Version: "2.0.0", OSType: otaprotocol.OSAndroid, TargetModel: "RK3588", Status: "published"})
	_ = r.CreateRelease(ctx, Release{ReleaseID: "r3", Version: "1.0.0", OSType: otaprotocol.OSLinux, TargetModel: "OrangePi5Max", Status: "published"})

	// Filter by OSType.
	got, _, err := r.ListReleases(ctx, ReleaseFilter{OSType: otaprotocol.OSLinux})
	if err != nil || len(got) != 1 || got[0].ReleaseID != "r3" {
		t.Fatalf("OSType filter: want 1 (r3), got %d err=%v", len(got), err)
	}

	// Filter by TargetModel.
	got, _, err = r.ListReleases(ctx, ReleaseFilter{TargetModel: "RK3588"})
	if err != nil || len(got) != 1 || got[0].ReleaseID != "r2" {
		t.Fatalf("TargetModel filter: want 1 (r2), got %d err=%v", len(got), err)
	}
}

// TestActiveDeploymentForTargetMissingRelease covers the branch in
// ActiveDeploymentForTarget where a deployment references a release that does
// not exist in the releases map — the function must skip it rather than crash.
func TestActiveDeploymentForTargetMissingRelease(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// Create an active deployment that references a release that does NOT exist.
	_ = r.CreateDeployment(ctx, Deployment{
		DeploymentID: "dep-orphan", ReleaseID: "nonexistent-release",
		Strategy: "rolling", Status: string(otaprotocol.DeploymentActive),
	})

	// The orphan deployment must be skipped without error.
	if _, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("orphan deployment: want ErrNotFound, got %v", err)
	}
}

// TestListAuditOversizedCursor covers the cursor-past-end clamp in ListAudit.
func TestListAuditOversizedCursor(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	_ = r.AppendAudit(ctx, AuditEntry{ID: "a1", Action: "DEVICE_REGISTER"})

	// Cursor beyond the matched-set length should return an empty page.
	got, next, err := r.ListAudit(ctx, AuditFilter{Cursor: encodeCursor(99)})
	if err != nil {
		t.Fatalf("ListAudit oversized cursor: %v", err)
	}
	if len(got) != 0 || next != "" {
		t.Fatalf("ListAudit oversized cursor: want empty page, got %d next=%q", len(got), next)
	}
}
