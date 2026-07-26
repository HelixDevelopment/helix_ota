package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestAccountsM3_ClaimAccountAccess_AntiTautology proves the
// requireClaimAccountAccess middleware is a GENUINE cross-tenant gate,
// not a trivial pass-through or tautological deny.
//
// Design: §4.2 token-claim scoping. Anti-tautology per §11.4.115.
//
// Flow:
//  1. Account-A has project P1. User-A is a member of acct-A with
//     project access to P1.
//  2. User-A's scoped token (acct-A) sees P1 via GET /projects (200).
//  3. User-B has NO account membership. User-B's scoped token (acct-A,
//     claiming an unverified account) is BLOCKED (403) — the middleware
//     detects User-B is NOT a member of the claimed account.
//  4. ANTI-TAUTOLOGY RED: disable the middleware → User-B's identical
//     token now SEES P1 (200) — because User-B HAS project access and
//     handleListProjects returns it. This proves cross-tenant access
//     is GENUINELY possible without the middleware's enforcement.
//  5. ANTI-TAUTOLOGY GREEN: re-enable the middleware → User-B's token
//     is BLOCKED again (403) — the middleware is PROVEN to be the
//     active isolation mechanism.
func TestAccountsM3_ClaimAccountAccess_AntiTautology(t *testing.T) {
	// Guard: always reset the test hook, even if the test panics.
	defer func() { TestDisableClaimAccountAccess = false }()

	env := newTestEnv(t)

	acctA := "acct-m3a"
	userA := "usera@test"
	userB := "userb@test"
	projID := "proj-m3-01"

	// --- Setup: create account-A, project P1, memberships, and
	// project access grants ---

	mustCreateAccount(t, env.repo, acctA, "CorpA", "corpa")
	mustCreateProject(t, env.repo, projID, acctA, "m3-project")

	// User-A is a member of account-A (viewer).
	mustCreateMembership(t, env.repo, userA, acctA, store.AccountRoleViewer)

	// Grant both users project-level access to P1 — without this,
	// handleListProjects (non-admin path) would return empty regardless
	// of the middleware. User-A needs it for Phase 1; User-B needs it
	// for the RED phase to prove the handler CAN return P1 when the
	// middleware is not enforcing.
	if err := env.repo.SetProjectAccess(t.Context(), store.ProjectAccess{
		ProjectID: projID, CallerID: userA, Role: store.ProjectRoleViewer,
	}); err != nil {
		t.Fatalf("grant userA project access: %v", err)
	}
	if err := env.repo.SetProjectAccess(t.Context(), store.ProjectAccess{
		ProjectID: projID, CallerID: userB, Role: store.ProjectRoleViewer,
	}); err != nil {
		t.Fatalf("grant userB project access: %v", err)
	}

	// --- Phase 1: Account-A scoped token → project IS returned ---

	tokenA, err := env.signer.MintAccount(userA,
		[]string{RoleViewer}, acctA, 0, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint token A: %v", err)
	}

	w := env.doJSON(http.MethodGet, "/api/v1/projects", tokenA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 1 GREEN (acct-A can list): want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
	decodeAndAssertProject(t, env, w, projID)

	// --- Phase 2: Unverified account claim → project NOT returned ---
	//
	// User-B has NO account membership at all. The token carries
	// account_id=acct-A, but the middleware re-verifies membership:
	// GetAccountMembership(userB, acctA) → ErrNotFound → 403 FORBIDDEN.

	tokenB, err := env.signer.MintAccount(userB,
		[]string{RoleViewer}, acctA, 0, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint token B: %v", err)
	}

	w = env.doJSON(http.MethodGet, "/api/v1/projects", tokenB, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Phase 2 GREEN (cross-tenant blocked): want 403, got %d (%s)",
			w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("Phase 2 error code: want %s, got %s", CodeForbidden, code)
	}

	// --- Phase 3: ANTI-TAUTOLOGY RED → GREEN ---
	//
	// RED: disable the middleware → User-B's identical token now CAN
	//      see P1 (handleListProjects finds the project access grant).
	//      This proves the project IS accessible from the handler level
	//      when the middleware is not enforcing — GENUINE leak.
	//
	// GREEN: re-enable the middleware → User-B is blocked again,
	//        proving the middleware is the active cross-tenant gate.

	// RED (GENUINE leak without middleware).
	TestDisableClaimAccountAccess = true
	w = env.doJSON(http.MethodGet, "/api/v1/projects", tokenB, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 3 RED (GENUINE leak without middleware): want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
	decodeAndAssertProject(t, env, w, projID)

	// GREEN (isolation RESTORED with middleware re-enabled).
	TestDisableClaimAccountAccess = false
	w = env.doJSON(http.MethodGet, "/api/v1/projects", tokenB, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Phase 3 GREEN (isolation restored): want 403, got %d (%s)",
			w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("Phase 3 GREEN error code: want %s, got %s", CodeForbidden, code)
	}
}

// decodeAndAssertProject decodes the list-projects response and
// verifies the expected project is present.
func decodeAndAssertProject(t *testing.T, env *testEnv, w *httptest.ResponseRecorder, wantID string) {
	t.Helper()
	var body struct{ Items []ProjectResponse }
	env.decode(w, &body)
	for _, p := range body.Items {
		if p.ProjectID == wantID {
			return
		}
	}
	t.Fatalf("project %s not found in response: %+v", wantID, body.Items)
}
