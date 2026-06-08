package api

import (
	"net/http"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// seedTelemetry registers a device + deployment and ingests two events for it.
func seedTelemetry(t *testing.T, env *testEnv) DeviceRegistered {
	t.Helper()
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")
	deps, _ := env.repo.ListActiveDeployments(nil)
	report := TelemetryReport{
		DeviceID:     dev.DeviceID,
		DeploymentID: deps[0].DeploymentID,
		Events: []TelemetryEventWire{
			{Event: otaprotocol.EventDownloadStarted, Version: "1.1.0", Timestamp: time.Date(2026, 6, 8, 0, 1, 0, 0, time.UTC)},
			{Event: otaprotocol.EventSuccess, Version: "1.1.0", Timestamp: time.Date(2026, 6, 8, 0, 2, 0, 0, time.UTC)},
		},
	}
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report)
	if w.Code != http.StatusAccepted {
		t.Fatalf("telemetry ingest want 202, got %d (%s)", w.Code, w.Body.String())
	}
	return dev
}

func TestDeviceTelemetryHistory(t *testing.T) {
	env := newTestEnv(t)
	dev := seedTelemetry(t, env)

	w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("device telemetry want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var hist TelemetryHistory
	env.decode(w, &hist)
	if hist.DeviceID != dev.DeviceID || len(hist.Events) != 2 {
		t.Fatalf("history mismatch: %+v", hist)
	}
	if hist.Events[0].Event != otaprotocol.EventDownloadStarted || hist.Events[1].Event != otaprotocol.EventSuccess {
		t.Fatalf("history order/content mismatch: %+v", hist.Events)
	}
}

func TestDeviceTelemetryOwnDeviceAllowedOtherForbidden(t *testing.T) {
	env := newTestEnv(t)
	dev := seedTelemetry(t, env)

	// The device reads its OWN telemetry -> 200.
	w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("device reading own telemetry want 200, got %d", w.Code)
	}
	// A device reading ANOTHER device's telemetry -> 403.
	w2 := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry", env.deviceToken("some-other-device"), nil, "")
	if w2.Code != http.StatusForbidden {
		t.Fatalf("device reading another's telemetry want 403, got %d", w2.Code)
	}
}

func TestTelemetryOverview(t *testing.T) {
	env := newTestEnv(t)
	seedTelemetry(t, env)

	viewerTok, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	w := env.do(http.MethodGet, "/api/v1/telemetry/overview", viewerTok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("overview want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var ov TelemetryOverview
	env.decode(w, &ov)
	if ov.Total != 2 || ov.EventCounts[string(otaprotocol.EventSuccess)] != 1 ||
		ov.EventCounts[string(otaprotocol.EventDownloadStarted)] != 1 {
		t.Fatalf("overview counts mismatch: %+v", ov)
	}
}
