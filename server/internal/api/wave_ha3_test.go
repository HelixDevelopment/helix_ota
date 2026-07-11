package api

import (
	"context"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestRecall_HA3_StampsFleetTargetVersion is the permanent HA-3 regression
// guard (§11.4.115 GREEN polarity). Before the fix, handleRecall created the
// recall deployment + rollback record but NEVER called assignTargetVersion, so
// a device's TargetVersion stayed pinned to the superseded release after a
// recall — GET /devices and device-status endpoints then reported stale,
// misleading target state (§11.4.108). After the fix, a recall stamps the
// fleet's TargetVersion to the recall (target) release, mirroring
// handleCreateDeployment.
//
// Anti-tautology anchor: replacing `s.assignTargetVersion(ctx, toRel, dep.Group)`
// with `_ = toRel` in handleRecall (removing the stamping while keeping the
// build valid) flips this test RED — the device keeps 1.1.0; restore → GREEN.
func TestRecall_HA3_StampsFleetTargetVersion(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()
	ctx := context.Background()

	// A device matching the recall target's os/model, initially carrying the
	// CURRENT release's version as its target (the stale state a recall must fix).
	const devID = "dev-ha3"
	if err := env.repo.CreateDevice(ctx, store.Device{
		DeviceID: devID, Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid,
		Group: "field-fleet-a", TargetVersion: "1.1.0",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}

	// Prior-good release to recall TO (1.0.0), same os/model so the device matches.
	priorArtifact := env.newArtifactDirect("1.0.0")
	if err := env.repo.CreateRelease(ctx, store.Release{
		ReleaseID: "rel-prior", ArtifactID: priorArtifact, Version: "1.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published",
		CreatedAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("insert prior release: %v", err)
	}

	w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok,
		RecallRequest{ToReleaseID: "rel-prior", Reason: "high error rate"})
	if w.Code != http.StatusCreated {
		t.Fatalf("recall want 201, got %d (%s)", w.Code, w.Body.String())
	}

	got := ""
	for _, d := range env.repo.AllDevices(ctx) {
		if d.DeviceID == devID {
			got = d.TargetVersion
		}
	}
	if got != "1.0.0" {
		t.Fatalf("HA-3: after recall the device TargetVersion must follow the recall release 1.0.0, got %q", got)
	}
}
