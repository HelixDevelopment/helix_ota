//go:build integration

// Coverage-push integration test (§11.4.69 real captured evidence, §11.4.74 reuse
// the containers brick): exercises the pgx production paths the shared contract
// (contract_test.go) does NOT drive — the project / project-access CRUD (0.0%
// before this run), ListDevices filter+paging (0.0%), NewPostgresRepositoryFromPool
// (0.0%), and the not-found / conflict error branches of the project methods.
// Every assertion runs against the REAL PostgreSQL booted on-demand through the
// digital.vasic.containers submodule (port 55432, same as the contract test,
// serialized via lockPgIntegration per §11.4.119). Run:
//
//	go test -tags integration ./internal/store/
package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/jackc/pgx/v5/pgxpool"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

// TestPostgresCoveragePaths_Integration boots a real Postgres and drives the
// genuinely-reachable pgx paths left uncovered by the contract scenario.
func TestPostgresCoveragePaths_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 serialize shared Postgres across integration packages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}

	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Fatalf("no container runtime (podman/docker) available: %v", err)
	}
	t.Logf("container runtime: %s", rt.Name())

	orch, err := compose.NewDefaultOrchestrator(projectDir, logging.NopLogger{})
	if err != nil {
		t.Fatalf("compose orchestrator (need podman/docker compose): %v", err)
	}

	ep := endpoint.NewEndpoint().
		WithHost("localhost").WithPort(pgHostPort).
		WithHealthType("tcp").WithRequired(true).WithEnabled(true).
		WithComposeFile("postgres.compose.yml").WithServiceName("postgres").
		WithTimeout(120 * time.Second).WithRetryCount(60).
		Build()

	mgr := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{"postgres": ep},
		boot.WithRuntime(rt),
		boot.WithOrchestrator(orch),
		boot.WithHealthChecker(health.NewDefaultChecker()),
		boot.WithProjectDir(projectDir),
		boot.WithLogger(logging.NopLogger{}),
	)

	summary, err := mgr.BootAll(ctx)
	if err != nil {
		t.Fatalf("BootAll (required postgres) failed: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })
	if summary.Failed > 0 {
		t.Fatalf("boot summary reports %d failed service(s)", summary.Failed)
	}
	t.Logf("boot summary: started=%d discovered=%d failed=%d in %s",
		summary.Started, summary.Discovered, summary.Failed, summary.TotalDuration)

	// Build the repo via the FromPool constructor (the 0.0% path) so it is
	// genuinely exercised, then run migration on a clean schema.
	pool := mustPool(t, ctx)
	t.Cleanup(pool.Close)
	repo := NewPostgresRepositoryFromPool(pool)

	resetSchema(t, ctx)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	runProjectAccessContract(t, ctx, repo)
	runListDevicesContract(t, ctx, repo)
	runUpdateDeviceConflictContract(t, ctx, repo)
}

// runUpdateDeviceConflictContract proves UpdateDevice maps a hardware_id
// UNIQUE-constraint violation (devices_hardware_id_uniq, schema_postgres.sql)
// to ErrConflict — the same mapping UpdateGroup/UpdateProject already apply to
// their own UNIQUE(name) rename conflicts. Before this fix, UpdateDevice's SQL
// UPDATE had no isUniqueViolation check at all, so this exact scenario returned
// the raw *pgconn.PgError past the Repository interface's documented error
// contract instead of the sentinel callers branch on with errors.Is.
func runUpdateDeviceConflictContract(t *testing.T, ctx context.Context, repo *PostgresRepository) {
	t.Helper()
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	if err := repo.CreateDevice(ctx, Device{DeviceID: "uc-dev-1", HardwareID: "UC-HW-1", RegisteredAt: ts}); err != nil {
		t.Fatalf("CreateDevice uc-dev-1: %v", err)
	}
	if err := repo.CreateDevice(ctx, Device{DeviceID: "uc-dev-2", HardwareID: "UC-HW-2", RegisteredAt: ts}); err != nil {
		t.Fatalf("CreateDevice uc-dev-2: %v", err)
	}

	// uc-dev-2 tries to steal uc-dev-1's hardware id via UpdateDevice.
	steal := Device{DeviceID: "uc-dev-2", HardwareID: "UC-HW-1", RegisteredAt: ts}
	if err := repo.UpdateDevice(ctx, steal); !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateDevice hardware_id collision want ErrConflict, got %v", err)
	}
	// uc-dev-2's row must be unchanged by the rejected update.
	if got, err := repo.GetDevice(ctx, "uc-dev-2"); err != nil || got.HardwareID != "UC-HW-2" {
		t.Fatalf("uc-dev-2 corrupted by rejected UpdateDevice: %+v err=%v", got, err)
	}
	// uc-dev-1's binding must be untouched too.
	if got, err := repo.GetDeviceByHardwareID(ctx, "UC-HW-1"); err != nil || got.DeviceID != "uc-dev-1" {
		t.Fatalf("UC-HW-1 binding corrupted by rejected UpdateDevice: %+v err=%v", got, err)
	}

	// A genuine (non-colliding) hardware_id change still succeeds and re-indexes.
	rename := Device{DeviceID: "uc-dev-2", HardwareID: "UC-HW-2-RENAMED", RegisteredAt: ts}
	if err := repo.UpdateDevice(ctx, rename); err != nil {
		t.Fatalf("UpdateDevice genuine hardware_id change: %v", err)
	}
	if got, err := repo.GetDeviceByHardwareID(ctx, "UC-HW-2-RENAMED"); err != nil || got.DeviceID != "uc-dev-2" {
		t.Fatalf("GetDeviceByHardwareID(UC-HW-2-RENAMED): %+v err=%v", got, err)
	}
	if _, err := repo.GetDeviceByHardwareID(ctx, "UC-HW-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDeviceByHardwareID(UC-HW-2) should be gone after rename, got %v", err)
	}
}

// mustPool opens a pool against the freshly-booted Postgres, retrying until it
// accepts queries (TCP-open != query-ready).
func mustPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	var lastErr error
	for i := 0; i < 60; i++ {
		pool, err := pgxpool.New(ctx, pgDSN)
		if err == nil {
			if perr := pool.Ping(ctx); perr == nil {
				return pool
			} else {
				lastErr = perr
				pool.Close()
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for postgres: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("postgres never became query-ready: %v", lastErr)
	return nil
}

// runProjectAccessContract exercises the project + project-access CRUD against
// the real DB: every method, plus the not-found and unique-conflict branches.
func runProjectAccessContract(t *testing.T, ctx context.Context, repo *PostgresRepository) {
	t.Helper()
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	// --- create + get round-trip ---
	p1 := Project{ProjectID: "prj-1", Name: "fleet-prod", Description: "production OTA",
		CreatedAt: ts, UpdatedAt: ts}
	if err := repo.CreateProject(ctx, p1); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	got, err := repo.GetProject(ctx, "prj-1")
	if err != nil || got.Name != "fleet-prod" || got.Description != "production OTA" {
		t.Fatalf("GetProject round-trip mismatch: %+v err=%v", got, err)
	}
	if _, err := repo.GetProject(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProject unknown want ErrNotFound, got %v", err)
	}

	// --- unique-name conflict (schema: projects.name UNIQUE -> 23505 -> ErrConflict) ---
	if err := repo.CreateProject(ctx, Project{ProjectID: "prj-2", Name: "fleet-prod",
		CreatedAt: ts, UpdatedAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate project name want ErrConflict, got %v", err)
	}

	// --- second project so List has >1 row ordered by created_at ---
	p2 := Project{ProjectID: "prj-3", Name: "fleet-staging", CreatedAt: ts.Add(time.Hour),
		UpdatedAt: ts.Add(time.Hour)}
	if err := repo.CreateProject(ctx, p2); err != nil {
		t.Fatalf("CreateProject p2: %v", err)
	}
	list, err := repo.ListProjects(ctx)
	if err != nil || len(list) != 2 || list[0].ProjectID != "prj-1" || list[1].ProjectID != "prj-3" {
		t.Fatalf("ListProjects: %+v err=%v", list, err)
	}

	// --- update (found) + update (not found) ---
	p1.Description = "prod (updated)"
	p1.UpdatedAt = ts.Add(2 * time.Hour)
	if err := repo.UpdateProject(ctx, p1); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if got, _ := repo.GetProject(ctx, "prj-1"); got.Description != "prod (updated)" {
		t.Fatalf("UpdateProject not applied: %+v", got)
	}
	if err := repo.UpdateProject(ctx, Project{ProjectID: "ghost", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateProject unknown want ErrNotFound, got %v", err)
	}
	// Rename prj-3 onto prj-1's name -> unique conflict on UPDATE.
	if err := repo.UpdateProject(ctx, Project{ProjectID: "prj-3", Name: "fleet-prod",
		UpdatedAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateProject duplicate name want ErrConflict, got %v", err)
	}

	// --- project access: set (insert), set (upsert), get, list, remove ---
	if err := repo.SetProjectAccess(ctx, ProjectAccess{ProjectID: "prj-1",
		CallerID: "alice@helix.test", Role: ProjectRoleAdmin}); err != nil {
		t.Fatalf("SetProjectAccess insert: %v", err)
	}
	// Upsert path: same (caller,project) updates the role.
	if err := repo.SetProjectAccess(ctx, ProjectAccess{ProjectID: "prj-1",
		CallerID: "alice@helix.test", Role: ProjectRoleOperator}); err != nil {
		t.Fatalf("SetProjectAccess upsert: %v", err)
	}
	if err := repo.SetProjectAccess(ctx, ProjectAccess{ProjectID: "prj-1",
		CallerID: "bob@helix.test", Role: ProjectRoleViewer}); err != nil {
		t.Fatalf("SetProjectAccess bob: %v", err)
	}
	acc, err := repo.GetProjectAccess(ctx, "alice@helix.test", "prj-1")
	if err != nil || acc.Role != ProjectRoleOperator {
		t.Fatalf("GetProjectAccess after upsert want operator: %+v err=%v", acc, err)
	}
	if _, err := repo.GetProjectAccess(ctx, "nobody@helix.test", "prj-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectAccess unknown want ErrNotFound, got %v", err)
	}
	members, err := repo.ListProjectMembers(ctx, "prj-1")
	if err != nil || len(members) != 2 || members[0].CallerID != "alice@helix.test" || members[1].CallerID != "bob@helix.test" {
		t.Fatalf("ListProjectMembers: %+v err=%v", members, err)
	}
	if err := repo.RemoveProjectAccess(ctx, "bob@helix.test", "prj-1"); err != nil {
		t.Fatalf("RemoveProjectAccess: %v", err)
	}
	if err := repo.RemoveProjectAccess(ctx, "bob@helix.test", "prj-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveProjectAccess already-gone want ErrNotFound, got %v", err)
	}
	if members, _ := repo.ListProjectMembers(ctx, "prj-1"); len(members) != 1 {
		t.Fatalf("after remove want 1 member, got %+v", members)
	}

	// --- delete (found) cascades access (FK ON DELETE CASCADE), then delete (not found) ---
	if err := repo.DeleteProject(ctx, "prj-1"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := repo.GetProject(ctx, "prj-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("project should be gone, got %v", err)
	}
	// FK cascade: alice's access row was removed with the project.
	if _, err := repo.GetProjectAccess(ctx, "alice@helix.test", "prj-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("access should cascade-delete with project, got %v", err)
	}
	if err := repo.DeleteProject(ctx, "prj-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteProject already-gone want ErrNotFound, got %v", err)
	}

	// Clean up prj-3 so the device section starts uncluttered (projects don't
	// affect devices, but keep the schema tidy for determinism).
	if err := repo.DeleteProject(ctx, "prj-3"); err != nil {
		t.Fatalf("DeleteProject prj-3 cleanup: %v", err)
	}
}

// runListDevicesContract exercises ListDevices: unfiltered, filtered by os_type /
// model / status, and offset-cursor paging — the 0.0% pgx path.
func runListDevicesContract(t *testing.T, ctx context.Context, repo *PostgresRepository) {
	t.Helper()
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	seed := []Device{
		{DeviceID: "d-a", HardwareID: "HW-a", Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid,
			UpdateState: "idle", RegisteredAt: ts},
		{DeviceID: "d-b", HardwareID: "HW-b", Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid,
			UpdateState: "downloading", RegisteredAt: ts},
		{DeviceID: "d-c", HardwareID: "HW-c", Model: "RaspberryPi5", OSType: otaprotocol.OSLinux,
			UpdateState: "idle", RegisteredAt: ts},
	}
	for _, d := range seed {
		if err := repo.CreateDevice(ctx, d); err != nil {
			t.Fatalf("seed CreateDevice %s: %v", d.DeviceID, err)
		}
	}

	// Unfiltered: all three, in insertion order (seq), no next cursor. The seed
	// is inserted d-a,d-b,d-c so insertion order coincides with device_id order
	// here; the non-lexicographic STORE-1 proof lives in runRepositoryContract.
	all, next, err := repo.ListDevices(ctx, DeviceFilter{})
	if err != nil || len(all) != 3 || all[0].DeviceID != "d-a" || all[2].DeviceID != "d-c" || next != "" {
		t.Fatalf("ListDevices all: n=%d next=%q err=%v", len(all), next, err)
	}

	// Filter by os_type.
	android, _, err := repo.ListDevices(ctx, DeviceFilter{OSType: otaprotocol.OSAndroid})
	if err != nil || len(android) != 2 {
		t.Fatalf("ListDevices os_type=android want 2, got %d err=%v", len(android), err)
	}

	// Filter by model.
	rpi, _, err := repo.ListDevices(ctx, DeviceFilter{TargetModel: "RaspberryPi5"})
	if err != nil || len(rpi) != 1 || rpi[0].DeviceID != "d-c" {
		t.Fatalf("ListDevices model=RaspberryPi5: %+v err=%v", rpi, err)
	}

	// Filter by status (update_state).
	idle, _, err := repo.ListDevices(ctx, DeviceFilter{Status: "idle"})
	if err != nil || len(idle) != 2 {
		t.Fatalf("ListDevices status=idle want 2, got %d err=%v", len(idle), err)
	}

	// Offset-cursor paging: limit 2 -> page1 has 2 + a next cursor; page2 has 1, no cursor.
	page1, n1, err := repo.ListDevices(ctx, DeviceFilter{Limit: 2})
	if err != nil || len(page1) != 2 || page1[0].DeviceID != "d-a" || page1[1].DeviceID != "d-b" || n1 == "" {
		t.Fatalf("ListDevices page1: %+v next=%q err=%v", deviceIDs(page1), n1, err)
	}
	page2, n2, err := repo.ListDevices(ctx, DeviceFilter{Limit: 2, Cursor: n1})
	if err != nil || len(page2) != 1 || page2[0].DeviceID != "d-c" || n2 != "" {
		t.Fatalf("ListDevices page2: %+v next=%q err=%v", deviceIDs(page2), n2, err)
	}

	// A filter that matches nothing returns an empty slice (not an error).
	none, _, err := repo.ListDevices(ctx, DeviceFilter{TargetModel: "NoSuchBoard"})
	if err != nil || len(none) != 0 {
		t.Fatalf("ListDevices no-match want 0, got %d err=%v", len(none), err)
	}
}

func deviceIDs(ds []Device) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.DeviceID
	}
	return out
}
