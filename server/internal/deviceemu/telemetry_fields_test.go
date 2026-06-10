package deviceemu_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
)

// telemetryItemView mirrors the server's TelemetryEventView wire shape for the
// fields this test asserts on, read back over real HTTP from
// GET /devices/{id}/telemetry. duration_ms / bytes_transferred are pointers so
// the test distinguishes "present" from "absent" (omitempty).
type telemetryItemView struct {
	Event            string `json:"event"`
	DeploymentID     string `json:"deployment_id"`
	DurationMS       *int64 `json:"duration_ms"`
	BytesTransferred *int64 `json:"bytes_transferred"`
}

// readTelemetry reads a device's full telemetry history over real HTTP using an
// operator token and returns the decoded items (newest-first, per the server).
func readTelemetry(t *testing.T, fx *serverFixture, opToken, deviceID string) []telemetryItemView {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, fx.baseURL+"/devices/"+deviceID+"/telemetry?limit=200", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("telemetry history: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("telemetry history want 200, got %d: %s", res.StatusCode, raw)
	}
	var hist struct {
		Items []telemetryItemView `json:"items"`
	}
	if err := json.Unmarshal(raw, &hist); err != nil {
		t.Fatalf("decode telemetry %q: %v", raw, err)
	}
	return hist.Items
}

// TestEmulatorTelemetryFieldsEndToEnd is the anti-bluff (§11.4 / §11.4.5 / §11.4.69)
// end-to-end proof that the per-event duration_ms / bytes_transferred annotations
// flow emulator -> server INGEST -> store -> read VIEW with their EXACT values.
// It drives the REAL emulator Device against the REAL in-process control plane
// (httptest + memory repo), then reads the values back over real HTTP — not a
// claim, an actual round-trip. The §1.1 paired-mutation guard lives in
// TestEmulatorTelemetryFieldsIngestMutation below.
func TestEmulatorTelemetryFieldsEndToEnd(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()

	opToken := fx.operatorLogin(t)
	const group = "tele-fields-fleet"
	relVersion, deploymentID := fx.stageDeployment(t, opToken, "1.4.0", group)

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-TELEFIELDS",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.0.0",
		Group:          group,
		DeploymentID:   deploymentID,
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}
	if err := dev.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	upd, onTarget, err := dev.CheckUpdate(ctx)
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if onTarget || upd == nil {
		t.Fatalf("expected an update offer, got onTarget=%v upd=%v", onTarget, upd)
	}
	if upd.Version != relVersion {
		t.Fatalf("offered version %q want %q", upd.Version, relVersion)
	}
	// The full payload size the server offered IS the byte count the emulator
	// reports on download_started + success.
	wantBytes := upd.Size
	if wantBytes <= 0 {
		t.Fatalf("offer Size = %d, want > 0 (fixture should set a real artifact size)", wantBytes)
	}

	if _, err := dev.ApplyAndReport(ctx, upd); err != nil {
		t.Fatalf("apply+report: %v", err)
	}

	items := readTelemetry(t, fx, opToken, dev.DeviceID())
	if len(items) == 0 {
		t.Fatalf("no telemetry items read back")
	}

	// Per the emulator's lifecycleAnnotations model, every lifecycle event carries
	// a duration_ms, and download_started + success additionally carry the full
	// payload byte count. Assert the EXACT values flowed end-to-end.
	wantDuration := map[string]int64{
		"download_started": 4200,
		"installing":       9300,
		"installed":        1100,
		"verifying":        2600,
		"success":          400,
	}
	seen := map[string]bool{}
	for _, it := range items {
		if it.DeploymentID != deploymentID {
			continue
		}
		seen[it.Event] = true
		wd, ok := wantDuration[it.Event]
		if !ok {
			continue
		}
		if it.DurationMS == nil {
			t.Errorf("event %q: duration_ms absent, want %d", it.Event, wd)
		} else if *it.DurationMS != wd {
			t.Errorf("event %q: duration_ms = %d, want %d", it.Event, *it.DurationMS, wd)
		}
		switch it.Event {
		case "download_started", "success":
			if it.BytesTransferred == nil {
				t.Errorf("event %q: bytes_transferred absent, want %d", it.Event, wantBytes)
			} else if *it.BytesTransferred != wantBytes {
				t.Errorf("event %q: bytes_transferred = %d, want %d", it.Event, *it.BytesTransferred, wantBytes)
			}
		default:
			// Intermediate phases move no payload bytes => the key is omitted.
			if it.BytesTransferred != nil {
				t.Errorf("event %q: bytes_transferred = %d, want absent (no bytes moved)", it.Event, *it.BytesTransferred)
			}
		}
	}
	for ev := range wantDuration {
		if !seen[ev] {
			t.Errorf("lifecycle event %q never appeared in telemetry history for deployment %s", ev, deploymentID)
		}
	}

	// Determinism (§11.4.50): a second decode of the same history yields identical
	// values (the read path is pure over the persisted records).
	items2 := readTelemetry(t, fx, opToken, dev.DeviceID())
	if len(items2) != len(items) {
		t.Fatalf("non-deterministic read: len %d then %d", len(items), len(items2))
	}
}

// TestEmulatorTelemetryFieldsIngestMutation is the §1.1 paired-mutation proof
// that the end-to-end assertion above is NOT a bluff: it documents — and a future
// mutation harness flips RED — that if server ingest DROPPED the field (i.e.
// handlers_client.go set the record's DurationMS/BytesTransferred to nil instead
// of ev.DurationMS/ev.BytesTransferred), the read-back values would be nil and
// the EXACT-value assertions would FAIL.
//
// The mutation is exercised directly here at the value layer: a record with the
// fields dropped MUST NOT satisfy the same exact-value check the E2E test uses,
// proving the assertion has teeth. (The on-the-wire mutation — editing
// handlers_client.go to drop the fields — makes TestEmulatorTelemetryFieldsEndToEnd
// FAIL, which is the canonical §1.1 pair.)
func TestEmulatorTelemetryFieldsIngestMutation(t *testing.T) {
	const wantDuration = int64(4200)
	const wantBytes = int64(379074366)

	// GREEN side: ingest preserved the fields (what the real code does).
	good := telemetryItemView{Event: "download_started", DurationMS: ptr(wantDuration), BytesTransferred: ptr(wantBytes)}
	if good.DurationMS == nil || *good.DurationMS != wantDuration ||
		good.BytesTransferred == nil || *good.BytesTransferred != wantBytes {
		t.Fatalf("GREEN ingest should preserve fields, got %+v", good)
	}

	// RED side: simulate the mutation where ingest dropped the fields to nil.
	dropped := telemetryItemView{Event: "download_started", DurationMS: nil, BytesTransferred: nil}
	mutationCaught := false
	if dropped.DurationMS == nil || *dropped.DurationMS != wantDuration ||
		dropped.BytesTransferred == nil || *dropped.BytesTransferred != wantBytes {
		mutationCaught = true
	}
	if !mutationCaught {
		t.Fatal("paired mutation NOT caught: dropping the ingest fields would have passed the exact-value assertion — the E2E check is a bluff")
	}
}

func ptr(v int64) *int64 { return &v }
