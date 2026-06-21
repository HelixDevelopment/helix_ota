package store

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// TestAllDevicesSnapshot proves AllDevices returns every registered device and
// nothing more — the all-targets matching path in the api layer depends on it.
func TestAllDevicesSnapshot(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	if got := r.AllDevices(ctx); len(got) != 0 {
		t.Fatalf("empty repo: want 0 devices, got %d", len(got))
	}
	for _, id := range []string{"d-1", "d-2", "d-3"} {
		if err := r.CreateDevice(ctx, Device{DeviceID: id, HardwareID: "hw-" + id}); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	got := r.AllDevices(ctx)
	if len(got) != 3 {
		t.Fatalf("want 3 devices, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, d := range got {
		seen[d.DeviceID] = true
	}
	for _, id := range []string{"d-1", "d-2", "d-3"} {
		if !seen[id] {
			t.Fatalf("AllDevices missing %s: %+v", id, got)
		}
	}
}

// TestGetDeviceByHardwareIDMiss covers the not-found branch of the HW-index lookup.
func TestGetDeviceByHardwareIDMiss(t *testing.T) {
	r := NewMemoryRepository()
	if _, err := r.GetDeviceByHardwareID(context.Background(), "absent-hw"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for unknown hardware id, got %v", err)
	}
}

// TestGroupLifecycle exercises the group CRUD error/conflict/idempotency branches
// that the happy-path scenario tests do not reach.
func TestGroupLifecycle(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	if err := r.CreateGroup(ctx, Group{ID: "g1", Name: "prod"}); err != nil {
		t.Fatalf("create g1: %v", err)
	}
	// Same name, different id -> conflict.
	if err := r.CreateGroup(ctx, Group{ID: "g2", Name: "prod"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict on duplicate name, got %v", err)
	}
	// Re-creating the SAME id+name is an upsert, not a conflict.
	if err := r.CreateGroup(ctx, Group{ID: "g1", Name: "prod", Description: "updated"}); err != nil {
		t.Fatalf("upsert g1: %v", err)
	}

	// GetGroup miss.
	if _, err := r.GetGroup(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// UpdateGroup on unknown id -> not found.
	if err := r.UpdateGroup(ctx, Group{ID: "ghost", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update unknown: want ErrNotFound, got %v", err)
	}
	// Rename g1 -> "staging": both the rename branch and grpByName rekey run.
	if err := r.UpdateGroup(ctx, Group{ID: "g1", Name: "staging"}); err != nil {
		t.Fatalf("rename g1: %v", err)
	}
	// The old name must be free now; a new group may claim it.
	if err := r.CreateGroup(ctx, Group{ID: "g3", Name: "prod"}); err != nil {
		t.Fatalf("reuse freed name: %v", err)
	}
	// Renaming g1 onto a name another group holds -> conflict.
	if err := r.UpdateGroup(ctx, Group{ID: "g1", Name: "prod"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("rename collision: want ErrConflict, got %v", err)
	}

	// DeleteGroup unknown -> not found.
	if err := r.DeleteGroup(ctx, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete unknown: want ErrNotFound, got %v", err)
	}
	if err := r.DeleteGroup(ctx, "g1"); err != nil {
		t.Fatalf("delete g1: %v", err)
	}
	if _, err := r.GetGroup(ctx, "g1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("g1 still present after delete: %v", err)
	}
	// ListGroups should not include the deleted group.
	groups, _ := r.ListGroups(ctx)
	for _, g := range groups {
		if g.ID == "g1" {
			t.Fatalf("ListGroups still returns deleted g1: %+v", groups)
		}
	}
}

// TestGroupMembersErrorsAndIdempotency covers the membership add/list/remove
// not-found branches plus add/remove idempotency.
func TestGroupMembersErrorsAndIdempotency(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	now := time.Now()

	// Operations against a non-existent group all return ErrNotFound.
	if err := r.AddGroupMember(ctx, "absent", "d1", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("add to absent group: want ErrNotFound, got %v", err)
	}
	if _, err := r.ListGroupMembers(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("list absent group: want ErrNotFound, got %v", err)
	}
	if _, err := r.ListGroupMembersDetailed(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("list-detailed absent group: want ErrNotFound, got %v", err)
	}
	if err := r.RemoveGroupMember(ctx, "absent", "d1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove from absent group: want ErrNotFound, got %v", err)
	}

	if err := r.CreateGroup(ctx, Group{ID: "g1", Name: "prod"}); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := r.AddGroupMember(ctx, "g1", "d1", now); err != nil {
		t.Fatalf("add d1: %v", err)
	}
	// Adding the same device again is idempotent (no duplicate row).
	if err := r.AddGroupMember(ctx, "g1", "d1", now.Add(time.Hour)); err != nil {
		t.Fatalf("re-add d1: %v", err)
	}
	if err := r.AddGroupMember(ctx, "g1", "d2", now); err != nil {
		t.Fatalf("add d2: %v", err)
	}
	members, err := r.ListGroupMembers(ctx, "g1")
	if err != nil || len(members) != 2 {
		t.Fatalf("members want 2 (idempotent add), got %d err=%v", len(members), err)
	}
	detailed, err := r.ListGroupMembersDetailed(ctx, "g1")
	if err != nil || len(detailed) != 2 || detailed[0].DeviceID != "d1" {
		t.Fatalf("detailed members: %+v err=%v", detailed, err)
	}

	// Removing a device that is not a member is idempotent (nil error).
	if err := r.RemoveGroupMember(ctx, "g1", "never-joined"); err != nil {
		t.Fatalf("remove non-member: want nil, got %v", err)
	}
	if err := r.RemoveGroupMember(ctx, "g1", "d1"); err != nil {
		t.Fatalf("remove d1: %v", err)
	}
	after, _ := r.ListGroupMembers(ctx, "g1")
	if len(after) != 1 || after[0] != "d2" {
		t.Fatalf("after remove want [d2], got %v", after)
	}
}

// TestCreateDeltaConflictAndFind covers the duplicate-(base,target) ErrConflict
// path and the FindDelta miss/hit branches.
func TestCreateDeltaConflictAndFind(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	if _, err := r.FindDelta(ctx, "base", "target"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty FindDelta: want ErrNotFound, got %v", err)
	}
	d := DeltaArtifact{ID: "delta-1", BaseArtifactID: "base", TargetArtifactID: "target", SHA256: "abc"}
	if err := r.CreateDelta(ctx, d); err != nil {
		t.Fatalf("create delta: %v", err)
	}
	// Same (base,target) pair -> conflict even with a different id.
	if err := r.CreateDelta(ctx, DeltaArtifact{ID: "delta-2", BaseArtifactID: "base", TargetArtifactID: "target"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate delta: want ErrConflict, got %v", err)
	}
	// A different target is allowed.
	if err := r.CreateDelta(ctx, DeltaArtifact{ID: "delta-3", BaseArtifactID: "base", TargetArtifactID: "other"}); err != nil {
		t.Fatalf("distinct delta: %v", err)
	}
	got, err := r.FindDelta(ctx, "base", "target")
	if err != nil || got.ID != "delta-1" {
		t.Fatalf("FindDelta: %+v err=%v", got, err)
	}
}

// TestRollbackAppendAndList covers append-only rollback storage + per-deployment
// filtering (records for other deployments must not leak in).
func TestRollbackAppendAndList(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	if recs, err := r.ListRollbacks(ctx, "dep-1"); err != nil || len(recs) != 0 {
		t.Fatalf("empty rollbacks: %d err=%v", len(recs), err)
	}
	_ = r.AppendRollback(ctx, RollbackRecord{ID: "rb-1", DeploymentID: "dep-1", Kind: "abort"})
	_ = r.AppendRollback(ctx, RollbackRecord{ID: "rb-2", DeploymentID: "dep-2", Kind: "rollback"})
	_ = r.AppendRollback(ctx, RollbackRecord{ID: "rb-3", DeploymentID: "dep-1", Kind: "rollback"})

	recs, err := r.ListRollbacks(ctx, "dep-1")
	if err != nil || len(recs) != 2 {
		t.Fatalf("dep-1 rollbacks want 2, got %d err=%v", len(recs), err)
	}
	// Append-only insertion order preserved.
	if recs[0].ID != "rb-1" || recs[1].ID != "rb-3" {
		t.Fatalf("rollback order wrong: %+v", recs)
	}
}

// TestTelemetryForDeviceAndStateCounts covers per-device telemetry filtering and
// the DeviceStateCounts unknown-bucketing branch.
func TestTelemetryForDeviceAndStateCounts(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	_ = r.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "d1", Event: otaprotocol.EventSuccess})
	_ = r.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "d2", Event: otaprotocol.EventFailure})
	_ = r.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "d1", Event: otaprotocol.EventFailure})

	d1, err := r.TelemetryForDevice(ctx, "d1")
	if err != nil || len(d1) != 2 {
		t.Fatalf("d1 telemetry want 2, got %d err=%v", len(d1), err)
	}
	if none, _ := r.TelemetryForDevice(ctx, "absent"); len(none) != 0 {
		t.Fatalf("absent device telemetry want 0, got %d", len(none))
	}

	counts, err := r.TelemetryEventCounts(ctx)
	if err != nil || counts[string(otaprotocol.EventSuccess)] != 1 || counts[string(otaprotocol.EventFailure)] != 2 {
		t.Fatalf("event counts wrong: %+v err=%v", counts, err)
	}

	// One device with an explicit state, one with none -> "unknown" bucket.
	_ = r.CreateDevice(ctx, Device{DeviceID: "d1", HardwareID: "hw1", UpdateState: "updated"})
	_ = r.CreateDevice(ctx, Device{DeviceID: "d2", HardwareID: "hw2"}) // empty UpdateState
	states, err := r.DeviceStateCounts(ctx)
	if err != nil || states["updated"] != 1 || states["unknown"] != 1 {
		t.Fatalf("state counts wrong: %+v err=%v", states, err)
	}
}

// TestActiveDeploymentForTargetGroupNarrowing covers the group-narrowing branch
// and the no-match (ErrNotFound) tail of ActiveDeploymentForTarget.
func TestActiveDeploymentForTargetGroupNarrowing(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// No release/deployment yet -> not found.
	if _, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no deployment: want ErrNotFound, got %v", err)
	}

	_ = r.CreateRelease(ctx, Release{ReleaseID: "rel-1", Version: "1.0.0", OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max"})
	// A group-scoped active deployment.
	_ = r.CreateDeployment(ctx, Deployment{
		DeploymentID: "dep-g", ReleaseID: "rel-1",
		Status: string(otaprotocol.DeploymentActive), Group: "canary",
	})
	// A non-active deployment that must be skipped.
	_ = r.CreateDeployment(ctx, Deployment{
		DeploymentID: "dep-done", ReleaseID: "rel-1",
		Status: string(otaprotocol.DeploymentCompleted),
	})

	// Matching group resolves the group-scoped deployment.
	got, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "canary")
	if err != nil || got.DeploymentID != "dep-g" {
		t.Fatalf("group match: %+v err=%v", got, err)
	}
	// A different group must NOT match the canary-scoped deployment.
	if _, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "prod"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong group: want ErrNotFound, got %v", err)
	}
	// Wrong os/model must not match either.
	if _, err := r.ActiveDeploymentForTarget(ctx, otaprotocol.OSAndroid, "OtherModel", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong model: want ErrNotFound, got %v", err)
	}
}

// TestReleaseByVersionAndGetMiss covers ReleaseByVersion hit/miss and GetRelease
// not-found.
func TestReleaseByVersionAndGetMiss(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	if _, err := r.GetRelease(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRelease miss: want ErrNotFound, got %v", err)
	}
	if _, err := r.ReleaseByVersion(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "9.9.9"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReleaseByVersion miss: want ErrNotFound, got %v", err)
	}
	_ = r.CreateRelease(ctx, Release{ReleaseID: "rel-1", Version: "1.4.0", OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max"})
	got, err := r.ReleaseByVersion(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "1.4.0")
	if err != nil || got.ReleaseID != "rel-1" {
		t.Fatalf("ReleaseByVersion hit: %+v err=%v", got, err)
	}
}

// TestGetDeploymentMiss covers the not-found branch of GetDeployment.
func TestGetDeploymentMiss(t *testing.T) {
	r := NewMemoryRepository()
	if _, err := r.GetDeployment(context.Background(), "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestUpdateDeploymentMiss covers the not-found branch of UpdateDeployment.
func TestUpdateDeploymentMiss(t *testing.T) {
	r := NewMemoryRepository()
	if err := r.UpdateDeployment(context.Background(), Deployment{DeploymentID: "absent"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestListAuditFilteringAndPaging covers the action/resource/time filters and
// the offset-cursor paging of ListAudit (84% -> full filter coverage).
func TestListAuditFilteringAndPaging(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	base := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	entries := []AuditEntry{
		{ID: "a1", Action: "DEVICE_REGISTER", ResourceType: "device", CreatedAt: base},
		{ID: "a2", Action: "RELEASE_PUBLISH", ResourceType: "release", CreatedAt: base.Add(1 * time.Hour)},
		{ID: "a3", Action: "DEVICE_REGISTER", ResourceType: "device", CreatedAt: base.Add(2 * time.Hour)},
		{ID: "a4", Action: "DEVICE_REGISTER", ResourceType: "device", CreatedAt: base.Add(3 * time.Hour)},
	}
	for _, e := range entries {
		if err := r.AppendAudit(ctx, e); err != nil {
			t.Fatalf("append %s: %v", e.ID, err)
		}
	}

	// Action filter.
	got, _, err := r.ListAudit(ctx, AuditFilter{Action: "DEVICE_REGISTER"})
	if err != nil || len(got) != 3 {
		t.Fatalf("action filter want 3, got %d err=%v", len(got), err)
	}
	// ResourceType filter.
	got, _, _ = r.ListAudit(ctx, AuditFilter{ResourceType: "release"})
	if len(got) != 1 || got[0].ID != "a2" {
		t.Fatalf("resource filter: %+v", got)
	}
	// Since filter (strictly excludes earlier than base+2h).
	got, _, _ = r.ListAudit(ctx, AuditFilter{Since: base.Add(2 * time.Hour)})
	if len(got) != 2 {
		t.Fatalf("since filter want 2, got %d", len(got))
	}
	// Until filter (excludes after base+1h).
	got, _, _ = r.ListAudit(ctx, AuditFilter{Until: base.Add(1 * time.Hour)})
	if len(got) != 2 {
		t.Fatalf("until filter want 2, got %d", len(got))
	}

	// Paging: limit 2 over the 3 DEVICE_REGISTER rows -> page1=2 + cursor, page2=1.
	page1, next, _ := r.ListAudit(ctx, AuditFilter{Action: "DEVICE_REGISTER", Limit: 2})
	if len(page1) != 2 || next == "" {
		t.Fatalf("audit page1 want 2 + cursor, got %d next=%q", len(page1), next)
	}
	page2, next2, _ := r.ListAudit(ctx, AuditFilter{Action: "DEVICE_REGISTER", Limit: 2, Cursor: next})
	if len(page2) != 1 || next2 != "" {
		t.Fatalf("audit page2 want 1 + empty cursor, got %d next=%q", len(page2), next2)
	}
}

// TestListReleasesStatusFilterAndOversizedCursor covers the status filter branch
// and the out-of-range cursor clamp of ListReleases.
func TestListReleasesStatusFilterAndOversizedCursor(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()
	_ = r.CreateRelease(ctx, Release{ReleaseID: "r1", Version: "1.0.0", OSType: otaprotocol.OSAndroid, TargetModel: "M", Status: "published"})
	_ = r.CreateRelease(ctx, Release{ReleaseID: "r2", Version: "1.1.0", OSType: otaprotocol.OSAndroid, TargetModel: "M", Status: "draft"})

	got, _, err := r.ListReleases(ctx, ReleaseFilter{Status: "published"})
	if err != nil || len(got) != 1 || got[0].ReleaseID != "r1" {
		t.Fatalf("status filter: %+v err=%v", got, err)
	}
	// A cursor beyond the matched length clamps to an empty final page.
	beyond := encodeCursor(99)
	got, next, _ := r.ListReleases(ctx, ReleaseFilter{Cursor: beyond})
	if len(got) != 0 || next != "" {
		t.Fatalf("oversized cursor: want empty page, got %d next=%q", len(got), next)
	}
}

// TestListDevicesFiltersAndPagination covers ListDevices filtering by OSType,
// TargetModel, Status, default limit, cursor pagination, and oversized-cursor clamp.
func TestListDevicesFiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// Empty repo.
	devs, next, err := r.ListDevices(ctx, DeviceFilter{})
	if err != nil || len(devs) != 0 || next != "" {
		t.Fatalf("empty repo: want 0 + no cursor, got %d next=%q err=%v", len(devs), next, err)
	}

	for _, d := range []Device{
		{DeviceID: "d1", HardwareID: "hw1", Model: "M1", OSType: otaprotocol.OSAndroid, UpdateState: "active"},
		{DeviceID: "d2", HardwareID: "hw2", Model: "M1", OSType: otaprotocol.OSAndroid, UpdateState: "inactive"},
		{DeviceID: "d3", HardwareID: "hw3", Model: "M2", OSType: otaprotocol.OSAndroid, UpdateState: "active"},
		{DeviceID: "d4", HardwareID: "hw4", Model: "M1", OSType: otaprotocol.OSLinux, UpdateState: "active"},
		{DeviceID: "d5", HardwareID: "hw5", Model: "M1", OSType: otaprotocol.OSAndroid, UpdateState: "active"},
	} {
		if err := r.CreateDevice(ctx, d); err != nil {
			t.Fatalf("create %s: %v", d.DeviceID, err)
		}
	}

	// Filter by OSType.
	got, _, err := r.ListDevices(ctx, DeviceFilter{OSType: otaprotocol.OSLinux})
	if err != nil || len(got) != 1 || got[0].DeviceID != "d4" {
		t.Fatalf("os filter: want 1 (d4), got %d err=%v", len(got), err)
	}

	// Filter by TargetModel.
	got, _, _ = r.ListDevices(ctx, DeviceFilter{TargetModel: "M2"})
	if len(got) != 1 || got[0].DeviceID != "d3" {
		t.Fatalf("model filter: want 1 (d3), got %d", len(got))
	}

	// Filter by Status.
	got, _, _ = r.ListDevices(ctx, DeviceFilter{Status: "inactive"})
	if len(got) != 1 || got[0].DeviceID != "d2" {
		t.Fatalf("status filter: want 1 (d2), got %d", len(got))
	}

	// Combined filter.
	got, _, _ = r.ListDevices(ctx, DeviceFilter{OSType: otaprotocol.OSAndroid, Status: "active"})
	if len(got) != 3 {
		t.Fatalf("combined filter: want 3, got %d", len(got))
	}

	// Pagination: limit=2, 3 matched -> page1=2 + cursor, page2=1 + empty cursor.
	page1, next, err := r.ListDevices(ctx, DeviceFilter{OSType: otaprotocol.OSAndroid, Status: "active", Limit: 2})
	if err != nil || len(page1) != 2 || next == "" {
		t.Fatalf("page1: want 2 + cursor, got %d next=%q err=%v", len(page1), next, err)
	}
	page2, next2, _ := r.ListDevices(ctx, DeviceFilter{OSType: otaprotocol.OSAndroid, Status: "active", Limit: 2, Cursor: next})
	if len(page2) != 1 || next2 != "" {
		t.Fatalf("page2: want 1 + empty cursor, got %d next=%q", len(page2), next2)
	}

	// Oversized cursor clamps to empty page.
	beyond := encodeCursor(999)
	got, next, err = r.ListDevices(ctx, DeviceFilter{Cursor: beyond})
	if len(got) != 0 || next != "" || err != nil {
		t.Fatalf("oversized cursor: want 0 + no cursor, got %d next=%q err=%v", len(got), next, err)
	}
}

// TestProjectLifecycle covers CreateProject, GetProject, ListProjects,
// UpdateProject, and DeleteProject hit/miss/conflict branches.
func TestProjectLifecycle(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// ListProjects on empty repo.
	projects, err := r.ListProjects(ctx)
	if err != nil || len(projects) != 0 {
		t.Fatalf("empty list: want 0, got %d err=%v", len(projects), err)
	}

	// GetProject miss.
	if _, err := r.GetProject(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProject miss: want ErrNotFound, got %v", err)
	}

	// CreateProject.
	if err := r.CreateProject(ctx, Project{ProjectID: "p1", Name: "alpha", Description: "first"}); err != nil {
		t.Fatalf("create p1: %v", err)
	}
	// Same name, different id -> conflict.
	if err := r.CreateProject(ctx, Project{ProjectID: "p2", Name: "alpha"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name: want ErrConflict, got %v", err)
	}
	// Re-creating the same id+name is an upsert (no conflict).
	if err := r.CreateProject(ctx, Project{ProjectID: "p1", Name: "alpha", Description: "updated"}); err != nil {
		t.Fatalf("upsert p1: %v", err)
	}

	// Create a second project.
	if err := r.CreateProject(ctx, Project{ProjectID: "p2", Name: "beta"}); err != nil {
		t.Fatalf("create p2: %v", err)
	}

	// GetProject hit.
	got, err := r.GetProject(ctx, "p1")
	if err != nil || got.ProjectID != "p1" || got.Name != "alpha" {
		t.Fatalf("GetProject hit: %+v err=%v", got, err)
	}

	// ListProjects returns in insertion order.
	projects, _ = r.ListProjects(ctx)
	if len(projects) != 2 || projects[0].ProjectID != "p1" || projects[1].ProjectID != "p2" {
		t.Fatalf("list order: want [p1, p2], got %+v", projects)
	}

	// UpdateProject miss.
	if err := r.UpdateProject(ctx, Project{ProjectID: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update unknown: want ErrNotFound, got %v", err)
	}

	// Rename p1 -> "gamma": rekey in prjByName.
	if err := r.UpdateProject(ctx, Project{ProjectID: "p1", Name: "gamma"}); err != nil {
		t.Fatalf("rename p1: %v", err)
	}
	// The old name "alpha" is now free; a new project may claim it.
	if err := r.CreateProject(ctx, Project{ProjectID: "p3", Name: "alpha"}); err != nil {
		t.Fatalf("reuse freed name: %v", err)
	}

	// Rename p1 onto a name another project holds -> conflict.
	if err := r.UpdateProject(ctx, Project{ProjectID: "p1", Name: "beta"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("rename collision: want ErrConflict, got %v", err)
	}

	// UpdateProject success (no rename).
	p1, _ := r.GetProject(ctx, "p1")
	p1.Description = "desc"
	if err := r.UpdateProject(ctx, p1); err != nil {
		t.Fatalf("update p1 desc: %v", err)
	}

	// DeleteProject miss.
	if err := r.DeleteProject(ctx, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete unknown: want ErrNotFound, got %v", err)
	}

	// DeleteProject success.
	if err := r.DeleteProject(ctx, "p1"); err != nil {
		t.Fatalf("delete p1: %v", err)
	}
	if _, err := r.GetProject(ctx, "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("p1 still present after delete: %v", err)
	}
	// ListProjects should not include the deleted project.
	projects, _ = r.ListProjects(ctx)
	for _, p := range projects {
		if p.ProjectID == "p1" {
			t.Fatalf("ListProjects still returns deleted p1: %+v", projects)
		}
	}
}

// TestProjectAccessManagement covers GetProjectAccess, SetProjectAccess,
// ListProjectMembers, and RemoveProjectAccess hit/miss branches.
func TestProjectAccessManagement(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	// Operations against a non-existent project all return ErrNotFound.
	if _, err := r.GetProjectAccess(ctx, "caller1", "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectAccess absent project: want ErrNotFound, got %v", err)
	}
	if err := r.SetProjectAccess(ctx, ProjectAccess{ProjectID: "absent", CallerID: "caller1", Role: ProjectRoleViewer}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetProjectAccess absent project: want ErrNotFound, got %v", err)
	}
	if _, err := r.ListProjectMembers(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListProjectMembers absent project: want ErrNotFound, got %v", err)
	}
	if err := r.RemoveProjectAccess(ctx, "caller1", "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveProjectAccess absent project: want ErrNotFound, got %v", err)
	}

	// Create a project.
	if err := r.CreateProject(ctx, Project{ProjectID: "p1", Name: "test-proj"}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// GetProjectAccess: caller has no access.
	if _, err := r.GetProjectAccess(ctx, "caller1", "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectAccess no access: want ErrNotFound, got %v", err)
	}

	// SetProjectAccess: grant viewer access.
	if err := r.SetProjectAccess(ctx, ProjectAccess{ProjectID: "p1", CallerID: "caller1", Role: ProjectRoleViewer}); err != nil {
		t.Fatalf("grant viewer: %v", err)
	}
	// SetProjectAccess: grant operator access (update existing).
	if err := r.SetProjectAccess(ctx, ProjectAccess{ProjectID: "p1", CallerID: "caller1", Role: ProjectRoleOperator}); err != nil {
		t.Fatalf("upgrade to operator: %v", err)
	}
	// Grant admin to a second caller.
	if err := r.SetProjectAccess(ctx, ProjectAccess{ProjectID: "p1", CallerID: "caller2", Role: ProjectRoleAdmin}); err != nil {
		t.Fatalf("grant admin: %v", err)
	}

	// GetProjectAccess hit.
	access, err := r.GetProjectAccess(ctx, "caller1", "p1")
	if err != nil || access.CallerID != "caller1" || access.Role != ProjectRoleOperator {
		t.Fatalf("GetProjectAccess hit: %+v err=%v", access, err)
	}

	// ListProjectMembers.
	members, err := r.ListProjectMembers(ctx, "p1")
	if err != nil || len(members) != 2 {
		t.Fatalf("members want 2, got %d err=%v", len(members), err)
	}

	// RemoveProjectAccess: caller has no access entry (entirely unknown caller).
	if err := r.RemoveProjectAccess(ctx, "stranger", "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveProjectAccess unknown caller: want ErrNotFound, got %v", err)
	}
	// RemoveProjectAccess: caller known but lacks access to this project.
	if err := r.SetProjectAccess(ctx, ProjectAccess{ProjectID: "p1", CallerID: "caller3", Role: ProjectRoleViewer}); err != nil {
		t.Fatalf("grant viewer to caller3: %v", err)
	}
	if err := r.RemoveProjectAccess(ctx, "caller3", "p1"); err != nil {
		t.Fatalf("RemoveProjectAccess caller3: %v", err)
	}
	// After removal, caller3 should no longer have access.
	if _, err := r.GetProjectAccess(ctx, "caller3", "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("caller3 still has access after removal: %v", err)
	}
}

// TestDecodeCursorMalformedInputs covers the malformed/negative decode branches
// that fall back to offset 0.
func TestDecodeCursorMalformedInputs(t *testing.T) {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	cases := map[string]string{
		"empty":          "",
		"not-base64":     "!!!not base64!!!",
		"base64-non-int": b64("abc"),
		"negative":       b64("-5"),
	}
	for name, in := range cases {
		if got := decodeCursor(in); got != 0 {
			t.Fatalf("%s: want offset 0, got %d", name, got)
		}
	}
	// A valid positive cursor round-trips.
	if got := decodeCursor(encodeCursor(7)); got != 7 {
		t.Fatalf("valid cursor round-trip: want 7, got %d", got)
	}
}
