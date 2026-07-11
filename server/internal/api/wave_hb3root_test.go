package api

import (
	"context"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestRecall_HB3ROOT_SecondRecallOnSupersededDeploymentRejected is the
// permanent HB-3-ROOT guard (§11.4.115 GREEN polarity). handleRecall took
// NEITHER s.deployMu NOR an active-status check, so recalling an
// already-superseded deployment (the deterministic manifestation of a
// recall-vs-create / recall-vs-recall race) created a SECOND
// simultaneously-active deployment for one target — a polling device would
// then be offered a nondeterministic release. The fix re-reads the deployment
// under deployMu and rejects a recall of an already-superseded one.
//
// Anti-tautology anchor: disabling the `if dep.Status == "superseded"` guard
// (`if false && ...`) lets the second recall proceed → 201 + a 2nd active
// deployment → RED; restore → 409 + exactly one active → GREEN.
func TestRecall_HB3ROOT_SecondRecallOnSupersededDeploymentRejected(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()
	ctx := context.Background()

	priorArtifact := env.newArtifactDirect("1.0.0")
	if err := env.repo.CreateRelease(ctx, store.Release{
		ReleaseID: "rel-prior", ArtifactID: priorArtifact, Version: "1.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published",
		CreatedAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("insert prior release: %v", err)
	}

	w1 := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok,
		RecallRequest{ToReleaseID: "rel-prior", Reason: "first"})
	if w1.Code != http.StatusCreated {
		t.Fatalf("first recall want 201, got %d (%s)", w1.Code, w1.Body.String())
	}

	// A SECOND recall of the same (now-superseded) deployment must be rejected.
	w2 := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok,
		RecallRequest{ToReleaseID: "rel-prior", Reason: "second"})
	if w2.Code != http.StatusConflict {
		t.Fatalf("HB-3-ROOT: recalling an already-superseded deployment must 409, got %d (%s)", w2.Code, w2.Body.String())
	}

	active, err := env.repo.ListActiveDeployments(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("HB-3-ROOT: exactly one active deployment must remain after a double-recall; got %d", len(active))
	}
}
