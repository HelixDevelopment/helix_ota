package api

import (
	"context"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestRecallRecordsRollback drives a server-driven recall: a deployment on
// release 1.1.0 is recalled to a prior release, recording a rollback_history
// row that GET /rollbacks returns.
func TestRecallRecordsRollback(t *testing.T) {
	env := newTestEnv(t)
	// setupDeployment uploads+releases 1.1.0 and creates an all-targets deployment.
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	// A prior-good release to recall TO. Inserted directly (the API would reject
	// creating an older version via S4 monotonicity — a recall target is by
	// definition a release that predates the current one).
	priorArtifact := env.newArtifactDirect("1.0.0")
	if err := env.repo.CreateRelease(context.Background(), store.Release{
		ReleaseID: "rel-prior", ArtifactID: priorArtifact, Version: "1.0.0",
		OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Status: "published",
		CreatedAt: env.srv.now(),
	}); err != nil {
		t.Fatalf("insert prior release: %v", err)
	}

	// Recall.
	w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok,
		RecallRequest{ToReleaseID: "rel-prior", Reason: "high error rate"})
	if w.Code != http.StatusCreated {
		t.Fatalf("recall want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var rb RollbackView
	env.decode(w, &rb)
	if rb.Kind != "rollback" || rb.ToReleaseID != "rel-prior" || rb.FromReleaseID == "" || rb.TriggeredBy != "admin@helix.test" {
		t.Fatalf("recall record mismatch: %+v", rb)
	}

	// History lists it.
	lw := env.do(http.MethodGet, "/api/v1/deployments/"+depID+"/rollbacks", tok, nil, "")
	if lw.Code != http.StatusOK {
		t.Fatalf("list rollbacks want 200, got %d", lw.Code)
	}
	var list RollbackList
	env.decode(lw, &list)
	if len(list.Items) != 1 || list.Items[0].Kind != "rollback" {
		t.Fatalf("rollback history mismatch: %+v", list.Items)
	}
}

func TestRecallValidation(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	// Unknown deployment -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/ghost/recall", tok, RecallRequest{ToReleaseID: "r"}); w.Code != http.StatusNotFound {
		t.Fatalf("recall unknown deployment want 404, got %d", w.Code)
	}
	// Missing to_release_id -> 400.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok, RecallRequest{}); w.Code != http.StatusBadRequest {
		t.Fatalf("recall without to_release_id want 400, got %d", w.Code)
	}
	// Non-existent target release -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", tok, RecallRequest{ToReleaseID: "no-such-release"}); w.Code != http.StatusNotFound {
		t.Fatalf("recall to unknown release want 404, got %d", w.Code)
	}
}

func TestRecallForbiddenForViewer(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	viewer, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", viewer, RecallRequest{ToReleaseID: "r"}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer recall want 403, got %d", w.Code)
	}
}
