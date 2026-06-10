package api

import (
	"net/http"
	"strings"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// createReleaseFor uploads + releases an artifact and returns the release id.
func createReleaseFor(t *testing.T, env *testEnv, version string) string {
	t.Helper()
	artID := uploadValid(t, env, version)
	w := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: artID, Version: version, OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("release want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var rel Release
	env.decode(w, &rel)
	return rel.ReleaseID
}

func TestDeploymentCreateAllTargets(t *testing.T) {
	env := newTestEnv(t)
	// Register a matching device so target_count is meaningful.
	registerDevice(t, env, DeviceRegistration{HardwareID: "dep-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	relID := createReleaseFor(t, env, "1.1.0")

	w := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: relID, Strategy: "all-targets",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("deployment want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var dep Deployment
	env.decode(w, &dep)
	if dep.Strategy != "all-targets" || dep.Status != string(otaprotocol.DeploymentActive) {
		t.Fatalf("deployment fields mismatch: %+v", dep)
	}
	if dep.TargetCount != 1 {
		t.Fatalf("target_count want 1, got %d", dep.TargetCount)
	}

	g := env.do(http.MethodGet, "/api/v1/deployments/"+dep.DeploymentID, env.adminToken(), nil, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get deployment want 200, got %d", g.Code)
	}
	var st DeploymentStatus
	env.decode(g, &st)
	// One targeted device, no telemetry yet -> pending=1.
	if st.Progress.Pending != 1 {
		t.Fatalf("progress pending want 1, got %+v", st.Progress)
	}
}

// TestDeploymentListReturnsActive proves GET /deployments returns the active
// deployments (closes the GAP the emulator surfaced: an operator/agent could
// create a deployment but had no list endpoint to enumerate active ones). The
// seeded deployment must appear with its id, release id, and active status.
func TestDeploymentListReturnsActive(t *testing.T) {
	env := newTestEnv(t)
	registerDevice(t, env, DeviceRegistration{HardwareID: "list-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	relID := createReleaseFor(t, env, "1.1.0")
	cw := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: relID, Strategy: "all-targets",
	})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create deployment want 201, got %d (%s)", cw.Code, cw.Body.String())
	}
	var created Deployment
	env.decode(cw, &created)

	w := env.do(http.MethodGet, "/api/v1/deployments", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list deployments want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var list DeploymentList
	env.decode(w, &list)
	if len(list.Items) != 1 {
		t.Fatalf("want 1 active deployment, got %d: %+v", len(list.Items), list)
	}
	got := list.Items[0]
	if got.DeploymentID != created.DeploymentID {
		t.Fatalf("listed deployment id = %q, want %q", got.DeploymentID, created.DeploymentID)
	}
	if got.ReleaseID != relID {
		t.Fatalf("listed release id = %q, want %q", got.ReleaseID, relID)
	}
	if got.Status != string(otaprotocol.DeploymentActive) {
		t.Fatalf("listed status = %q, want active", got.Status)
	}
}

// TestDeploymentListEmpty proves the list endpoint returns an empty (non-null)
// items array when no deployments exist.
func TestDeploymentListEmpty(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/deployments", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list deployments want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"items":[]`) {
		t.Fatalf("empty list must render items:[], got %s", got)
	}
}

// TestDeploymentListForbidsDeviceToken proves the list endpoint is operator/
// viewer-scoped: a device-scoped token is forbidden (matches the RBAC of the
// sibling list endpoints).
func TestDeploymentListForbidsDeviceToken(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/deployments", env.deviceToken("some-device"), nil, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("device token on list deployments want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDeploymentRejectsNonAllTargets(t *testing.T) {
	env := newTestEnv(t)
	relID := createReleaseFor(t, env, "1.1.0")
	w := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: relID, Strategy: "canary",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("staged strategy want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeValidationFailed {
		t.Fatalf("want VALIDATION_FAILED, got %s", got)
	}
}

func TestDeploymentUnknownRelease(t *testing.T) {
	env := newTestEnv(t)
	w := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: "missing", Strategy: "all-targets",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown release want 404, got %d", w.Code)
	}
}

func TestDeploymentConflictWhenActive(t *testing.T) {
	env := newTestEnv(t)
	relID := createReleaseFor(t, env, "1.1.0")
	first := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: relID, Strategy: "all-targets",
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("first deployment want 201, got %d", first.Code)
	}
	// A second active deployment for the same target set conflicts.
	second := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: relID, Strategy: "all-targets",
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("overlapping deployment want 409, got %d (%s)", second.Code, second.Body.String())
	}
}
