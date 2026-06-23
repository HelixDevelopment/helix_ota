package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// This file covers the list/get handler repo-error 500 branches: when the
// store.Repository fails (DB down / query error), the read handlers MUST return
// a clean 500 INTERNAL with the structured error envelope (code + message +
// request_id) — never a panic, never a leak of the underlying error string.
//
// It builds a test-local error-injecting Repository (errInjectRepo) that embeds
// the in-memory repo and, when armed, returns a fixed error from exactly one
// method per test. The error message is a sentinel (errInjected) so each test
// can ALSO assert the handler did NOT leak the raw store error into the JSON
// body — proving the envelope is structured, not a passthrough. Every test
// fails if its specific `if err != nil { ...500 }` guard regresses (a panic, a
// 200 with a truncated list, or a leaked message).

// errInjected is the sentinel error every armed repo method returns. Its text
// is distinctive so a leak into the response body is detectable.
var errInjected = errors.New("INJECTED-STORE-FAILURE-sentinel-d34db33f")

// errInjectRepo embeds the in-memory repository and overrides the read methods
// the list/get handlers depend on. When the matching field is non-empty, the
// method returns errInjected instead of delegating — modelling a DB-down /
// query error on exactly that seam. All non-armed methods delegate unchanged,
// so seeding fixtures through the embedded repo behaves normally.
type errInjectRepo struct {
	*store.MemoryRepository

	failListProjects        bool
	failGetProject          bool
	failGetProjectAccess    bool
	failListDevices         bool
	failGetDevice           bool
	failGetDeviceByHWID     bool
	failListReleases        bool
	failGetRelease          bool
	failListActiveDeploys   bool
	failGetDeployment       bool
	failListGroups          bool
	failGetGroup            bool
	failListGroupMembers    bool
	failListGroupMembersDet bool
	failListRollbacks       bool
	failTelemetryForDevice  bool
	failTelemetryCounts     bool
	failDeviceStateCounts   bool
	failListAudit           bool
}

func (r *errInjectRepo) ListProjects(ctx context.Context) ([]store.Project, error) {
	if r.failListProjects {
		return nil, errInjected
	}
	return r.MemoryRepository.ListProjects(ctx)
}

func (r *errInjectRepo) GetProject(ctx context.Context, id string) (store.Project, error) {
	if r.failGetProject {
		return store.Project{}, errInjected
	}
	return r.MemoryRepository.GetProject(ctx, id)
}

func (r *errInjectRepo) GetProjectAccess(ctx context.Context, callerID, projectID string) (store.ProjectAccess, error) {
	if r.failGetProjectAccess {
		return store.ProjectAccess{}, errInjected
	}
	return r.MemoryRepository.GetProjectAccess(ctx, callerID, projectID)
}

func (r *errInjectRepo) ListDevices(ctx context.Context, f store.DeviceFilter) ([]store.Device, string, error) {
	if r.failListDevices {
		return nil, "", errInjected
	}
	return r.MemoryRepository.ListDevices(ctx, f)
}

func (r *errInjectRepo) GetDevice(ctx context.Context, id string) (store.Device, error) {
	if r.failGetDevice {
		return store.Device{}, errInjected
	}
	return r.MemoryRepository.GetDevice(ctx, id)
}

func (r *errInjectRepo) GetDeviceByHardwareID(ctx context.Context, hwid string) (store.Device, error) {
	if r.failGetDeviceByHWID {
		return store.Device{}, errInjected
	}
	return r.MemoryRepository.GetDeviceByHardwareID(ctx, hwid)
}

func (r *errInjectRepo) ListReleases(ctx context.Context, f store.ReleaseFilter) ([]store.Release, string, error) {
	if r.failListReleases {
		return nil, "", errInjected
	}
	return r.MemoryRepository.ListReleases(ctx, f)
}

func (r *errInjectRepo) GetRelease(ctx context.Context, id string) (store.Release, error) {
	if r.failGetRelease {
		return store.Release{}, errInjected
	}
	return r.MemoryRepository.GetRelease(ctx, id)
}

func (r *errInjectRepo) ListActiveDeployments(ctx context.Context) ([]store.Deployment, error) {
	if r.failListActiveDeploys {
		return nil, errInjected
	}
	return r.MemoryRepository.ListActiveDeployments(ctx)
}

func (r *errInjectRepo) GetDeployment(ctx context.Context, id string) (store.Deployment, error) {
	if r.failGetDeployment {
		return store.Deployment{}, errInjected
	}
	return r.MemoryRepository.GetDeployment(ctx, id)
}

func (r *errInjectRepo) ListGroups(ctx context.Context) ([]store.Group, error) {
	if r.failListGroups {
		return nil, errInjected
	}
	return r.MemoryRepository.ListGroups(ctx)
}

func (r *errInjectRepo) GetGroup(ctx context.Context, id string) (store.Group, error) {
	if r.failGetGroup {
		return store.Group{}, errInjected
	}
	return r.MemoryRepository.GetGroup(ctx, id)
}

func (r *errInjectRepo) ListGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	if r.failListGroupMembers {
		return nil, errInjected
	}
	return r.MemoryRepository.ListGroupMembers(ctx, groupID)
}

func (r *errInjectRepo) ListGroupMembersDetailed(ctx context.Context, groupID string) ([]store.GroupMember, error) {
	if r.failListGroupMembersDet {
		return nil, errInjected
	}
	return r.MemoryRepository.ListGroupMembersDetailed(ctx, groupID)
}

func (r *errInjectRepo) ListRollbacks(ctx context.Context, deploymentID string) ([]store.RollbackRecord, error) {
	if r.failListRollbacks {
		return nil, errInjected
	}
	return r.MemoryRepository.ListRollbacks(ctx, deploymentID)
}

func (r *errInjectRepo) TelemetryForDevice(ctx context.Context, deviceID string) ([]store.TelemetryRecord, error) {
	if r.failTelemetryForDevice {
		return nil, errInjected
	}
	return r.MemoryRepository.TelemetryForDevice(ctx, deviceID)
}

func (r *errInjectRepo) TelemetryEventCounts(ctx context.Context) (map[string]int64, error) {
	if r.failTelemetryCounts {
		return nil, errInjected
	}
	return r.MemoryRepository.TelemetryEventCounts(ctx)
}

func (r *errInjectRepo) DeviceStateCounts(ctx context.Context) (map[string]int64, error) {
	if r.failDeviceStateCounts {
		return nil, errInjected
	}
	return r.MemoryRepository.DeviceStateCounts(ctx)
}

func (r *errInjectRepo) ListAudit(ctx context.Context, f store.AuditFilter) ([]store.AuditEntry, string, error) {
	if r.failListAudit {
		return nil, "", errInjected
	}
	return r.MemoryRepository.ListAudit(ctx, f)
}

// errEnv is a test environment whose Server is wired to an errInjectRepo, so a
// test can arm a specific seam and drive a request through the real router.
type errEnv struct {
	t      *testing.T
	srv    *Server
	repo   *errInjectRepo
	signer *TokenSigner
	idSeq  int
}

// newErrEnv builds the same deterministic environment as newTestEnv but with an
// error-injecting repository wrapping the in-memory store.
func newErrEnv(t *testing.T) *errEnv {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	repo := &errInjectRepo{MemoryRepository: store.NewMemoryRepository()}
	cfg := config.Config{
		APIBasePath:     "/api/v1",
		AccessTokenTTL:  15 * time.Minute,
		DeviceTokenTTL:  24 * time.Hour,
		MaxUploadBytes:  8 << 20,
		ArtifactBaseURL: "https://artifacts.test",
		TokenSecret:     []byte("test-secret"),
	}
	e := &errEnv{t: t, repo: repo}
	e.srv = NewServer(Options{
		Config: cfg,
		Repo:   repo,
		Users: NewStaticUserDirectory(StaticUser{
			Username: "admin@helix.test",
			Password: "s3cret",
			Roles:    []string{RoleAdmin, RoleOperator, RoleViewer},
		}),
		Health:      health.New(func(context.Context) bool { return true }),
		ArtifactKey: ed25519.PublicKey(pub),
		Now:         func() time.Time { return time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC) },
		NewID: func() string {
			e.idSeq++
			return fmt.Sprintf("eid-%04d", e.idSeq)
		},
	})
	e.signer = e.srv.signer
	return e
}

// do drives a request through the wired router and returns the recorder.
func (e *errEnv) do(method, path, token string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.srv.Router().ServeHTTP(w, r)
	return w
}

// adminToken mints an admin token (RoleAdmin/Operator/Viewer).
func (e *errEnv) adminToken() string {
	e.t.Helper()
	tok, err := e.signer.Mint("admin@helix.test", []string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint admin token: %v", err)
	}
	return tok
}

// viewerToken mints a non-admin viewer token, used to reach the non-admin
// branch of handleListProjects (which filters by project access).
func (e *errEnv) viewerToken() string {
	e.t.Helper()
	tok, err := e.signer.Mint("viewer@helix.test", []string{RoleViewer}, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint viewer token: %v", err)
	}
	return tok
}

// deviceToken mints a device-scoped token (for the device-telemetry read path).
func (e *errEnv) deviceToken(deviceID string) string {
	e.t.Helper()
	tok, err := e.signer.Mint(deviceID, []string{RoleDevice}, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint device token: %v", err)
	}
	return tok
}

// assert500Structured checks the recorder is a clean 500 INTERNAL with the
// structured envelope: code==INTERNAL, a non-empty human message, a non-empty
// request_id, and — critically — NO leak of the raw store error sentinel into
// the response body. A passthrough of errInjected, a panic-derived 500 without
// the envelope, or a 2xx all fail here.
func (e *errEnv) assert500Structured(w *httptest.ResponseRecorder) {
	e.t.Helper()
	if w.Code != http.StatusInternalServerError {
		e.t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
	if body := w.Body.String(); bytes.Contains([]byte(body), []byte(errInjected.Error())) {
		e.t.Fatalf("response leaked raw store error into body: %s", body)
	}
	var env ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		e.t.Fatalf("500 body is not the JSON error envelope: %v (%s)", err, w.Body.String())
	}
	if env.Error.Code != CodeInternal {
		e.t.Fatalf("want code INTERNAL, got %q (%s)", env.Error.Code, w.Body.String())
	}
	if env.Error.Message == "" {
		e.t.Fatalf("500 envelope has empty message (%s)", w.Body.String())
	}
	if env.Error.RequestID == "" {
		e.t.Fatalf("500 envelope missing request_id (%s)", w.Body.String())
	}
}

// TestListHandlerRepoErrorReturnsClean500 is the core table: each row arms one
// repo seam and drives the read handler that depends on it, asserting a clean
// structured 500. Fixtures are seeded through the embedded memory repo BEFORE
// the seam is armed where the handler needs an existing parent resource to
// reach the failing call.
func TestListHandlerRepoErrorReturnsClean500(t *testing.T) {
	type tc struct {
		name   string
		arm    func(r *errInjectRepo)
		setup  func(e *errEnv) // optional: seed fixtures + return path/token via fields below
		method string
		path   string
		token  func(e *errEnv) string
	}
	admin := func(e *errEnv) string { return e.adminToken() }

	cases := []tc{
		{
			name:   "ListProjects (admin path)",
			arm:    func(r *errInjectRepo) { r.failListProjects = true },
			method: http.MethodGet, path: "/api/v1/projects", token: admin,
		},
		{
			name:   "ListDevices",
			arm:    func(r *errInjectRepo) { r.failListDevices = true },
			method: http.MethodGet, path: "/api/v1/devices", token: admin,
		},
		{
			name:   "ListReleases",
			arm:    func(r *errInjectRepo) { r.failListReleases = true },
			method: http.MethodGet, path: "/api/v1/releases", token: admin,
		},
		{
			name:   "ListActiveDeployments",
			arm:    func(r *errInjectRepo) { r.failListActiveDeploys = true },
			method: http.MethodGet, path: "/api/v1/deployments", token: admin,
		},
		{
			name:   "ListGroups",
			arm:    func(r *errInjectRepo) { r.failListGroups = true },
			method: http.MethodGet, path: "/api/v1/groups", token: admin,
		},
		{
			name:   "ListRollbacks",
			arm:    func(r *errInjectRepo) { r.failListRollbacks = true },
			method: http.MethodGet, path: "/api/v1/deployments/dep-x/rollbacks", token: admin,
		},
		{
			name:   "TelemetryEventCounts (overview)",
			arm:    func(r *errInjectRepo) { r.failTelemetryCounts = true },
			method: http.MethodGet, path: "/api/v1/telemetry/overview", token: admin,
		},
		{
			name:   "ListAudit",
			arm:    func(r *errInjectRepo) { r.failListAudit = true },
			method: http.MethodGet, path: "/api/v1/audit", token: admin,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newErrEnv(t)
			if c.setup != nil {
				c.setup(e)
			}
			c.arm(e.repo)
			w := e.do(c.method, c.path, c.token(e))
			e.assert500Structured(w)
		})
	}
}

// TestTelemetryOverviewDeviceStateCountsError covers the SECOND repo call in
// handleTelemetryOverview: TelemetryEventCounts succeeds but DeviceStateCounts
// fails, exercising the by-state 500 branch distinctly from the counts branch.
func TestTelemetryOverviewDeviceStateCountsError(t *testing.T) {
	e := newErrEnv(t)
	e.repo.failDeviceStateCounts = true
	w := e.do(http.MethodGet, "/api/v1/telemetry/overview", e.adminToken())
	e.assert500Structured(w)
}

// TestDeviceTelemetryRepoError covers handleDeviceTelemetry's TelemetryForDevice
// 500 branch. The device must exist first so the handler reaches the read.
func TestDeviceTelemetryRepoError(t *testing.T) {
	e := newErrEnv(t)
	if err := e.repo.CreateDevice(context.Background(), store.Device{
		DeviceID: "dev-tele", HardwareID: "hw-tele", OSType: otaprotocol.OSAndroid,
		Model: "OrangePi5Max", RegisteredAt: e.srv.now(),
	}); err != nil {
		t.Fatalf("seed device: %v", err)
	}
	e.repo.failTelemetryForDevice = true
	w := e.do(http.MethodGet, "/api/v1/devices/dev-tele/telemetry", e.adminToken())
	e.assert500Structured(w)
}

// TestListGroupMembersRepoErrorIs404 documents the actual contract: the members
// list handler maps a ListGroupMembersDetailed error to 404 NOT_FOUND (it
// treats any failure as "group not found"), NOT a 500. This is by design — the
// memory repo returns ErrNotFound for an unknown group — so an injected error
// here surfaces as 404. Asserting it pins the branch and proves the handler
// does not panic on a repo error.
func TestListGroupMembersRepoErrorIs404(t *testing.T) {
	e := newErrEnv(t)
	e.repo.failListGroupMembersDet = true
	w := e.do(http.MethodGet, "/api/v1/groups/g-x/members", e.adminToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("ListGroupMembersDetailed error want 404 (handler maps repo error to not-found), got %d (%s)", w.Code, w.Body.String())
	}
	var env ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("404 body not envelope: %v", err)
	}
	if env.Error.Code != CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %q", env.Error.Code)
	}
}

// TestListProjectsNonAdminRepoError covers the non-admin branch of
// handleListProjects: a viewer token takes the filter path, and a ListProjects
// failure there returns the dedicated 500 (a distinct guard from the admin
// path's). Regression (dropping the listErr check) would 200 with no items.
func TestListProjectsNonAdminRepoError(t *testing.T) {
	e := newErrEnv(t)
	e.repo.failListProjects = true
	w := e.do(http.MethodGet, "/api/v1/projects", e.viewerToken())
	e.assert500Structured(w)
}

// TestListProjectsNonAdminFilterAppend covers the non-admin success branch of
// handleListProjects: a viewer WITH access to a seeded project takes the filter
// path and appends that project to the result (line 163-165). This is the
// complement of TestListProjectsNonAdminRepoError (which fails the list). Both
// together exercise the full non-admin branch.
func TestListProjectsNonAdminFilterAppend(t *testing.T) {
	e := newErrEnv(t)
	ctx := context.Background()
	if err := e.repo.CreateProject(ctx, store.Project{
		ProjectID: "proj-vis", Name: "visible", CreatedAt: e.srv.now(), UpdatedAt: e.srv.now(),
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := e.repo.CreateProject(ctx, store.Project{
		ProjectID: "proj-hidden", Name: "hidden", CreatedAt: e.srv.now(), UpdatedAt: e.srv.now(),
	}); err != nil {
		t.Fatalf("seed hidden project: %v", err)
	}
	// Grant the viewer access to exactly one of the two projects.
	if err := e.repo.SetProjectAccess(ctx, store.ProjectAccess{
		ProjectID: "proj-vis", CallerID: "viewer@helix.test", Role: store.ProjectRoleViewer,
	}); err != nil {
		t.Fatalf("grant access: %v", err)
	}
	w := e.do(http.MethodGet, "/api/v1/projects", e.viewerToken())
	if w.Code != http.StatusOK {
		t.Fatalf("non-admin project list want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Items []ProjectResponse `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v (%s)", err, w.Body.String())
	}
	if len(body.Items) != 1 || body.Items[0].ProjectID != "proj-vis" {
		t.Fatalf("non-admin filter want exactly [proj-vis], got %+v", body.Items)
	}
}

// TestGetProjectOwnReadError covers handleGetProject's OWN GetProject 404 branch
// (line 188-191), distinct from the access-bypass GetProject. A non-admin viewer
// with granted access passes requireProjectAccess (GetProjectAccess succeeds),
// then the handler's GetProject is armed to fail — yielding the 404. Regression
// (returning the project anyway) would 200.
func TestGetProjectOwnReadError(t *testing.T) {
	e := newErrEnv(t)
	ctx := context.Background()
	if err := e.repo.CreateProject(ctx, store.Project{
		ProjectID: "proj-rd", Name: "readable", CreatedAt: e.srv.now(), UpdatedAt: e.srv.now(),
	}); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if err := e.repo.SetProjectAccess(ctx, store.ProjectAccess{
		ProjectID: "proj-rd", CallerID: "viewer@helix.test", Role: store.ProjectRoleViewer,
	}); err != nil {
		t.Fatalf("grant access: %v", err)
	}
	// Arm the handler's GetProject (access check used GetProjectAccess, which is
	// left healthy, so requireProjectAccess passes for the non-admin path).
	e.repo.failGetProject = true
	w := e.do(http.MethodGet, "/api/v1/projects/proj-rd", e.viewerToken())
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetProject read error want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("404 body not envelope: %v", err)
	}
	if env.Error.Code != CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %q", env.Error.Code)
	}
}

// TestGroupListMemberRollupRepoError covers handleListGroups' per-group member
// rollup: ListGroups succeeds (a seeded group exists) but ListGroupMembers (the
// rollup call inside groupViewWithMembers) fails. The handler tolerates the
// rollup error by rendering an empty member set, so this MUST stay 200 — proving
// the rollup failure does not crash the list. Asserting it locks that behaviour.
func TestGroupListMemberRollupRepoError(t *testing.T) {
	e := newErrEnv(t)
	if err := e.repo.CreateGroup(context.Background(), store.Group{
		ID: "g-roll", Name: "rollup", CreatedAt: e.srv.now(),
	}); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	e.repo.failListGroupMembers = true
	w := e.do(http.MethodGet, "/api/v1/groups", e.adminToken())
	// groupViewWithMembers swallows the rollup error (empty members), so the
	// list still succeeds. If a future change makes the rollup fatal, this guard
	// catches the behavioural shift.
	if w.Code != http.StatusOK {
		t.Fatalf("group list with member-rollup error want 200 (rollup error tolerated), got %d (%s)", w.Code, w.Body.String())
	}
}
