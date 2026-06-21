// Package api — targeted coverage tests for requireProjectAccess (56.0 %)
// and deriveProgress (52.4 %), the two lowest-coverage functions in the
// api package as of the 2026-06-21 baseline.
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ---------------------------------------------------------------------------
// requireProjectAccess
// ---------------------------------------------------------------------------

// TestRequireProjectAccessUnauthenticated proves that when no claims are
// present in the gin context the function returns 401 UNAUTHENTICATED.
func TestRequireProjectAccessUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	repo := store.NewMemoryRepository()

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleViewer)
	if callerID != "" {
		t.Fatalf("callerID want empty, got %q", callerID)
	}
	if access != (store.ProjectAccess{}) {
		t.Fatalf("access want zero, got %+v", access)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status want 401, got %d", rec.Code)
	}
}

// TestRequireProjectAccessAdminBypassNoProject proves that an admin gets
// 404 NOT_FOUND when the project does not exist (the admin bypass calls
// GetProject which returns ErrNotFound).
func TestRequireProjectAccessAdminBypassNoProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "admin@test",
		Roles:   []string{RoleAdmin},
	})
	repo := store.NewMemoryRepository()

	callerID, access := requireProjectAccess(c, repo, "nonexistent", store.ProjectRoleAdmin)
	if callerID != "" {
		t.Fatalf("callerID want empty, got %q", callerID)
	}
	if access != (store.ProjectAccess{}) {
		t.Fatalf("access want zero, got %+v", access)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status want 404, got %d", rec.Code)
	}
}

// TestRequireProjectAccessAdminBypassOk proves that an admin gains
// implicit ProjectRoleAdmin access to any existing project.
func TestRequireProjectAccessAdminBypassOk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-1", Name: "test-project",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "admin@test",
		Roles:   []string{RoleAdmin},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleAdmin)
	if callerID != "admin@test" {
		t.Fatalf("callerID want admin@test, got %q", callerID)
	}
	if access.Role != store.ProjectRoleAdmin {
		t.Fatalf("role want admin, got %q", access.Role)
	}
	if access.ProjectID != "proj-1" {
		t.Fatalf("project id want proj-1, got %q", access.ProjectID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestRequireProjectAccessAdminBypassPointerClaims proves that the admin
// bypass also works when claims are stored as *Claims pointer.
func TestRequireProjectAccessAdminBypassPointerClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-2", Name: "another-project",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "boss@test",
		Roles:   []string{RoleAdmin},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-2", store.ProjectRoleAdmin)
	if callerID != "boss@test" {
		t.Fatalf("callerID want boss@test, got %q", callerID)
	}
	if access.Role != store.ProjectRoleAdmin {
		t.Fatalf("role want admin, got %q", access.Role)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d", rec.Code)
	}
}

// TestRequireProjectAccessNoAccess proves a caller with no access record
// receives 403 FORBIDDEN.
func TestRequireProjectAccessNoAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-1", Name: "test-project",
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "viewer@test",
		Roles:   []string{RoleViewer},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleViewer)
	if callerID != "" {
		t.Fatalf("callerID want empty, got %q", callerID)
	}
	if access != (store.ProjectAccess{}) {
		t.Fatalf("access want zero, got %+v", access)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status want 403, got %d", rec.Code)
	}
}

// TestRequireProjectAccessInsufficientRole proves a caller with a role
// below the minimum receives 403 FORBIDDEN.
func TestRequireProjectAccessInsufficientRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-1", Name: "test-project",
	})
	_ = repo.SetProjectAccess(context.Background(), store.ProjectAccess{
		ProjectID: "proj-1", CallerID: "viewer@test", Role: store.ProjectRoleViewer,
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "viewer@test",
		Roles:   []string{RoleViewer},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleAdmin)
	if callerID != "" {
		t.Fatalf("callerID want empty, got %q", callerID)
	}
	if access != (store.ProjectAccess{}) {
		t.Fatalf("access want zero, got %+v", access)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status want 403, got %d", rec.Code)
	}
}

// TestRequireProjectAccessSufficientRole proves a caller with an adequate
// role is granted access.
func TestRequireProjectAccessSufficientRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-1", Name: "test-project",
	})
	_ = repo.SetProjectAccess(context.Background(), store.ProjectAccess{
		ProjectID: "proj-1", CallerID: "op@test", Role: store.ProjectRoleOperator,
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "op@test",
		Roles:   []string{RoleOperator},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleViewer)
	if callerID != "op@test" {
		t.Fatalf("callerID want op@test, got %q", callerID)
	}
	if access.Role != store.ProjectRoleOperator {
		t.Fatalf("role want operator, got %q", access.Role)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d", rec.Code)
	}
}

// TestRequireProjectAccessViewerCanView proves a viewer with the correct
// role can access a project at viewer level.
func TestRequireProjectAccessViewerCanView(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := store.NewMemoryRepository()
	_ = repo.CreateProject(context.Background(), store.Project{
		ProjectID: "proj-1", Name: "test-project",
	})
	_ = repo.SetProjectAccess(context.Background(), store.ProjectAccess{
		ProjectID: "proj-1", CallerID: "viewer@test", Role: store.ProjectRoleViewer,
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxClaims, &Claims{
		Subject: "viewer@test",
		Roles:   []string{RoleViewer},
	})

	callerID, access := requireProjectAccess(c, repo, "proj-1", store.ProjectRoleViewer)
	if callerID != "viewer@test" {
		t.Fatalf("callerID want viewer@test, got %q", callerID)
	}
	if access.Role != store.ProjectRoleViewer {
		t.Fatalf("role want viewer, got %q", access.Role)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// deriveProgress
// ---------------------------------------------------------------------------

// addTelemetry is a test helper that appends a telemetry record to the
// in-memory repository.
func addTelemetry(t *testing.T, repo *store.MemoryRepository, deviceID, deploymentID string, event otaprotocol.TelemetryEvent) {
	t.Helper()
	err := repo.AppendTelemetry(context.Background(), store.TelemetryRecord{
		DeviceID:     deviceID,
		DeploymentID: deploymentID,
		Event:        event,
		Version:      "1.0.0",
		Timestamp:    time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("append telemetry: %v", err)
	}
}

// TestDeriveProgressEmpty proves deriveProgress returns zero progress
// when the repo has no telemetry for the deployment (or the deployment
// does not exist).
func TestDeriveProgressEmpty(t *testing.T) {
	env := newTestEnv(t)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  0,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p != (DeploymentProgress{}) {
		t.Fatalf("progress want zero, got %+v", p)
	}
}

// TestDeriveProgressPendingOnly proves that when TargetCount > 0 and
// there is no telemetry, all targets show as Pending.
func TestDeriveProgressPendingOnly(t *testing.T) {
	env := newTestEnv(t)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  5,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Pending != 5 {
		t.Fatalf("pending want 5, got %+v", p)
	}
}

// TestDeriveProgressSingleDeviceSuccess proves a single device that
// reported EventSuccess is counted as Succeeded (and not as Pending).
func TestDeriveProgressSingleDeviceSuccess(t *testing.T) {
	env := newTestEnv(t)

	addTelemetry(t, env.repo, "dev-1", "dep-1", otaprotocol.EventSuccess)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  1,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Succeeded != 1 {
		t.Fatalf("succeeded want 1, got %+v", p)
	}
	if p.Pending != 0 {
		t.Fatalf("pending want 0, got %+v", p)
	}
}

// TestDeriveProgressAllEventTypes proves every event type maps to the
// correct progress counter.
func TestDeriveProgressAllEventTypes(t *testing.T) {
	env := newTestEnv(t)

	// One device per event type so each is counted independently.
	addTelemetry(t, env.repo, "dev-dl", "dep-1", otaprotocol.EventDownloadStarted)
	addTelemetry(t, env.repo, "dev-ins", "dep-1", otaprotocol.EventInstalling)
	addTelemetry(t, env.repo, "dev-installed", "dep-1", otaprotocol.EventInstalled)
	addTelemetry(t, env.repo, "dev-ver", "dep-1", otaprotocol.EventVerifying)
	addTelemetry(t, env.repo, "dev-ok", "dep-1", otaprotocol.EventSuccess)
	addTelemetry(t, env.repo, "dev-fail", "dep-1", otaprotocol.EventFailure)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  6,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Downloading != 1 {
		t.Fatalf("downloading want 1, got %d", p.Downloading)
	}
	if p.Installed != 3 { // installing + installed + verifying
		t.Fatalf("installed want 3, got %d", p.Installed)
	}
	if p.Succeeded != 1 {
		t.Fatalf("succeeded want 1, got %d", p.Succeeded)
	}
	if p.Failed != 1 {
		t.Fatalf("failed want 1, got %d", p.Failed)
	}
	if p.Pending != 0 {
		t.Fatalf("pending want 0, got %d", p.Pending)
	}
}

// TestDeriveProgressLatestEventPerDevice proves that when a device reports
// multiple events only the latest (by timestamp) is counted.
func TestDeriveProgressLatestEventPerDevice(t *testing.T) {
	env := newTestEnv(t)

	// dev-1 reports DownloadStarted first, then Success later.
	base := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	_ = env.repo.AppendTelemetry(context.Background(), store.TelemetryRecord{
		DeviceID: "dev-1", DeploymentID: "dep-1",
		Event: otaprotocol.EventDownloadStarted, Version: "1.0.0",
		Timestamp: base,
	})
	_ = env.repo.AppendTelemetry(context.Background(), store.TelemetryRecord{
		DeviceID: "dev-1", DeploymentID: "dep-1",
		Event: otaprotocol.EventSuccess, Version: "1.0.0",
		Timestamp: base.Add(5 * time.Minute),
	})

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  1,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Succeeded != 1 {
		t.Fatalf("succeeded want 1 (latest event wins), got %+v", p)
	}
	if p.Downloading != 0 {
		t.Fatalf("downloading want 0 (superseded by later event), got %d", p.Downloading)
	}
}

// TestDeriveProgressMultipleDeploymentsIsolated proves telemetry for
// different deployments does not cross-contaminate.
func TestDeriveProgressMultipleDeploymentsIsolated(t *testing.T) {
	env := newTestEnv(t)

	addTelemetry(t, env.repo, "dev-a", "dep-A", otaprotocol.EventSuccess)
	addTelemetry(t, env.repo, "dev-b", "dep-A", otaprotocol.EventFailure)
	addTelemetry(t, env.repo, "dev-c", "dep-B", otaprotocol.EventSuccess)

	depA := store.Deployment{DeploymentID: "dep-A", TargetCount: 2}
	pA := env.srv.deriveProgress(context.Background(), depA)
	if pA.Succeeded != 1 || pA.Failed != 1 || pA.Pending != 0 {
		t.Fatalf("dep-A want 1 succeeded, 1 failed, got %+v", pA)
	}

	depB := store.Deployment{DeploymentID: "dep-B", TargetCount: 1}
	pB := env.srv.deriveProgress(context.Background(), depB)
	if pB.Succeeded != 1 || pB.Pending != 0 {
		t.Fatalf("dep-B want 1 succeeded, got %+v", pB)
	}
}

// TestDeriveProgressPendingWithTelemetry proves that Pending = TargetCount
// - unique-reporter-count, and does not go negative.
func TestDeriveProgressPendingWithTelemetry(t *testing.T) {
	env := newTestEnv(t)

	addTelemetry(t, env.repo, "dev-1", "dep-1", otaprotocol.EventSuccess)
	addTelemetry(t, env.repo, "dev-2", "dep-1", otaprotocol.EventDownloadStarted)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  5,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Succeeded != 1 || p.Downloading != 1 {
		t.Fatalf("want 1 succeeded, 1 downloading, got %+v", p)
	}
	if p.Pending != 3 { // 5 - 2 devices reporting
		t.Fatalf("pending want 3 (5 target - 2 reporting), got %d", p.Pending)
	}
}

// TestDeriveProgressNoNegativePending proves Pending never goes below 0
// when TargetCount is smaller than the number of unique reporting devices.
func TestDeriveProgressNoNegativePending(t *testing.T) {
	env := newTestEnv(t)

	addTelemetry(t, env.repo, "dev-1", "dep-1", otaprotocol.EventSuccess)
	addTelemetry(t, env.repo, "dev-2", "dep-1", otaprotocol.EventFailure)

	dep := store.Deployment{
		DeploymentID: "dep-1",
		TargetCount:  0,
	}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p.Pending != 0 {
		t.Fatalf("pending want 0 (no negative), got %d", p.Pending)
	}
}

// errorOnTelemetryRepo wraps a Repository and injects a fake error for a
// specific deployment ID, so the err != nil branch of deriveProgress is
// exercised.
type errorOnTelemetryRepo struct {
	store.Repository
	errDeploymentID string
}

func (r *errorOnTelemetryRepo) TelemetryForDeployment(_ context.Context, deploymentID string) ([]store.TelemetryRecord, error) {
	if deploymentID == r.errDeploymentID {
		return nil, store.ErrNotFound
	}
	return r.Repository.TelemetryForDeployment(context.Background(), deploymentID)
}

// TestDeriveProgressTelemetryError proves that when TelemetryForDeployment
// returns an error, deriveProgress returns an empty progress without panic.
func TestDeriveProgressTelemetryError(t *testing.T) {
	env := newTestEnv(t)

	wrapped := &errorOnTelemetryRepo{Repository: env.repo, errDeploymentID: "dep-err"}
	env.srv.repo = wrapped

	dep := store.Deployment{DeploymentID: "dep-err", TargetCount: 10}
	p := env.srv.deriveProgress(context.Background(), dep)
	if p != (DeploymentProgress{}) {
		t.Fatalf("progress want zero on error, got %+v", p)
	}

	// Verify non-err deployment still works.
	addTelemetry(t, env.repo, "dev-1", "dep-ok", otaprotocol.EventSuccess)
	depOK := store.Deployment{DeploymentID: "dep-ok", TargetCount: 1}
	pOK := env.srv.deriveProgress(context.Background(), depOK)
	if pOK.Succeeded != 1 {
		t.Fatalf("non-error deployment want 1 succeeded, got %+v", pOK)
	}
}
