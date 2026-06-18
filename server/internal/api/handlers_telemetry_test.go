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
	if hist.DeviceID != dev.DeviceID || len(hist.Items) != 2 {
		t.Fatalf("history mismatch: %+v", hist)
	}
	// Newest-first: success was reported after download_started.
	if hist.Items[0].Event != otaprotocol.EventSuccess || hist.Items[1].Event != otaprotocol.EventDownloadStarted {
		t.Fatalf("history order (newest-first) mismatch: %+v", hist.Items)
	}
	if hist.NextCursor != nil {
		t.Fatalf("single page should have nil next_cursor, got %v", *hist.NextCursor)
	}

	// Pagination: limit=1 returns the newest + a next_cursor to the second.
	pw := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry?limit=1", env.adminToken(), nil, "")
	var page1 TelemetryHistory
	env.decode(pw, &page1)
	if len(page1.Items) != 1 || page1.Items[0].Event != otaprotocol.EventSuccess || page1.NextCursor == nil {
		t.Fatalf("page1 mismatch: %+v", page1)
	}
	p2 := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry?limit=1&cursor="+*page1.NextCursor, env.adminToken(), nil, "")
	var page2 TelemetryHistory
	env.decode(p2, &page2)
	if len(page2.Items) != 1 || page2.Items[0].Event != otaprotocol.EventDownloadStarted || page2.NextCursor != nil {
		t.Fatalf("page2 mismatch: %+v", page2)
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
