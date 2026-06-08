package api

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestAuditSinceUntilFilter exercises the additive ?since/?until time bounds on
// GET /audit (operational_endpoints.md §4.3).
func TestAuditSinceUntilFilter(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	ctx := context.Background()
	base := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	for i, act := range []string{"ACT_T1", "ACT_T2", "ACT_T3"} {
		if err := env.repo.AppendAudit(ctx, store.AuditEntry{
			ID: act, Action: act, ResourceType: "device", CreatedAt: base.Add(time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("AppendAudit %s: %v", act, err)
		}
	}

	// since = base+1h excludes ACT_T1.
	since := url.QueryEscape(base.Add(time.Hour).Format(time.RFC3339))
	w := env.do(http.MethodGet, "/api/v1/audit?since="+since, tok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("audit since want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var list AuditLogList
	env.decode(w, &list)
	got := map[string]bool{}
	for _, it := range list.Items {
		got[it.Action] = true
	}
	if got["ACT_T1"] || !got["ACT_T2"] || !got["ACT_T3"] {
		t.Fatalf("since filter mismatch: %+v", got)
	}

	// until = base+1h excludes ACT_T3.
	until := url.QueryEscape(base.Add(time.Hour).Format(time.RFC3339))
	w2 := env.do(http.MethodGet, "/api/v1/audit?until="+until, tok, nil, "")
	var list2 AuditLogList
	env.decode(w2, &list2)
	got2 := map[string]bool{}
	for _, it := range list2.Items {
		got2[it.Action] = true
	}
	if !got2["ACT_T1"] || !got2["ACT_T2"] || got2["ACT_T3"] {
		t.Fatalf("until filter mismatch: %+v", got2)
	}

	// Malformed timestamp -> 400.
	if bad := env.do(http.MethodGet, "/api/v1/audit?since=notatime", tok, nil, ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("bad since want 400, got %d", bad.Code)
	}
}

// TestTelemetryOverviewFailureRateAndByState exercises the additive
// failure_rate + by_state fields on GET /telemetry/overview.
func TestTelemetryOverviewFailureRateAndByState(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	ctx := context.Background()
	now := env.srv.now()
	if err := env.repo.CreateDevice(ctx, store.Device{DeviceID: "od-1", HardwareID: "H1", Model: "OrangePi5Max",
		OSType: otaprotocol.OSAndroid, UpdateState: "installed", RegisteredAt: now}); err != nil {
		t.Fatalf("CreateDevice od-1: %v", err)
	}
	if err := env.repo.CreateDevice(ctx, store.Device{DeviceID: "od-2", HardwareID: "H2", Model: "OrangePi5Max",
		OSType: otaprotocol.OSAndroid, UpdateState: "failure", RegisteredAt: now}); err != nil {
		t.Fatalf("CreateDevice od-2: %v", err)
	}
	// 3 success + 1 failure => failure_rate = 1/4 = 0.25.
	for i := 0; i < 3; i++ {
		_ = env.repo.AppendTelemetry(ctx, store.TelemetryRecord{DeviceID: "od-1",
			Event: otaprotocol.EventSuccess, Timestamp: now, ReceivedAt: now})
	}
	_ = env.repo.AppendTelemetry(ctx, store.TelemetryRecord{DeviceID: "od-2",
		Event: otaprotocol.EventFailure, Timestamp: now, ReceivedAt: now})

	w := env.do(http.MethodGet, "/api/v1/telemetry/overview", tok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("overview want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var ov TelemetryOverview
	env.decode(w, &ov)
	if ov.FailureRate != 0.25 {
		t.Fatalf("failure_rate want 0.25, got %v (counts=%+v)", ov.FailureRate, ov.EventCounts)
	}
	if ov.ByState["installed"] < 1 || ov.ByState["failure"] < 1 {
		t.Fatalf("by_state mismatch: %+v", ov.ByState)
	}
}
