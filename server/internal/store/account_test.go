package store

// Accounts M1 — the tenant layer above Project + THE load-bearing cross-tenant
// isolation invariant (design §0). This suite runs in-suite under plain
// `go test ./...` on the in-memory backend (TestMemoryCrossTenantIsolation +
// the runRepositoryContract wiring), and against the pgx/PostgreSQL backend in
// the -tags integration contract, so the two are proven at PARITY.
//
// Anti-tautology (§11.4.115 / design §5.1): the isolation assertions below use a
// THREE-PART "not-a-false-negative" shape (a broken query returns empty for
// EVERYONE): (i) A's own row is PRESENT, (ii) B's row is ABSENT under A's scope,
// (iii) vice-versa. The paired §1.1 mutation is dropping the `p.AccountID !=
// accountID` scope predicate in memory.go ListProjectsForAccount /
// GetProjectForAccount (the pgx twin is the `AND account_id=$N` WHERE clause):
// with the predicate removed part (ii) FAILs (acct-A's scope returns acct-B's
// project). Proven RED->GREEN this session against the in-memory backend and
// captured in the commit message — the guard genuinely catches the leak and is
// not a tautology.

import (
	"context"
	"errors"
	"testing"
	"time"
)

// runAccountContract exercises the Accounts M1 tenant layer + cross-tenant
// isolation against ANY Repository, so the in-memory and pgx backends are held
// to identical behaviour. The caller MUST pass a freshly-emptied repository.
func runAccountContract(t *testing.T, repo Repository, ts time.Time) {
	t.Helper()
	ctx := context.Background()

	// --- two tenants; CreateAccountWithOwner closes the orphan-tenant hole ---
	acctA := Account{AccountID: "acct-a", Name: "Account A", Slug: "account-a", Status: AccountStatusActive, CreatedAt: ts, UpdatedAt: ts}
	acctB := Account{AccountID: "acct-b", Name: "Account B", Slug: "account-b", Status: AccountStatusActive, CreatedAt: ts, UpdatedAt: ts}
	if err := repo.CreateAccountWithOwner(ctx, acctA, "user-a", AccountRoleAdmin); err != nil {
		t.Fatalf("CreateAccountWithOwner A: %v", err)
	}
	if err := repo.CreateAccount(ctx, acctB); err != nil {
		t.Fatalf("CreateAccount B: %v", err)
	}
	// An empty owner id is rejected BEFORE any account row is written (no orphan).
	if err := repo.CreateAccountWithOwner(ctx, Account{AccountID: "acct-empty", Name: "Empty Owner", Slug: "empty-owner", Status: AccountStatusActive, CreatedAt: ts, UpdatedAt: ts}, "", AccountRoleAdmin); err == nil {
		t.Fatal("CreateAccountWithOwner with an empty owner id must error")
	}
	if _, err := repo.GetAccount(ctx, "acct-empty"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a rejected CreateAccountWithOwner must not leave an orphan account, got %v", err)
	}
	// Duplicate name -> conflict; duplicate slug -> conflict (both UNIQUE).
	if err := repo.CreateAccount(ctx, Account{AccountID: "acct-c", Name: "Account A", Slug: "account-c", Status: AccountStatusActive, CreatedAt: ts, UpdatedAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate account name want ErrConflict, got %v", err)
	}
	if err := repo.CreateAccount(ctx, Account{AccountID: "acct-d", Name: "Account D", Slug: "account-a", Status: AccountStatusActive, CreatedAt: ts, UpdatedAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate account slug want ErrConflict, got %v", err)
	}
	if got, err := repo.GetAccount(ctx, "acct-a"); err != nil || got.Name != "Account A" || got.Slug != "account-a" || got.Status != AccountStatusActive {
		t.Fatalf("GetAccount A round-trip: %+v err=%v", got, err)
	}
	if _, err := repo.GetAccount(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAccount unknown want ErrNotFound, got %v", err)
	}
	// ListAccounts holds both (order-insensitive so memory-insertion vs pgx-seq
	// order never makes this backend-specific).
	accs, err := repo.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	seenAcct := map[string]bool{}
	for _, a := range accs {
		seenAcct[a.AccountID] = true
	}
	if !seenAcct["acct-a"] || !seenAcct["acct-b"] {
		t.Fatalf("ListAccounts must hold acct-a and acct-b, got %+v", accs)
	}

	// --- membership isolation ---
	// The owner membership CreateAccountWithOwner wrote is present + is_owner.
	if mem, err := repo.GetAccountMembership(ctx, "user-a", "acct-a"); err != nil ||
		mem.Role != AccountRoleAdmin || !mem.IsOwner || mem.UserID != "user-a" || mem.AccountID != "acct-a" {
		t.Fatalf("GetAccountMembership owner (user-a, acct-a): %+v err=%v", mem, err)
	}
	// user-a has NO membership in acct-b -> ErrNotFound (the cross-tenant deny).
	if _, err := repo.GetAccountMembership(ctx, "user-a", "acct-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant membership (user-a in acct-b) want ErrNotFound, got %v", err)
	}
	// user-a's membership list holds acct-a only (never acct-b).
	mems, err := repo.ListAccountMemberships(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListAccountMemberships: %v", err)
	}
	if len(mems) != 1 || mems[0].AccountID != "acct-a" {
		t.Fatalf("ListAccountMemberships(user-a) want exactly [acct-a], got %+v", mems)
	}

	// --- project tenant-scoping isolation (THE §0 invariant) ---
	prjA := Project{ProjectID: "prj-a1", AccountID: "acct-a", Name: "A-fleet", CreatedAt: ts, UpdatedAt: ts}
	prjB := Project{ProjectID: "prj-b1", AccountID: "acct-b", Name: "B-fleet", CreatedAt: ts, UpdatedAt: ts}
	if err := repo.CreateProject(ctx, prjA); err != nil {
		t.Fatalf("CreateProject A: %v", err)
	}
	if err := repo.CreateProject(ctx, prjB); err != nil {
		t.Fatalf("CreateProject B: %v", err)
	}

	// (i) A's project PRESENT under A; (ii) B's project ABSENT under A.
	listA, err := repo.ListProjectsForAccount(ctx, "acct-a")
	if err != nil {
		t.Fatalf("ListProjectsForAccount A: %v", err)
	}
	if !containsProject(listA, "prj-a1") {
		t.Fatalf("ListProjectsForAccount(acct-a) MUST contain prj-a1 (present-check), got %+v", listA)
	}
	if containsProject(listA, "prj-b1") {
		t.Fatalf("CROSS-TENANT LEAK: ListProjectsForAccount(acct-a) returned acct-b's prj-b1: %+v", listA)
	}
	// (iii) vice-versa: B's scope holds prj-b1 and NEVER prj-a1.
	listB, err := repo.ListProjectsForAccount(ctx, "acct-b")
	if err != nil {
		t.Fatalf("ListProjectsForAccount B: %v", err)
	}
	if !containsProject(listB, "prj-b1") || containsProject(listB, "prj-a1") {
		t.Fatalf("CROSS-TENANT LEAK (acct-b scope): got %+v", listB)
	}

	// get-by-id is tenant-scoped: A can get its own project...
	if got, err := repo.GetProjectForAccount(ctx, "acct-a", "prj-a1"); err != nil || got.ProjectID != "prj-a1" || got.AccountID != "acct-a" {
		t.Fatalf("GetProjectForAccount(acct-a, prj-a1): %+v err=%v", got, err)
	}
	// ...but MUST NOT get account B's project -> ErrNotFound (anti-enumeration §4.3).
	if _, err := repo.GetProjectForAccount(ctx, "acct-a", "prj-b1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CROSS-TENANT LEAK: GetProjectForAccount(acct-a, prj-b1) want ErrNotFound, got %v", err)
	}
	if _, err := repo.GetProjectForAccount(ctx, "acct-b", "prj-a1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CROSS-TENANT LEAK: GetProjectForAccount(acct-b, prj-a1) want ErrNotFound, got %v", err)
	}
	// An unknown project id -> ErrNotFound (same shape as a cross-tenant miss).
	if _, err := repo.GetProjectForAccount(ctx, "acct-a", "no-such"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectForAccount unknown id want ErrNotFound, got %v", err)
	}
}

func containsProject(ps []Project, id string) bool {
	for _, p := range ps {
		if p.ProjectID == id {
			return true
		}
	}
	return false
}

// TestMemoryCrossTenantIsolation is the in-suite flagship for the §0 invariant on
// the in-memory backend (the default, RLS-less store — its isolation rests
// ENTIRELY on this scope). Runs under plain `go test ./...`.
func TestMemoryCrossTenantIsolation(t *testing.T) {
	runAccountContract(t, NewMemoryRepository(), time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
}
