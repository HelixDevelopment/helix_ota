package api

import (
	"net/http"
	"testing"
)

// These tests pin the anti-downgrade invariant of the device update-check
// (additions_synthesis §8 G1 / addition-#3 R16): the control plane MUST NEVER
// offer a device a release whose version is older than or equal to the device's
// known current version. The property is emergent from handleClientUpdate's
// `CompareDotted(current, release) >= 0 -> 204` branch; these regression tests
// make it explicit so a future refactor cannot silently weaken it.

// TestClientUpdateNeverOffersDowngrade: a device strictly AHEAD of the deployed
// release is never offered that (older) release — the check returns 204.
func TestClientUpdateNeverOffersDowngrade(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "2.0.0", "1.1.0") // device ahead of release
	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("device ahead of release must NOT be offered a downgrade; want 204, got %d (%s)",
			w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("204 must have an empty body, got %q", w.Body.String())
	}
}

// TestClientUpdateQueryReportedAheadShortCircuits: a device-reported
// current_version strictly ahead of the release also yields 204 — no downgrade
// can be forced through the query-override path either.
func TestClientUpdateQueryReportedAheadShortCircuits(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "1.0.0", "1.1.0")
	w := env.do(http.MethodGet, "/api/v1/client/update?current_version=2.5.0",
		env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("query-reported-ahead must yield 204, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestClientUpdateUnknownVersionOffered: a device that has never reported a
// version (empty current) is behind by definition and IS offered the release
// (200). This documents the deliberate fall-through when no version is known.
func TestClientUpdateUnknownVersionOffered(t *testing.T) {
	env := newTestEnv(t)
	dev := setupDeployment(t, env, "", "1.1.0")
	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("device with unknown current must be offered the release; want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
}

// TestClientUpdateUnknownDeviceNoContent: an authenticated device token whose
// device record is absent is treated as up-to-date (204), never an error — this
// also covers the GetDevice-miss branch of handleClientUpdate.
func TestClientUpdateUnknownDeviceNoContent(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken("ghost-device"), nil, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("unknown device want 204, got %d (%s)", w.Code, w.Body.String())
	}
}
