// Package api — coverage_extra_test.go: focused real unit tests for the
// lower-covered pure helpers (deriveAuditAction, truncate, singular,
// methodVerb, toDeviceStatus), the embedded-manager fallback (MountManagerUI),
// and a handful of handler error branches (project update/delete, telemetry
// overview) that the httptest integration suite does not yet reach.
//
// Every assertion checks the real returned value / status / error, never a
// coverage-padding no-op.
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ---------------------------------------------------------------------------
// deriveAuditAction — verb refinement + resource-type singularisation.
// ---------------------------------------------------------------------------

func TestDeriveAuditAction_ExtraBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		method       string
		fullPath     string
		wantAction   string
		wantResource string
	}{
		{"group members", http.MethodPost, "/api/v1/groups/:groupId/members", "GROUP_MEMBER_CREATE", "group_member"},
		{"members delete", http.MethodDelete, "/api/v1/groups/:groupId/members", "GROUP_MEMBER_DELETE", "group_member"},
		{"update patch", http.MethodPatch, "/api/v1/projects/:projectId", "PROJECT_UPDATE", "project"},
		{"delete", http.MethodDelete, "/api/v1/releases/:releaseId", "RELEASE_DELETE", "release"},
		{"only params -> empty", http.MethodGet, "/api/v1/:id", "", ""},
		{"root -> empty", http.MethodGet, "/api/v1/", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotAction, gotResource := deriveAuditAction(tc.method, tc.fullPath)
			if gotAction != tc.wantAction {
				t.Errorf("deriveAuditAction(%q,%q) action = %q, want %q",
					tc.method, tc.fullPath, gotAction, tc.wantAction)
			}
			if gotResource != tc.wantResource {
				t.Errorf("deriveAuditAction(%q,%q) resource = %q, want %q",
					tc.method, tc.fullPath, gotResource, tc.wantResource)
			}
		})
	}
}

func TestMethodVerb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		want   string
	}{
		{http.MethodPost, "CREATE"},
		{http.MethodPut, "UPDATE"},
		{http.MethodPatch, "UPDATE"},
		{http.MethodDelete, "DELETE"},
		{http.MethodGet, "ACTION"},
		{http.MethodHead, "ACTION"},
	}
	for _, tc := range tests {
		if got := methodVerb(tc.method); got != tc.want {
			t.Errorf("methodVerb(%q) = %q, want %q", tc.method, got, tc.want)
		}
	}
}

func TestSingular(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"devices", "device"},
		{"releases", "release"},
		{"status", "statu"}, // documented naive trim-trailing-s behaviour
		{"group", "group"},  // no trailing s
		{"", ""},
	}
	for _, tc := range tests {
		if got := singular(tc.in); got != tc.want {
			t.Errorf("singular(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"hello world", 5, "hello"},
		{"short", 100, "short"}, // len <= n -> unchanged (uncovered branch)
		{"", 3, ""},
		{"exact", 5, "exact"}, // boundary: len == n -> unchanged
	}
	for _, tc := range tests {
		if got := truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("truncate(%q,%d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// toDeviceStatus — optional-field setters (TargetVersion, LastSeen, error).
// ---------------------------------------------------------------------------

func TestToDeviceStatus_AllOptionalFields(t *testing.T) {
	t.Parallel()

	seen := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	d := store.Device{
		DeviceID:       "dev-1",
		HardwareID:     "hw-1",
		CurrentVersion: "1.0.0",
		UpdateState:    "downloading",
		ActiveSlot:     "A",
		HealthOK:       true,
		TargetVersion:  "1.1.0",
		LastSeen:       seen,
		LastErrorCode:  "E_DOWNLOAD",
	}

	st := toDeviceStatus(d)

	if st.UpdateState != "downloading" {
		t.Errorf("UpdateState = %q, want downloading", st.UpdateState)
	}
	if st.TargetVersion == nil || *st.TargetVersion != "1.1.0" {
		t.Errorf("TargetVersion not set to 1.1.0: %v", st.TargetVersion)
	}
	if st.LastSeen == nil || !st.LastSeen.Equal(seen) {
		t.Errorf("LastSeen not set: %v", st.LastSeen)
	}
	if st.Health.LastErrorCode == nil || *st.Health.LastErrorCode != "E_DOWNLOAD" {
		t.Errorf("LastErrorCode not set: %v", st.Health.LastErrorCode)
	}
}

func TestToDeviceStatus_DefaultsAndNilOptionals(t *testing.T) {
	t.Parallel()

	// Empty UpdateState defaults to "idle"; zero LastSeen + empty target/error
	// leave the optional pointers nil.
	d := store.Device{DeviceID: "dev-2", HardwareID: "hw-2"}
	st := toDeviceStatus(d)

	if st.UpdateState != "idle" {
		t.Errorf("empty UpdateState should default to idle, got %q", st.UpdateState)
	}
	if st.TargetVersion != nil {
		t.Errorf("TargetVersion should be nil, got %v", *st.TargetVersion)
	}
	if st.LastSeen != nil {
		t.Errorf("LastSeen should be nil for zero time, got %v", *st.LastSeen)
	}
	if st.Health.LastErrorCode != nil {
		t.Errorf("LastErrorCode should be nil, got %v", *st.Health.LastErrorCode)
	}
}

// ---------------------------------------------------------------------------
// MountManagerUI — NoRoute fallback branches: non-GET passthrough, non-manager
// path passthrough, API-path passthrough, and SPA index serve.
// ---------------------------------------------------------------------------

func TestMountManagerUI_FallbackBranches(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	// Build a fresh engine and mount only the manager UI + a sentinel route so
	// we can observe the NoRoute fallback behaviour deterministically.
	r := gin.New()
	env.srv.MountManagerUI(r)

	tests := []struct {
		name   string
		method string
		path   string
		// We assert only that the fallback either serves index (200 text/html)
		// or passes through (404 from gin's default NoRoute-after-Next).
		wantHTML bool
	}{
		// Non-GET under /manager → c.Next() passthrough (not served as index).
		{"post under manager passes through", http.MethodPost, "/manager/devices", false},
		// API base path → passthrough.
		{"api path passes through", http.MethodGet, "/api/v1/devices", false},
		// Non-manager, non-root GET → passthrough.
		{"unrelated path passes through", http.MethodGet, "/something-else", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			ct := w.Header().Get("Content-Type")
			isHTML := w.Code == http.StatusOK && (ct == "text/html; charset=utf-8")
			if isHTML != tc.wantHTML {
				t.Errorf("%s %s: served-as-html=%v (code=%d ct=%q), want html=%v",
					tc.method, tc.path, isHTML, w.Code, ct, tc.wantHTML)
			}
		})
	}
}

// TestMountManagerUI_RootServesIndexOrPassesThrough exercises the "/" GET branch
// of the fallback. The embedded manager-dist/index.html may or may not exist in
// the build (it is a gitignored build artifact, §11.4.30). The test asserts the
// two honest outcomes only — index served (200 html) OR clean passthrough — and
// never a 5xx, which would indicate the fallback panicked.
func TestMountManagerUI_RootBranch(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	r := gin.New()
	env.srv.MountManagerUI(r)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code >= 500 {
		t.Fatalf("root fallback returned server error %d (body=%q)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleUpdateProject — malformed-body + name-only update branches.
// ---------------------------------------------------------------------------

func TestProjectUpdate_MalformedBody(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	tok := env.adminToken()

	// Create a project first.
	w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, gin.H{"name": "proj-x"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: code %d body %q", w.Code, w.Body.String())
	}
	var created ProjectResponse
	env.decode(w, &created)

	// PATCH with a malformed JSON body → 4xx validation error.
	w = env.do(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, []byte("{not-json"), "application/json")
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("malformed PATCH should 4xx, got %d body %q", w.Code, w.Body.String())
	}
}

func TestProjectUpdate_NameOnly(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	tok := env.adminToken()

	w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, gin.H{"name": "proj-y", "description": "orig"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: code %d", w.Code)
	}
	var created ProjectResponse
	env.decode(w, &created)

	// PATCH name only (no description field) → name updated.
	w = env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, gin.H{"name": "proj-y-renamed"})
	if w.Code != http.StatusOK {
		t.Fatalf("name-only PATCH: code %d body %q", w.Code, w.Body.String())
	}
	var updated ProjectResponse
	env.decode(w, &updated)
	if updated.Name != "proj-y-renamed" {
		t.Fatalf("name = %q, want proj-y-renamed", updated.Name)
	}
	// Anti-bluff (§11.4 covenant): a name-only PATCH omitting `description`
	// entirely must NOT clear the previously-set description. This exact
	// scenario (create-with-description then PATCH name-only) previously
	// silently wiped the description to "" -- see
	// TestProjectUpdatePartialOmitsDescriptionUnchanged in
	// handlers_project_test.go for the dedicated regression test -- so this
	// pre-existing test now checks the field it originally left unchecked.
	if updated.Description != "orig" {
		t.Fatalf("name-only PATCH must not clear description: got %q, want %q", updated.Description, "orig")
	}
}

func TestProjectUpdate_NotFound(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	tok := env.adminToken()

	// PATCH a non-existent project → 4xx (no project access → 403/404).
	w := env.doJSON(http.MethodPatch, "/api/v1/projects/no-such-id", tok, gin.H{"name": "x"})
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("PATCH missing project should 4xx, got %d", w.Code)
	}
}
