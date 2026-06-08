package api

import (
	"net/http"
	"testing"
)

// activeDeploymentID returns the id of the all-targets deployment created by
// setupDeployment.
func activeDeploymentID(t *testing.T, env *testEnv) string {
	t.Helper()
	deps, _ := env.repo.ListActiveDeployments(nil)
	if len(deps) == 0 {
		t.Fatalf("expected an active deployment")
	}
	return deps[0].DeploymentID
}

func twoPhaseBody() RolloutCreate {
	return RolloutCreate{Phases: []RolloutPhaseSpec{
		{Percentage: 50, SuccessThreshold: 0.9, ErrorThreshold: 0.1, DurationSeconds: 0, AutoProgress: true},
		{Percentage: 100, SuccessThreshold: 0.9, ErrorThreshold: 0.1, DurationSeconds: 0, AutoProgress: true},
	}}
}

func TestRolloutCreateGetEvaluate(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	// Create + start.
	w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, twoPhaseBody())
	if w.Code != http.StatusCreated {
		t.Fatalf("create rollout want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var st RolloutState
	env.decode(w, &st)
	if st.Status != "active" || st.CurrentPhase != 0 || len(st.Phases) != 2 {
		t.Fatalf("rollout state after create: %+v", st)
	}

	// Get.
	gw := env.do(http.MethodGet, "/api/v1/deployments/"+depID+"/rollout", tok, nil, "")
	if gw.Code != http.StatusOK {
		t.Fatalf("get rollout want 200, got %d", gw.Code)
	}

	// Evaluate phase 0 healthy -> advance to phase 1.
	ew := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok,
		RolloutVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	if ew.Code != http.StatusOK {
		t.Fatalf("evaluate want 200, got %d (%s)", ew.Code, ew.Body.String())
	}
	var dec RolloutDecision
	env.decode(ew, &dec)
	if dec.Action != "advance" || dec.State.CurrentPhase != 1 {
		t.Fatalf("evaluate decision: %+v", dec)
	}

	// Evaluate final healthy -> complete.
	ew2 := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok,
		RolloutVerdict{SuccessRate: 0.95, ErrorRate: 0.0})
	var dec2 RolloutDecision
	env.decode(ew2, &dec2)
	if dec2.Action != "complete" || dec2.State.Status != "completed" {
		t.Fatalf("final evaluate decision: %+v", dec2)
	}
}

func TestRolloutHaltsOnErrorBreach(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()
	env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, twoPhaseBody())

	ew := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok,
		RolloutVerdict{SuccessRate: 0.99, ErrorRate: 0.5})
	var dec RolloutDecision
	env.decode(ew, &dec)
	if dec.Action != "halt" || dec.State.Status != "halted" {
		t.Fatalf("breach decision want halt/halted, got %+v", dec)
	}
}

func TestRolloutUnknownDeploymentAndBadPlan(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Unknown deployment -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/ghost/rollout", tok, twoPhaseBody()); w.Code != http.StatusNotFound {
		t.Fatalf("rollout on unknown deployment want 404, got %d", w.Code)
	}

	// Existing deployment but invalid plan (final != 100) -> 400.
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	bad := RolloutCreate{Phases: []RolloutPhaseSpec{{Percentage: 50, SuccessThreshold: 0.9, ErrorThreshold: 0.1, AutoProgress: true}}}
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, bad); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid plan (final != 100) want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRolloutCreateForbiddenForViewer(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	viewer, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", viewer, twoPhaseBody()); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create rollout want 403, got %d", w.Code)
	}
}
