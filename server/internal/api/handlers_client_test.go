package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestClientUpdateOffersDelta proves delta selection: a device on 1.0.0 with a
// registered 1.0.0->1.1.0 delta gets the delta sidecar in its update offer;
// without a registered delta the offer carries no delta (TestClientUpdate200WhenBehind).
func TestClientUpdateOffersDelta(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "1.0.0", "1.1.0") // device@1.0.0, target release+artifact@1.1.0
	ctx := context.Background()

	// Resolve the target (1.1.0) artifact.
	target, err := env.repo.ReleaseByVersion(ctx, otaprotocol.OSAndroid, "OrangePi5Max", "1.1.0")
	if err != nil {
		t.Fatalf("resolve target release: %v", err)
	}
	// Insert a base artifact + base release at 1.0.0 (direct — the API's S4 would
	// reject an older version), then register the 1.0.0->1.1.0 delta.
	if err := env.repo.CreateArtifact(ctx, store.Artifact{ArtifactID: "base-art", SHA256: "basehash",
		Size: 100, OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Version: "1.0.0", Verified: true}); err != nil {
		t.Fatalf("base artifact: %v", err)
	}
	if err := env.repo.CreateRelease(ctx, store.Release{ReleaseID: "base-rel", ArtifactID: "base-art",
		Version: "1.0.0", OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published", CreatedAt: env.srv.now()}); err != nil {
		t.Fatalf("base release: %v", err)
	}
	if err := env.repo.CreateDelta(ctx, store.DeltaArtifact{ID: "delta-1", BaseArtifactID: "base-art",
		TargetArtifactID: target.ArtifactID, SHA256: "deltahash", Size: 42, StorageRef: "s3://d/1", CreatedAt: env.srv.now()}); err != nil {
		t.Fatalf("create delta: %v", err)
	}

	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("behind device want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var upd otaprotocol.UpdateAvailable
	env.decode(w, &upd)
	if upd.Delta == nil {
		t.Fatalf("expected a delta offer, got none: %+v", upd)
	}
	if upd.Delta.BaseVersion != "1.0.0" || upd.Delta.SHA256 != "deltahash" || upd.Delta.Size != 42 || upd.Delta.URL == "" {
		t.Fatalf("delta offer mismatch: %+v", upd.Delta)
	}
	// The full payload is still present as the fallback.
	if upd.URL == "" || upd.SHA256 == "" {
		t.Fatalf("full payload must remain as fallback: %+v", upd)
	}
}

// setupDeployment registers a device, uploads+releases an artifact at version,
// and creates an all-targets deployment. It returns the device and the release
// version so update-check tests can drive 204 vs 200.
func setupDeployment(t *testing.T, env *testEnv, deviceCurrent, releaseVersion string) DeviceRegistered {
	t.Helper()
	dev := registerDevice(t, env, DeviceRegistration{
		HardwareID:     "client-hw",
		Model:          "OrangePi5Max",
		OS:             otaprotocol.OSAndroid,
		CurrentVersion: deviceCurrent,
		Group:          "field-fleet-a",
	})

	payload := []byte("client payload " + releaseVersion)
	file := zipStored(t, payload)
	meta := env.validMeta(file, releaseVersion)
	body, ct := uploadMultipart(t, file, meta)
	uw := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if uw.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d (%s)", uw.Code, uw.Body.String())
	}
	var art Artifact
	env.decode(uw, &art)

	rw := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art.ArtifactID, Version: releaseVersion, OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if rw.Code != http.StatusCreated {
		t.Fatalf("release want 201, got %d (%s)", rw.Code, rw.Body.String())
	}
	var rel Release
	env.decode(rw, &rel)

	dw := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: rel.ReleaseID, Strategy: "all-targets", Group: "field-fleet-a",
	})
	if dw.Code != http.StatusCreated {
		t.Fatalf("deployment want 201, got %d (%s)", dw.Code, dw.Body.String())
	}
	return dev
}

func TestClientUpdate200WhenBehind(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")

	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("behind device want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var upd otaprotocol.UpdateAvailable
	env.decode(w, &upd)
	if upd.Version != "1.1.0" {
		t.Fatalf("update version want 1.1.0, got %q", upd.Version)
	}
	if upd.URL == "" || upd.SHA256 == "" || upd.Signature == "" {
		t.Fatalf("update missing url/sha256/signature: %+v", upd)
	}
	// payload_properties must carry the four AOSP headers.
	for _, k := range []string{"FILE_HASH", "FILE_SIZE", "METADATA_HASH", "METADATA_SIZE"} {
		if _, ok := upd.PayloadProperties[k]; !ok {
			t.Fatalf("payload_properties missing %s: %+v", k, upd.PayloadProperties)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("update response must be Cache-Control: no-store")
	}
}

func TestClientUpdate204WhenOnTarget(t *testing.T) {
	env := newTestEnv(t)
	// Device already at the release version -> 204.
	dev := setupDeployment(t, env, "1.1.0", "1.1.0")

	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("on-target device want 204, got %d (%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("204 must have empty body, got %q", w.Body.String())
	}
}

func TestClientUpdate204WhenNoDeployment(t *testing.T) {
	env := newTestEnv(t)
	dev := registerDevice(t, env, DeviceRegistration{
		HardwareID: "lonely-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid, CurrentVersion: "1.0.0",
	})
	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("no deployment want 204, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestClientUpdateCurrentVersionQueryShortCircuits(t *testing.T) {
	env := newTestEnv(t)
	// Device stored at 1.0.0 but reports 1.1.0 via query -> 204.
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")
	w := env.do(http.MethodGet, "/api/v1/client/update?current_version=1.1.0", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("query short-circuit want 204, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestClientTelemetryIngest(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")

	// Determine the deployment id by reading it back is not exposed; instead use
	// a known deployment via active lookup through an update report. We need the
	// deployment id, so list active deployments through the repo directly.
	deps, _ := env.repo.ListActiveDeployments(nil)
	if len(deps) == 0 {
		t.Fatalf("expected an active deployment")
	}
	deploymentID := deps[0].DeploymentID

	report := TelemetryReport{
		DeviceID:     dev.DeviceID,
		DeploymentID: deploymentID,
		Events: []TelemetryEventWire{
			{Event: otaprotocol.EventDownloadStarted, Version: "1.1.0", Timestamp: time.Date(2026, 6, 7, 0, 15, 0, 0, time.UTC)},
			{Event: otaprotocol.EventSuccess, Version: "1.1.0", Timestamp: time.Date(2026, 6, 7, 0, 20, 0, 0, time.UTC)},
		},
	}
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report)
	if w.Code != http.StatusAccepted {
		t.Fatalf("telemetry want 202, got %d (%s)", w.Code, w.Body.String())
	}
	var ack TelemetryAck
	env.decode(w, &ack)
	if ack.Accepted != 2 || ack.Rejected != 0 {
		t.Fatalf("ack want accepted=2 rejected=0, got %+v", ack)
	}

	// The success event must update the device's current version + state.
	sw := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/status", env.adminToken(), nil, "")
	var st DeviceStatus
	env.decode(sw, &st)
	if st.UpdateState != string(otaprotocol.EventSuccess) {
		t.Fatalf("device update_state want success, got %q", st.UpdateState)
	}
	if st.CurrentVersion != "1.1.0" {
		t.Fatalf("device current_version want 1.1.0, got %q", st.CurrentVersion)
	}
}

func TestClientTelemetryWrongDeviceForbidden(t *testing.T) {
	env := newTestEnv(t)
	dev := registerDevice(t, env, DeviceRegistration{HardwareID: "tele-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})

	report := TelemetryReport{
		DeviceID:     "some-other-device",
		DeploymentID: "dep-1",
		Events: []TelemetryEventWire{
			{Event: otaprotocol.EventSuccess, Timestamp: time.Now()},
		},
	}
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report)
	if w.Code != http.StatusForbidden {
		t.Fatalf("reporting for another device want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeForbidden {
		t.Fatalf("want FORBIDDEN, got %s", got)
	}
}

func TestClientTelemetryEmptyEvents(t *testing.T) {
	env := newTestEnv(t)
	dev := registerDevice(t, env, DeviceRegistration{HardwareID: "empty-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	report := TelemetryReport{DeviceID: dev.DeviceID, DeploymentID: "dep-1", Events: []TelemetryEventWire{}}
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty events want 400, got %d", w.Code)
	}
}
