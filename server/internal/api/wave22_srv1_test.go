package api

import (
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// Wave22 SRV-1 regression: the release eligibility floor (min_current_version)
// is accepted, stored, and persisted — but handleClientUpdate must actually
// ENFORCE it. A device whose current version is strictly below the
// operator-declared floor MUST NOT be offered the gated release (204), while a
// device at or above the floor (and behind the release version) IS offered it
// (200). Before the SRV-1 fix, the floor was never read in handleClientUpdate,
// so a below-floor device was offered + installed the gated release.
//
// This test also pins the complementary operator-visibility change: the
// release-create response echoes min_current_version so an operator can confirm
// the floor stuck.

// setupDeploymentWithFloor mirrors setupDeployment but publishes the release
// with a min_current_version eligibility floor. It asserts the create-release
// response echoes that floor (SRV-1 operator-visibility) and returns the device.
func setupDeploymentWithFloor(t *testing.T, env *testEnv, deviceCurrent, releaseVersion, floor string) DeviceRegistered {
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
		ArtifactID:        art.ArtifactID,
		Version:           releaseVersion,
		OS:                otaprotocol.OSAndroid,
		TargetModel:       "OrangePi5Max",
		MinCurrentVersion: floor,
	})
	if rw.Code != http.StatusCreated {
		t.Fatalf("release want 201, got %d (%s)", rw.Code, rw.Body.String())
	}
	var rel Release
	env.decode(rw, &rel)
	// SRV-1 operator-visibility: the stored floor is echoed back on create.
	if rel.MinCurrentVersion != floor {
		t.Fatalf("release response must echo min_current_version %q, got %q (%s)",
			floor, rel.MinCurrentVersion, rw.Body.String())
	}

	dw := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: rel.ReleaseID, Strategy: "all-targets", Group: "field-fleet-a",
	})
	if dw.Code != http.StatusCreated {
		t.Fatalf("deployment want 201, got %d (%s)", dw.Code, dw.Body.String())
	}
	return dev
}

// TestClientUpdateEnforcesMinCurrentVersionFloor is the SRV-1 anti-tautology
// regression: below-floor -> 204, at-or-above-floor -> 200 with the offer.
func TestClientUpdateEnforcesMinCurrentVersionFloor(t *testing.T) {
	// Below the floor: device@1.0.0, release 2.0.0 gated at min_current_version
	// 1.5.0. The device is behind the release (so it passes the on-target check)
	// but below the floor -> ineligible -> 204. This is the RED case: before the
	// fix this returned 200 with the 2.0.0 offer.
	t.Run("below_floor_is_ineligible_204", func(t *testing.T) {
		env := newTestEnv(t)
		dev := setupDeploymentWithFloor(t, env, "1.0.0", "2.0.0", "1.5.0")

		w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("device below the min_current_version floor must NOT be offered the gated release; "+
				"want 204, got %d (%s)", w.Code, w.Body.String())
		}
		if w.Body.Len() != 0 {
			t.Fatalf("204 must have an empty body, got %q", w.Body.String())
		}
	})

	// At/above the floor: device@1.6.0, same release 2.0.0 gated at 1.5.0. The
	// device is still behind the release version (2.0.0) and at/above the floor
	// -> eligible -> 200 with the 2.0.0 offer. Positive control proving the guard
	// does not over-block.
	t.Run("at_or_above_floor_is_offered_200", func(t *testing.T) {
		env := newTestEnv(t)
		dev := setupDeploymentWithFloor(t, env, "1.6.0", "2.0.0", "1.5.0")

		w := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
		if w.Code != http.StatusOK {
			t.Fatalf("device at/above the floor and behind the release must be offered it; "+
				"want 200, got %d (%s)", w.Code, w.Body.String())
		}
		var upd otaprotocol.UpdateAvailable
		env.decode(w, &upd)
		if upd.Version != "2.0.0" {
			t.Fatalf("update offer version want 2.0.0, got %q (%+v)", upd.Version, upd)
		}
	})
}
