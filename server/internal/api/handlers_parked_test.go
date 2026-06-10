package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// §11.4.43 RED-first: these tests exercise the two parked-but-software-actionable
// items from the CONTINUATION NEXT wave — per-device telemetry filters and
// group-membership / group-list pagination. Both are handler-layer only (no
// store.Repository change → no memory/pgx parity surface, §11.4.92 Pass 2).

// seedThreeTelemetryEvents registers a device+deployment and ingests three events
// at distinct event types and timestamps so filters have something to bite on.
func seedThreeTelemetryEvents(t *testing.T, env *testEnv) DeviceRegistered {
	t.Helper()
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")
	deps, _ := env.repo.ListActiveDeployments(nil)
	report := TelemetryReport{
		DeviceID:     dev.DeviceID,
		DeploymentID: deps[0].DeploymentID,
		Events: []TelemetryEventWire{
			{Event: otaprotocol.EventDownloadStarted, Version: "1.1.0", Timestamp: time.Date(2026, 6, 8, 0, 1, 0, 0, time.UTC)},
			{Event: otaprotocol.EventInstalling, Version: "1.1.0", Timestamp: time.Date(2026, 6, 8, 0, 2, 0, 0, time.UTC)},
			{Event: otaprotocol.EventFailure, Version: "1.1.0", Timestamp: time.Date(2026, 6, 8, 0, 3, 0, 0, time.UTC)},
		},
	}
	if w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report); w.Code != http.StatusAccepted {
		t.Fatalf("telemetry ingest want 202, got %d (%s)", w.Code, w.Body.String())
	}
	return dev
}

func TestDeviceTelemetryFilters(t *testing.T) {
	env := newTestEnv(t)
	dev := seedThreeTelemetryEvents(t, env)
	tok := env.adminToken()

	get := func(q string) TelemetryHistory {
		t.Helper()
		w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry"+q, tok, nil, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET telemetry%s want 200, got %d (%s)", q, w.Code, w.Body.String())
		}
		var h TelemetryHistory
		env.decode(w, &h)
		return h
	}

	// event filter: only the failure event.
	if h := get("?event=failure"); len(h.Items) != 1 || h.Items[0].Event != otaprotocol.EventFailure {
		t.Fatalf("event filter mismatch: %+v", h.Items)
	}
	// since (inclusive) -> installing + failure, newest-first.
	if h := get("?since=2026-06-08T00:02:00Z"); len(h.Items) != 2 ||
		h.Items[0].Event != otaprotocol.EventFailure || h.Items[1].Event != otaprotocol.EventInstalling {
		t.Fatalf("since filter mismatch: %+v", h.Items)
	}
	// until (inclusive) -> download_started only.
	if h := get("?until=2026-06-08T00:01:00Z"); len(h.Items) != 1 ||
		h.Items[0].Event != otaprotocol.EventDownloadStarted {
		t.Fatalf("until filter mismatch: %+v", h.Items)
	}
	// combined event + time window narrows to the single installing event.
	if h := get("?event=installing&since=2026-06-08T00:01:30Z&until=2026-06-08T00:02:30Z"); len(h.Items) != 1 ||
		h.Items[0].Event != otaprotocol.EventInstalling {
		t.Fatalf("combined filter mismatch: %+v", h.Items)
	}
	// filters compose with pagination: event filter is applied BEFORE the limit.
	if h := get("?since=2026-06-08T00:02:00Z&limit=1"); len(h.Items) != 1 ||
		h.Items[0].Event != otaprotocol.EventFailure || h.NextCursor == nil {
		t.Fatalf("filter+limit page1 mismatch: %+v", h)
	}

	// malformed since -> 400 VALIDATION_FAILED.
	if w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry?since=not-a-time", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed since want 400, got %d", w.Code)
	}
	// unknown event value -> 400 (closed set, §11.4.6 no-guessing).
	if w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/telemetry?event=bogus", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bogus event want 400, got %d", w.Code)
	}
}

func TestGroupMembersPagination(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	cw := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "fleet-page"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create group want 201, got %d", cw.Code)
	}
	var g GroupView
	env.decode(cw, &g)

	ids := []string{"pd-1", "pd-2", "pd-3"}
	for _, id := range ids {
		if err := env.repo.CreateDevice(context.Background(), store.Device{DeviceID: id,
			HardwareID: "HW-" + id, Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid, RegisteredAt: env.srv.now()}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	if w := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.GroupID+"/members", tok, MemberAdd{DeviceIDs: ids}); w.Code != http.StatusOK {
		t.Fatalf("add members want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// page 1: limit=2 -> 2 items + a next_cursor.
	w1 := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID+"/members?limit=2", tok, nil, "")
	var p1 GroupMembers
	env.decode(w1, &p1)
	if len(p1.Items) != 2 || p1.NextCursor == nil {
		t.Fatalf("members page1 mismatch: %+v", p1)
	}
	// page 2: the remaining 1, nil cursor.
	w2 := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID+"/members?limit=2&cursor="+*p1.NextCursor, tok, nil, "")
	var p2 GroupMembers
	env.decode(w2, &p2)
	if len(p2.Items) != 1 || p2.NextCursor != nil {
		t.Fatalf("members page2 mismatch: %+v", p2)
	}
	// no overlap, no gap: the union of both pages is exactly the 3 unique ids.
	seen := map[string]bool{}
	for _, it := range append(append([]GroupMemberView{}, p1.Items...), p2.Items...) {
		if seen[it.DeviceID] {
			t.Fatalf("paginated members duplicated %s", it.DeviceID)
		}
		seen[it.DeviceID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("paginated members dropped %s (seen=%v)", id, seen)
		}
	}
	// bad limit -> 400.
	if w := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID+"/members?limit=0", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("bad members limit want 400, got %d", w.Code)
	}
}

func TestListGroupsPagination(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	for _, n := range []string{"g-a", "g-b", "g-c"} {
		if w := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: n}); w.Code != http.StatusCreated {
			t.Fatalf("create %s want 201, got %d", n, w.Code)
		}
	}
	w1 := env.do(http.MethodGet, "/api/v1/groups?limit=2", tok, nil, "")
	var p1 GroupList
	env.decode(w1, &p1)
	if len(p1.Items) != 2 || p1.NextCursor == nil {
		t.Fatalf("groups page1 mismatch: %+v", p1)
	}
	w2 := env.do(http.MethodGet, "/api/v1/groups?limit=2&cursor="+*p1.NextCursor, tok, nil, "")
	var p2 GroupList
	env.decode(w2, &p2)
	if len(p2.Items) != 1 || p2.NextCursor != nil {
		t.Fatalf("groups page2 mismatch: %+v", p2)
	}
	// bad cursor is tolerated (decodes to start), bad limit is a 400.
	if w := env.do(http.MethodGet, "/api/v1/groups?limit=999", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("over-range groups limit want 400, got %d", w.Code)
	}
}
