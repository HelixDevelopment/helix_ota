// Package device -- wireformat_telemetry_test.go: proves the telemetry report
// body the real ApplyPortClient sends is byte-for-byte decodable by the REAL
// server-side handler (internal/api), not merely by a permissive test
// double.
//
// Forensic root cause (§11.4.108 SOURCE<->RUNTIME contract): the server's
// TelemetryEventWire wire struct (internal/api/wire.go) declares
// `Timestamp time.Time `json:"timestamp"“ -- encoding/json's default
// time.Time codec requires a QUOTED RFC 3339 string on the wire. Before the
// fix accompanying this test, internal/device/client.go's TelemetryEvent
// declared `Timestamp int64 `json:"timestamp"“ and the real caller
// (cmd/applyport/main.go:reportEvent) populated it with
// `time.Now().Unix()`, so the wire body carries a bare JSON NUMBER for
// "timestamp". Decoding a bare number into a time.Time field is always a
// decode error in encoding/json, so internal/api's bindJSON
// (internal/api/handlers_client.go:handleClientTelemetry) rejects the
// request with "malformed telemetry report body" on EVERY SINGLE real
// telemetry report -- meaning the whole update lifecycle (download_started
// / installing / installed / verifying / success / failure) never reaches
// the server for the real applyport CLI. cmd/applyport/main.go's
// reportEvent() does not propagate this failure (it only logs it), so the
// defect is silent: the device believes it reported telemetry, the operator
// sees nothing on the server side, and Deployment/Device dashboards stay
// stale. The existing TestApplyPortClient_ReportTelemetry(_WithHealth) never
// caught this because their httptest fixture decodes the captured body back
// into the CLIENT's OWN TelemetryReport struct (a self-referential round
// trip), never the server's real wire struct. This test decodes the exact
// bytes ApplyPortClient.ReportTelemetry() puts on the wire using the REAL
// server-side wire struct (internal/api.TelemetryReport) and the same
// strict decode discipline internal/api/bind.go uses, closing that gap.
package device

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
)

// TestApplyPortClient_ReportTelemetry_WireBodyDecodesOnRealServerStruct
// captures the exact JSON bytes ApplyPortClient.ReportTelemetry() sends --
// built exactly the way cmd/applyport/main.go:reportEvent constructs it,
// including the real Unix-seconds Timestamp value -- and decodes them with
// the REAL server-side wire type (internal/api.TelemetryReport) under the
// server's own strict-decode discipline.
func TestApplyPortClient_ReportTelemetry_WireBodyDecodesOnRealServerStruct(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":1,"rejected":0,"request_id":"req-001"}`))
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	report := TelemetryReport{
		DeviceID:     "dev-001",
		DeploymentID: "dep-001",
		Events: []TelemetryEvent{
			// Mirrors cmd/applyport/main.go:reportEvent exactly:
			// Timestamp: time.Now().Unix().
			{Event: "download_started", Version: "2.0.0", Timestamp: time.Now().Unix()},
		},
	}
	if _, err := client.ReportTelemetry(context.Background(), report); err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if len(capturedBody) == 0 {
		t.Fatal("test bug: server did not capture a request body")
	}

	// Decode with the REAL server wire struct under the REAL server's strict
	// decode discipline (DisallowUnknownFields) -- exactly what
	// internal/api/bind.go:bindJSON does before handleClientTelemetry
	// processes the events.
	var wire api.TelemetryReport
	if err := strictDecode(capturedBody, &wire); err != nil {
		t.Fatalf("the real server's bindJSON would reject this telemetry report body: %v "+
			"(body: %s). This proves a live applyport CLI telemetry report against the "+
			"real Helix OTA server is rejected on every call -- the whole update "+
			"lifecycle (download_started/installing/installed/verifying/success/failure) "+
			"never reaches the server.", err, string(capturedBody))
	}
	if len(wire.Events) != 1 {
		t.Fatalf("server-side wire struct has %d events, want 1", len(wire.Events))
	}
	if wire.Events[0].Timestamp.IsZero() {
		t.Fatal("server-side wire struct Timestamp is zero -- the timestamp the " +
			"client sent never reached the field the server persists (store.TelemetryRecord.Timestamp)")
	}
}
