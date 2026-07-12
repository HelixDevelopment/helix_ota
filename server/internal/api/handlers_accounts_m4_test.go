package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestAccountsM4_SelectAccount_AntiTautology proves the select-account
// membership check is a GENUINE cross-tenant gate, not a trivial
// always-deny or always-allow tautology.
//
// Accounts M4 (design §4.3). Anti-tautology per §11.4.115.
//
// Flow:
//  1. Account-A exists. User-A is a member of account-A (admin).
//     Account-B exists. User-A is NOT a member of account-B.
//  2. User-A logs in → unscoped token. Token carries no account_id claim.
//  3. GREEN (Phase 1): User-A selects account-A (member) → 200 with
//     account-A-scoped token. Proves the happy path works.
//  4. RED (Phase 2): User-A attempts select-account for account-B
//     (non-member) → 403 FORBIDDEN. The membership check blocks
//     cross-tenant access.
//  5. ANTI-TAUTOLOGY RED (Phase 3): disable the membership check →
//     User-A's identical select-account for account-B NOW succeeds (200).
//     This proves the handler CAN mint a scoped token for account-B when
//     the check is bypassed — GENUINE leak without the gate.
//  6. ANTI-TAUTOLOGY GREEN (Phase 4): re-enable the check → User-A is
//     blocked again (403). Proves the membership check is the ACTIVE
//     isolation mechanism.
func TestAccountsM4_SelectAccount_AntiTautology(t *testing.T) {
	// Guard: always reset the test hook, even if the test panics.
	defer func() { TestDisableSelectAccountMembershipCheck = false }()

	env := newTestEnv(t)

	acctA := "acct-m4a"
	acctB := "acct-m4b"
	adminUser := "admin@helix.test"

	// --- Setup: create both accounts, grant the admin user membership in A only ---

	mustCreateAccount(t, env.repo, acctA, "CorpA", "corpa")
	mustCreateAccount(t, env.repo, acctB, "CorpB", "corpb")
	mustCreateMembership(t, env.repo, adminUser, acctA, store.AccountRoleAdmin)

	// --- Phase 1: GREEN — User-A selects their OWN account (member) ---

	// Login to get an unscoped token.
	w := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", LoginRequest{
		Username: "admin@helix.test", Password: "s3cret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 1 login: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var login TokenResponse
	env.decode(w, &login)
	unscopedToken := login.AccessToken

	// Select account-A (member) — should succeed.
	selReq := SelectAccountRequest{AccountID: acctA}
	w = env.doJSON(http.MethodPost, "/api/v1/auth/select-account", unscopedToken, selReq)
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 1 GREEN (select own account): want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
	var selResp TokenResponse
	env.decode(w, &selResp)
	if selResp.AccessToken == "" {
		t.Fatal("Phase 1 GREEN: expected non-empty access token")
	}

	// Verify the returned token is scoped to account-A.
	claims, err := env.signer.Verify(selResp.AccessToken, env.srv.now())
	if err != nil {
		t.Fatalf("Phase 1 GREEN verify token: %v", err)
	}
	if claims.AccountID != acctA {
		t.Fatalf("Phase 1 GREEN account scope: want %s, got %q", acctA, claims.AccountID)
	}

	// --- Phase 2: RED — User-A tries to select account-B (non-member) ---

	w = env.doJSON(http.MethodPost, "/api/v1/auth/select-account", unscopedToken,
		SelectAccountRequest{AccountID: acctB})
	if w.Code != http.StatusForbidden {
		t.Fatalf("Phase 2 RED (non-member blocked): want 403, got %d (%s)",
			w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("Phase 2 RED error code: want %s, got %s", CodeForbidden, code)
	}

	// --- Phase 3: ANTI-TAUTOLOGY RED → GREEN ---
	//
	// RED: disable the membership check → User-A CAN select account-B.
	//      This proves the handler CAN mint a scoped token for a non-member
	//      account — the leak is GENUINE without the gate.
	//
	// GREEN: re-enable the check → User-A is blocked again, proving the
	//        membership check is the ACTIVE isolation mechanism.

	// RED (GENUINE leak without membership check).
	TestDisableSelectAccountMembershipCheck = true
	w = env.doJSON(http.MethodPost, "/api/v1/auth/select-account", unscopedToken,
		SelectAccountRequest{AccountID: acctB})
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 3 RED (GENUINE leak without membership check): want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
	var leakResp TokenResponse
	env.decode(w, &leakResp)
	leakClaims, err := env.signer.Verify(leakResp.AccessToken, env.srv.now())
	if err != nil {
		t.Fatalf("Phase 3 RED verify leaked token: %v", err)
	}
	if leakClaims.AccountID != acctB {
		t.Fatalf("Phase 3 RED account scope: want %s, got %q", acctB, leakClaims.AccountID)
	}

	// GREEN (isolation RESTORED with membership check re-enabled).
	TestDisableSelectAccountMembershipCheck = false
	w = env.doJSON(http.MethodPost, "/api/v1/auth/select-account", unscopedToken,
		SelectAccountRequest{AccountID: acctB})
	if w.Code != http.StatusForbidden {
		t.Fatalf("Phase 3 GREEN (isolation restored): want 403, got %d (%s)",
			w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("Phase 3 GREEN error code: want %s, got %s", CodeForbidden, code)
	}
}

// TestAccountsM4_LoginReturnsAccounts proves the login response carries the
// available-accounts list when the user has account memberships.
func TestAccountsM4_LoginReturnsAccounts(t *testing.T) {
	env := newTestEnv(t)

	acctA := "acct-m4-la"
	acctB := "acct-m4-lb"
	user := "admin@helix.test"

	mustCreateAccount(t, env.repo, acctA, "CorpA", "corpa")
	mustCreateAccount(t, env.repo, acctB, "CorpB", "corpb")
	mustCreateMembership(t, env.repo, user, acctA, store.AccountRoleAdmin)
	mustCreateMembership(t, env.repo, user, acctB, store.AccountRoleViewer)

	w := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", LoginRequest{
		Username: user, Password: "s3cret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp TokenResponse
	env.decode(w, &resp)

	if len(resp.Accounts) != 2 {
		t.Fatalf("expected 2 accounts in login response, got %d: %+v",
			len(resp.Accounts), resp.Accounts)
	}

	// Build a lookup map for assertion.
	byID := make(map[string]AccountEntry, len(resp.Accounts))
	for _, a := range resp.Accounts {
		byID[a.AccountID] = a
	}

	a, ok := byID[acctA]
	if !ok {
		t.Fatalf("account %s not found in response", acctA)
	}
	if a.AccountName != "CorpA" || a.Role != "admin" {
		t.Fatalf("account A: want (CorpA, admin), got (%s, %s)", a.AccountName, a.Role)
	}

	b, ok := byID[acctB]
	if !ok {
		t.Fatalf("account %s not found in response", acctB)
	}
	if b.AccountName != "CorpB" || b.Role != "viewer" {
		t.Fatalf("account B: want (CorpB, viewer), got (%s, %s)", b.AccountName, b.Role)
	}

	// The access token should be unscoped (no account_id claim yet).
	if resp.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	claims, err := env.signer.Verify(resp.AccessToken, env.srv.now())
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.AccountID != "" {
		t.Fatalf("login token should be unscoped (no account_id), got %q", claims.AccountID)
	}
}

// TestAccountsM4_LoginWithoutMembershipsReturnsEmptyAccounts proves that a user
// with no account memberships gets an empty accounts list (not an error).
func TestAccountsM4_LoginWithoutMembershipsReturnsEmptyAccounts(t *testing.T) {
	env := newTestEnv(t)

	w := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", LoginRequest{
		Username: "admin@helix.test", Password: "s3cret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp TokenResponse
	env.decode(w, &resp)

	if resp.Accounts != nil && len(resp.Accounts) > 0 {
		t.Fatalf("expected nil or empty accounts, got %d", len(resp.Accounts))
	}
}

// TestAccountsM4_SelectAccount_SuperAdminBypass proves a super-admin can
// select any account without membership (design §3.4 bypass).
func TestAccountsM4_SelectAccount_SuperAdminBypass(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m4-sa"
	mustCreateAccount(t, env.repo, acctID, "SABypass", "sabypass")

	// Super-admin has NO membership in this account.
	_, err := env.repo.GetAccountMembership(t.Context(), "super@test", acctID)
	if err == nil {
		t.Fatal("precondition: super-admin must NOT have membership")
	}

	// Mint an unscoped super-admin token.
	superTok, err := env.signer.Mint("super@test", []string{RoleSuperAdmin},
		time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint super-admin token: %v", err)
	}

	w := env.doJSON(http.MethodPost, "/api/v1/auth/select-account", superTok,
		SelectAccountRequest{AccountID: acctID})
	if w.Code != http.StatusOK {
		t.Fatalf("super-admin select-account: want 200, got %d (%s)",
			w.Code, w.Body.String())
	}
}

// TestAccountsM4_SelectAccount_RequiresAuth proves select-account requires
// a valid bearer token.
func TestAccountsM4_SelectAccount_RequiresAuth(t *testing.T) {
	env := newTestEnv(t)

	w := env.doJSON(http.MethodPost, "/api/v1/auth/select-account", "",
		SelectAccountRequest{AccountID: "anything"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d (%s)", w.Code, w.Body.String())
	}
}
