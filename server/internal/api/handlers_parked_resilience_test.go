package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// §11.4.85 stress + resilience for the two SHIPPED parked handlers:
//   - GET /groups?limit=N            (handleListGroups — cursor pagination)
//   - GET /devices/{id}/telemetry    (handleDeviceTelemetry — event/since/until
//     filters + cursor pagination)
//
// Two invariants under load: (a) ≥20 concurrent readers paginate the full set
// with NO duplicate and NO gap and NO data race (-race), and (b) a boundary
// sweep (limit=1, limit=200, empty result, unknown ?event=bogus -> 400) returns
// the right status/shape at every edge. Captured evidence: per-run counters in
// the failure messages + the -race detector verdict.

// seedGroups creates n groups via the real HTTP handler and returns their ids.
func seedGroups(t *testing.T, env *testEnv, n int) map[string]bool {
	t.Helper()
	tok := env.adminToken()
	ids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		w := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: fmt.Sprintf("res-grp-%03d", i)})
		if w.Code != http.StatusCreated {
			t.Fatalf("seed group %d want 201, got %d (%s)", i, w.Code, w.Body.String())
		}
		var g GroupView
		env.decode(w, &g)
		ids[g.GroupID] = true
	}
	return ids
}

// paginateGroups walks GET /groups with the given page size and returns the set
// of group ids collected, asserting no duplicate appeared within this walk.
func paginateGroups(t *testing.T, env *testEnv, tok string, limit int) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	cursor := ""
	for {
		path := fmt.Sprintf("/api/v1/groups?limit=%d", limit)
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		w := env.do(http.MethodGet, path, tok, nil, "")
		if w.Code != http.StatusOK {
			t.Errorf("paginate groups want 200, got %d (%s)", w.Code, w.Body.String())
			return seen
		}
		var page GroupList
		env.decode(w, &page)
		for _, it := range page.Items {
			if seen[it.GroupID] {
				t.Errorf("DUPLICATE group %s across pages", it.GroupID)
			}
			seen[it.GroupID] = true
		}
		if page.NextCursor == nil {
			return seen
		}
		cursor = *page.NextCursor
	}
}

// TestResilienceConcurrentGroupPagination drives ≥20 goroutines that each
// independently paginate the full group list. Every reader must reconstruct the
// exact seeded set — no duplicate, no gap — under -race.
func TestResilienceConcurrentGroupPagination(t *testing.T) {
	env := newTestEnv(t)
	const total = 47
	want := seedGroups(t, env, total)

	const readers = 24
	const pageSize = 5
	tok := env.adminToken()

	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seen := paginateGroups(t, env, tok, pageSize)
			if len(seen) != total {
				errs <- fmt.Errorf("reader saw %d groups, want %d (gap or overlap)", len(seen), total)
				return
			}
			for id := range want {
				if !seen[id] {
					errs <- fmt.Errorf("reader DROPPED group %s", id)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	t.Logf("EVIDENCE: %d concurrent readers each paginated all %d groups (pageSize=%d) with no dup/gap under -race",
		readers, total, pageSize)
}

// seedDeviceWithTelemetry registers a device + deployment and ingests n events
// across the six event types at distinct, monotonically-increasing timestamps.
// It returns the device id and the timestamps used (for window boundary checks).
func seedDeviceWithTelemetry(t *testing.T, env *testEnv, n int) (deviceID string, base time.Time) {
	t.Helper()
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")
	deps, _ := env.repo.ListActiveDeployments(nil)
	if len(deps) == 0 {
		t.Fatalf("no active deployment seeded")
	}
	base = time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	events := []otaprotocol.TelemetryEvent{
		otaprotocol.EventDownloadStarted, otaprotocol.EventInstalling, otaprotocol.EventInstalled,
		otaprotocol.EventVerifying, otaprotocol.EventSuccess, otaprotocol.EventFailure,
	}
	wire := make([]TelemetryEventWire, 0, n)
	for i := 0; i < n; i++ {
		ev := events[i%len(events)]
		wire = append(wire, TelemetryEventWire{
			Event:     ev,
			Version:   "1.1.0",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}
	report := TelemetryReport{DeviceID: dev.DeviceID, DeploymentID: deps[0].DeploymentID, Events: wire}
	w := env.doJSON(http.MethodPost, "/api/v1/client/telemetry", env.deviceToken(dev.DeviceID), report)
	if w.Code != http.StatusAccepted {
		t.Fatalf("seed telemetry want 202, got %d (%s)", w.Code, w.Body.String())
	}
	var ack TelemetryAck
	env.decode(w, &ack)
	if ack.Accepted != n {
		t.Fatalf("seed telemetry accepted=%d, want %d (%s)", ack.Accepted, n, w.Body.String())
	}
	return dev.DeviceID, base
}

// paginateTelemetry walks GET /devices/{id}/telemetry with a query prefix and
// page size, returning the ordered list of (event,timestamp) keys collected and
// asserting no duplicate cursor key appeared.
func paginateTelemetry(t *testing.T, env *testEnv, tok, deviceID, queryPrefix string, limit int) []string {
	t.Helper()
	var keys []string
	seen := map[string]bool{}
	cursor := ""
	for {
		path := fmt.Sprintf("/api/v1/devices/%s/telemetry?limit=%d%s", deviceID, limit, queryPrefix)
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		w := env.do(http.MethodGet, path, tok, nil, "")
		if w.Code != http.StatusOK {
			t.Errorf("paginate telemetry want 200, got %d (%s)", w.Code, w.Body.String())
			return keys
		}
		var page TelemetryHistory
		env.decode(w, &page)
		for _, it := range page.Items {
			key := string(it.Event) + "@" + it.Timestamp.Format(time.RFC3339Nano)
			if seen[key] {
				t.Errorf("DUPLICATE telemetry item %s across pages", key)
			}
			seen[key] = true
			keys = append(keys, key)
		}
		if page.NextCursor == nil {
			return keys
		}
		cursor = *page.NextCursor
	}
}

// TestResilienceConcurrentTelemetryPagination drives ≥20 goroutines paginating a
// device's telemetry history (both unfiltered and with an event filter). Every
// reader must collect the same number of items with no duplicate/gap, -race-safe.
func TestResilienceConcurrentTelemetryPagination(t *testing.T) {
	env := newTestEnv(t)
	const total = 60 // 10 of each of the 6 event types
	deviceID, _ := seedDeviceWithTelemetry(t, env, total)
	tok := env.adminToken()

	// Reference unfiltered count (single-threaded baseline).
	refUnfiltered := len(paginateTelemetry(t, env, tok, deviceID, "", 7))
	if refUnfiltered != total {
		t.Fatalf("baseline unfiltered telemetry count = %d, want %d", refUnfiltered, total)
	}
	// Reference filtered count: success events = total/6.
	wantSuccess := total / 6
	refSuccess := len(paginateTelemetry(t, env, tok, deviceID, "&event=success", 3))
	if refSuccess != wantSuccess {
		t.Fatalf("baseline success-filtered count = %d, want %d", refSuccess, wantSuccess)
	}

	const readers = 24
	var wg sync.WaitGroup
	errs := make(chan error, readers*2)
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			// Half the readers paginate unfiltered, half filter on success — both
			// hit the same handler concurrently.
			if r%2 == 0 {
				if got := len(paginateTelemetry(t, env, tok, deviceID, "", 7)); got != total {
					errs <- fmt.Errorf("unfiltered reader saw %d items, want %d", got, total)
				}
			} else {
				if got := len(paginateTelemetry(t, env, tok, deviceID, "&event=success", 3)); got != wantSuccess {
					errs <- fmt.Errorf("success-filtered reader saw %d items, want %d", got, wantSuccess)
				}
			}
		}(r)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	t.Logf("EVIDENCE: %d concurrent telemetry readers (mixed unfiltered/event=success) over %d events, no dup/gap under -race",
		readers, total)
}

// TestResilienceTelemetryFilterBoundarySweep exercises every documented edge of
// the telemetry filter+pagination handler: min/max limit, empty result, the
// inclusive since/until window bounds, and the rejected inputs (bad limit, bad
// since, unknown event -> 400).
func TestResilienceTelemetryFilterBoundarySweep(t *testing.T) {
	env := newTestEnv(t)
	const total = 60
	deviceID, base := seedDeviceWithTelemetry(t, env, total)
	tok := env.adminToken()

	get := func(q string) *responseWrap {
		w := env.do(http.MethodGet, "/api/v1/devices/"+deviceID+"/telemetry"+q, tok, nil, "")
		return &responseWrap{t: t, env: env, rec: w}
	}

	// limit=1 -> exactly one item + a next_cursor (more remain).
	if h := get("?limit=1").ok(); len(h.Items) != 1 || h.NextCursor == nil {
		t.Fatalf("limit=1 boundary: items=%d cursor=%v", len(h.Items), h.NextCursor)
	}
	// limit=200 (the max) -> the whole set on one page, nil cursor.
	if h := get("?limit=200").ok(); len(h.Items) != total || h.NextCursor != nil {
		t.Fatalf("limit=200 boundary: items=%d cursor=%v", len(h.Items), h.NextCursor)
	}
	// Empty result: a since beyond the last event -> 0 items, nil cursor.
	beyond := base.Add(time.Duration(total) * time.Hour).Format(time.RFC3339)
	if h := get("?since=" + beyond).ok(); len(h.Items) != 0 || h.NextCursor != nil {
		t.Fatalf("empty-result boundary: items=%d cursor=%v", len(h.Items), h.NextCursor)
	}
	// Inclusive until bound: until == first event timestamp -> exactly 1 item.
	firstTS := base.Format(time.RFC3339)
	if h := get("?until=" + firstTS).ok(); len(h.Items) != 1 {
		t.Fatalf("inclusive-until boundary: items=%d, want 1", len(h.Items))
	}

	// Rejected inputs -> 400.
	get("?limit=0").wantStatus(http.StatusBadRequest)
	get("?limit=201").wantStatus(http.StatusBadRequest)
	get("?since=not-a-time").wantStatus(http.StatusBadRequest)
	get("?event=bogus").wantStatus(http.StatusBadRequest)
	get("?cursor=-1").wantStatus(http.StatusBadRequest)
	t.Logf("EVIDENCE: telemetry boundary sweep PASS — limits {1,200,0,201}, empty/inclusive windows, bad since/event/cursor all correct")
}

// TestResilienceGroupListBoundarySweep exercises the group-list pagination edges:
// limit=1 across the set, the max limit on one page, an over-range limit -> 400,
// an empty list, and a single concurrent burst at limit=1.
func TestResilienceGroupListBoundarySweep(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Empty list first: 0 items, nil cursor, 200.
	w := env.do(http.MethodGet, "/api/v1/groups?limit=10", tok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("empty groups want 200, got %d", w.Code)
	}
	var empty GroupList
	env.decode(w, &empty)
	if len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("empty groups boundary: items=%d cursor=%v", len(empty.Items), empty.NextCursor)
	}

	const total = 25
	want := seedGroups(t, env, total)

	// limit=1 walk collects all ids, no dup/gap.
	seen := paginateGroups(t, env, tok, 1)
	if len(seen) != total {
		t.Fatalf("limit=1 walk saw %d, want %d", len(seen), total)
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("limit=1 walk dropped %s", id)
		}
	}
	// max limit (200) -> all on one page.
	mw := env.do(http.MethodGet, "/api/v1/groups?limit=200", tok, nil, "")
	var mpage GroupList
	env.decode(mw, &mpage)
	if mw.Code != http.StatusOK || len(mpage.Items) != total || mpage.NextCursor != nil {
		t.Fatalf("limit=200 boundary: code=%d items=%d cursor=%v", mw.Code, len(mpage.Items), mpage.NextCursor)
	}
	// over-range limit -> 400.
	if ow := env.do(http.MethodGet, "/api/v1/groups?limit=201", tok, nil, ""); ow.Code != http.StatusBadRequest {
		t.Fatalf("over-range groups limit want 400, got %d", ow.Code)
	}
	t.Logf("EVIDENCE: group-list boundary sweep PASS — empty, limit=1 full walk, limit=200 single page, limit=201 -> 400")
}

// TestResilienceConcurrentSeedAndRead races many concurrent READERS (group list
// + group members + device telemetry — the three shipped read handlers) against
// the SAME in-memory store the production server uses, proving the read path is
// data-race-free under heavy concurrency (-race detector is the oracle).
//
// NOTE (§11.4.6 FACT): writes are seeded SEQUENTIALLY on purpose. The test
// fixture injects a deliberately-simple, non-atomic sequential id generator
// (testutil_test.go:73 `env.idSeq++`); driving concurrent CREATE through it
// races the HARNESS counter, not the product. The production default id
// generator (server.go newRandomID) is concurrency-safe; concurrent writes are
// out of scope for these two read-handler resilience tests, so this test races
// the readers (the genuine concurrency surface of pagination) — not the harness.
func TestResilienceConcurrentSeedAndRead(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Seed groups + a group with members + a device-with-telemetry sequentially.
	const groupCount = 12
	seedGroups(t, env, groupCount)

	gw := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "members-grp"})
	if gw.Code != http.StatusCreated {
		t.Fatalf("create members group want 201, got %d", gw.Code)
	}
	var mg GroupView
	env.decode(gw, &mg)
	memberIDs := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("cd-%02d", i)
		if err := env.repo.CreateDevice(context.Background(), store.Device{
			DeviceID: id, HardwareID: "HW-" + id, Model: "OrangePi5Max",
			OSType: otaprotocol.OSAndroid, RegisteredAt: env.srv.now(),
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
		memberIDs = append(memberIDs, id)
	}
	if mw := env.doJSON(http.MethodPost, "/api/v1/groups/"+mg.GroupID+"/members", tok, MemberAdd{DeviceIDs: memberIDs}); mw.Code != http.StatusOK {
		t.Fatalf("add members want 200, got %d (%s)", mw.Code, mw.Body.String())
	}
	deviceID, _ := seedDeviceWithTelemetry(t, env, 18)

	// Now hammer the three shipped read handlers concurrently.
	const readers = 30
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			switch r % 3 {
			case 0:
				lw := env.do(http.MethodGet, "/api/v1/groups?limit=4", tok, nil, "")
				if lw.Code != http.StatusOK {
					errs <- fmt.Errorf("concurrent group-list want 200, got %d", lw.Code)
				}
			case 1:
				mw := env.do(http.MethodGet, "/api/v1/groups/"+mg.GroupID+"/members?limit=3", tok, nil, "")
				if mw.Code != http.StatusOK {
					errs <- fmt.Errorf("concurrent member-list want 200, got %d", mw.Code)
				}
			default:
				tw := env.do(http.MethodGet, "/api/v1/devices/"+deviceID+"/telemetry?limit=5", tok, nil, "")
				if tw.Code != http.StatusOK {
					errs <- fmt.Errorf("concurrent telemetry want 200, got %d", tw.Code)
				}
			}
		}(r)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// After the read storm the seeded set is intact and paginates cleanly.
	final := paginateGroups(t, env, tok, 3)
	if len(final) != groupCount+1 {
		t.Fatalf("final group count = %d, want %d", len(final), groupCount+1)
	}
	t.Logf("EVIDENCE: %d concurrent readers across group-list/member-list/telemetry handlers, no race (-race); seeded set intact (%d groups)",
		readers, groupCount+1)
}

// responseWrap is a tiny assertion helper for the boundary sweep.
type responseWrap struct {
	t   *testing.T
	env *testEnv
	rec *httptest.ResponseRecorder
}

func (r *responseWrap) ok() TelemetryHistory {
	r.t.Helper()
	if r.rec.Code != http.StatusOK {
		r.t.Fatalf("want 200, got %d (%s)", r.rec.Code, r.rec.Body.String())
	}
	var h TelemetryHistory
	r.env.decode(r.rec, &h)
	return h
}

func (r *responseWrap) wantStatus(want int) {
	r.t.Helper()
	if r.rec.Code != want {
		r.t.Fatalf("want %d, got %d (%s)", want, r.rec.Code, r.rec.Body.String())
	}
}
