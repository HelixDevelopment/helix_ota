package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// --- Accounts M2: token claim extraction + authZ middleware tests (design §5.1) ---

func TestAccountsM2_RequireAccountAccess_DeniesUnscopedToken(t *testing.T) {
	// RED: a token WITHOUT an account_id claim is denied on every account-scoped
	// route (fail-closed per design §3.3/J). The account_id claim is optional in the
	// struct but the middleware requires it for non-super-admin callers.
	env := newTestEnv(t)

	// Seed an account + a project so the route has real data to attempt access on.
	acctID := "acct-01"
	mustCreateAccount(t, env.repo, acctID, "TestCorp", "testcorp")
	mustCreateProject(t, env.repo, "proj-01", acctID, "main")

	// Mint an UNscoped token (no account_id claim).
	unscopedToken, err := env.signer.Mint("admin@helix.test",
		[]string{RoleAdmin, RoleOperator, RoleViewer}, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint unscoped token: %v", err)
	}

	w := env.doJSON(http.MethodGet, "/api/v1/accounts/acct-01/projects", unscopedToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("RED: unscoped token on account-scoped route: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("RED: unscoped token error code: want %s, got %s", CodeForbidden, code)
	}
}

func TestAccountsM2_RequireAccountAccess_DeniesNonMember(t *testing.T) {
	// RED: a token scoped to account-A is denied on a request targeting account-B
	// (the load-bearing cross-tenant isolation invariant, design §0).
	env := newTestEnv(t)

	acctA := "acct-a"
	acctB := "acct-b"
	mustCreateAccount(t, env.repo, acctA, "CorpA", "corpa")
	mustCreateAccount(t, env.repo, acctB, "CorpB", "corpb")
	mustCreateProject(t, env.repo, "proj-a1", acctA, "main")

	// Create membership for the caller in account-A only.
	mustCreateMembership(t, env.repo, "admin@helix.test", acctA, store.AccountRoleAdmin)

	// Mint a token scoped to account-A.
	tokenA, err := env.signer.MintAccount("admin@helix.test",
		[]string{RoleAdmin, RoleOperator, RoleViewer}, acctA, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint scoped token: %v", err)
	}

	// Attempt to access account-B's projects with account-A's token.
	w := env.doJSON(http.MethodGet, "/api/v1/accounts/acct-b/projects", tokenA, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("RED: cross-account access: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("RED: cross-account error code: want %s, got %s", CodeForbidden, code)
	}
}

func TestAccountsM2_RequireAccountAccess_AntiTautology(t *testing.T) {
	// Anti-tautology RED→GREEN (design §5.1 / §11.4.115):
	// 1. Confirm a NON-member of the account is DENIED (403) — RED.
	// 2. Add membership → confirm access is ALLOWED (200) — GREEN.
	// This proves the middleware is not a trivial "always deny" gate.
	env := newTestEnv(t)

	acctID := "acct-xt"
	mustCreateAccount(t, env.repo, acctID, "XTest", "xtest")
	mustCreateProject(t, env.repo, "proj-xt1", acctID, "main")

	// Phase 1: non-member — RED.
	nonMemberToken, err := env.signer.MintAccount("outsider@helix.test",
		[]string{RoleAdmin}, acctID, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint non-member token: %v", err)
	}
	w := env.doJSON(http.MethodGet, "/api/v1/accounts/acct-xt/projects", nonMemberToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Phase 1 RED (non-member): want 403, got %d (%s)", w.Code, w.Body.String())
	}

	// Phase 2: grant membership → GREEN.
	mustCreateMembership(t, env.repo, "outsider@helix.test", acctID, store.AccountRoleViewer)

	w = env.doJSON(http.MethodGet, "/api/v1/accounts/acct-xt/projects", nonMemberToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 2 GREEN (new member): want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var projects []AccountProjectResponse
	env.decode(w, &projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ProjectID != "proj-xt1" {
		t.Fatalf("expected proj-xt1, got %s", projects[0].ProjectID)
	}
}

func TestAccountsM2_RequireSuperAdmin_DeniesNonSuperAdmin(t *testing.T) {
	// RED: a token without the super_admin role is denied on super-admin routes.
	env := newTestEnv(t)

	adminToken := env.adminToken()
	w := env.doJSON(http.MethodGet, "/api/v1/admin/accounts", adminToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("RED: non-super-admin on super-admin route: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if code := env.errCode(w); code != CodeForbidden {
		t.Fatalf("RED: super-admin denial error code: want %s, got %s", CodeForbidden, code)
	}
}

func TestAccountsM2_RequireSuperAdmin_AllowsSuperAdmin(t *testing.T) {
	// GREEN: a token WITH the super_admin role can access super-admin routes.
	env := newTestEnv(t)

	// Create an account so the list returns something for visual confirmation.
	mustCreateAccount(t, env.repo, "acct-sa", "SA Test", "satest")

	superToken, err := env.signer.Mint("super@helix.test",
		[]string{RoleSuperAdmin}, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint super-admin token: %v", err)
	}

	w := env.doJSON(http.MethodGet, "/api/v1/admin/accounts", superToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GREEN: super-admin list accounts: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var accounts []AccountResponse
	env.decode(w, &accounts)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].AccountID != "acct-sa" {
		t.Fatalf("expected acct-sa, got %s", accounts[0].AccountID)
	}
}

func TestAccountsM2_RequireAccountAccess_SuperAdminBypass(t *testing.T) {
	// GREEN: a super-admin bypasses the tenant-isolation predicate (design §3.4).
	// They can access ANY account's resources without needing membership.
	env := newTestEnv(t)

	acctID := "acct-sa-bypass"
	mustCreateAccount(t, env.repo, acctID, "SABypass", "sabypass")
	mustCreateProject(t, env.repo, "proj-sa-01", acctID, "main")

	superToken, err := env.signer.Mint("super@helix.test",
		[]string{RoleSuperAdmin}, time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("mint super-admin token: %v", err)
	}

	// Super-admin has no membership in this account, yet can access.
	_, err = env.repo.GetAccountMembership(t.Context(), "super@helix.test", acctID)
	if err == nil {
		t.Fatal("precondition: super-admin should NOT have membership in this account")
	}

	w := env.doJSON(http.MethodGet, "/api/v1/accounts/acct-sa-bypass/projects", superToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GREEN: super-admin bypass on account-scoped route: want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAccountsM2_TokenRoundTrip_AccountIDClaim(t *testing.T) {
	// GREEN: prove MintAccount embeds the account_id claim and Verify recovers it.
	env := newTestEnv(t)

	tok, err := env.signer.MintAccount("testuser", []string{RoleViewer},
		"acct-claims-test", time.Hour, env.srv.now())
	if err != nil {
		t.Fatalf("MintAccount: %v", err)
	}

	claims, err := env.signer.Verify(tok, env.srv.now())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.AccountID != "acct-claims-test" {
		t.Fatalf("AccountID: want 'acct-claims-test', got %q", claims.AccountID)
	}
	if claims.Subject != "testuser" {
		t.Fatalf("Subject: want 'testuser', got %q", claims.Subject)
	}
	if !claims.HasRole(RoleViewer) {
		t.Fatal("HasRole(viewer): want true")
	}
}

// --- helpers ---

func mustCreateAccount(t *testing.T, repo *store.MemoryRepository, id, name, slug string) {
	t.Helper()
	err := repo.CreateAccount(t.Context(), store.Account{
		AccountID: id,
		Name:      name,
		Slug:      slug,
		Status:    store.AccountStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateAccount(%s): %v", id, err)
	}
}

func mustCreateProject(t *testing.T, repo *store.MemoryRepository, projID, acctID, name string) {
	t.Helper()
	err := repo.CreateProject(t.Context(), store.Project{
		ProjectID:   projID,
		AccountID:   acctID,
		Name:        name,
		Description: "test project",
	})
	if err != nil {
		t.Fatalf("CreateProject(%s): %v", projID, err)
	}
}

func mustCreateMembership(t *testing.T, repo *store.MemoryRepository, userID, acctID string, role store.AccountRole) {
	t.Helper()
	err := repo.SetAccountMembership(t.Context(), store.AccountMembership{
		UserID: userID, AccountID: acctID, Role: role,
		IsOwner: false, GrantedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("SetAccountMembership(%s, %s): %v", userID, acctID, err)
	}
}
