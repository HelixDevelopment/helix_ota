package store

// SRV-NEW-1 in-suite proof (runs under plain `go test ./...`, NO build tag).
//
// These tests drive the transport-independent migration logic — the registry
// invariants (validateMigrations), the applied-vs-pending computation
// (pendingMigrations), and the ordered/idempotent apply loop (applyMigrations)
// — against a fake in-memory executor, so the ordering + idempotency + ledger
// semantics are proven WITHOUT a real database.
//
// What runs where (§11.4.3 / §11.4.6 honest split):
//   - THIS file (in-suite, plain `go test`): registry ordering/no-gaps/no-dup,
//     applies only pending in ascending order, idempotent re-run, resume from a
//     partially-applied ledger, unknown-newer-version rejection, atomic
//     stop-on-failure. The fake executor models the atomic "run SQL + record
//     ledger" step; the real per-migration transaction is NOT exercised here.
//   - postgres_migrations_integration_test.go (-tags integration + live
//     Postgres via the containers submodule): the REAL pgx tx-per-migration and
//     the real schema_migrations ledger table. That path cannot run in this
//     environment (no container runtime), so it is tagged and stated honestly.
//
// Anti-tautology (§11.4.115): TestApplyMigrations_IdempotentReRun and
// TestApplyMigrations_AppliesPendingInOrder are the load-bearing assertions.
// Temporarily breaking the invariant in pendingMigrations (drop the
// `!applied[...]` skip so applied migrations are re-returned) makes the
// idempotency assertion FAIL; restoring it makes it PASS again — proving the
// test genuinely catches the defect and is not a tautology.

import (
	"context"
	"fmt"
	"testing"
)

// fakeExecutor is an in-memory migrationExecutor: its map stands in for the
// schema_migrations ledger and it records the exact order applyOne was called.
type fakeExecutor struct {
	ledger     map[int64]bool // versions "recorded" in the ledger
	applyOrder []int64        // versions applyOne was invoked for, in call order
	failOn     int64          // if non-zero, applyOne errors for this version (SQL "ran" but nothing is recorded)
}

func newFakeExecutor() *fakeExecutor { return &fakeExecutor{ledger: map[int64]bool{}} }

func (f *fakeExecutor) appliedVersions(_ context.Context) (map[int64]bool, error) {
	out := make(map[int64]bool, len(f.ledger))
	for v := range f.ledger {
		out[v] = true
	}
	return out, nil
}

func (f *fakeExecutor) applyOne(_ context.Context, m migration) error {
	f.applyOrder = append(f.applyOrder, m.Version)
	if f.failOn == m.Version {
		return fmt.Errorf("fake: forced failure on migration %d", m.Version)
	}
	f.ledger[m.Version] = true // atomic in the real path (SQL + INSERT in one tx)
	return nil
}

func threeMigrations() []migration {
	return []migration{
		{Version: 1, Name: "baseline", SQL: "SELECT 1"},
		{Version: 2, Name: "add_widgets", SQL: "SELECT 2"},
		{Version: 3, Name: "add_gizmos", SQL: "SELECT 3"},
	}
}

func TestValidateMigrations_RealRegistryIsValid(t *testing.T) {
	if err := validateMigrations(registeredMigrations); err != nil {
		t.Fatalf("the shipped registeredMigrations must validate: %v", err)
	}
	if registeredMigrations[0].Version != 1 || registeredMigrations[0].Name != "baseline" {
		t.Fatalf("migration 1 must be the baseline; got %+v", registeredMigrations[0])
	}
	if registeredMigrations[0].SQL != postgresSchema {
		t.Fatal("baseline migration SQL must be the embedded schema_postgres.sql (postgresSchema)")
	}
}

func TestValidateMigrations_RejectsBadRegistries(t *testing.T) {
	cases := []struct {
		name string
		ms   []migration
	}{
		{"empty", nil},
		{"does-not-start-at-1", []migration{{Version: 2, Name: "x", SQL: "S"}}},
		{"gap", []migration{
			{Version: 1, Name: "a", SQL: "S"},
			{Version: 3, Name: "c", SQL: "S"}, // gap: 2 missing
		}},
		{"duplicate", []migration{
			{Version: 1, Name: "a", SQL: "S"},
			{Version: 1, Name: "b", SQL: "S"}, // dup breaks the i+1 sequence
		}},
		{"out-of-order", []migration{
			{Version: 2, Name: "b", SQL: "S"},
			{Version: 1, Name: "a", SQL: "S"},
		}},
		{"empty-name", []migration{{Version: 1, Name: "", SQL: "S"}}},
		{"empty-sql", []migration{{Version: 1, Name: "a", SQL: ""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateMigrations(tc.ms); err == nil {
				t.Fatalf("validateMigrations must reject %s registry", tc.name)
			}
		})
	}
}

func TestApplyMigrations_AppliesPendingInOrder(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor()
	done, err := applyMigrations(ctx, threeMigrations(), ex)
	if err != nil {
		t.Fatalf("applyMigrations: %v", err)
	}
	want := []int64{1, 2, 3}
	assertEqualInts(t, "applied-this-run", done, want)
	assertEqualInts(t, "call-order", ex.applyOrder, want)
	for _, v := range want {
		if !ex.ledger[v] {
			t.Fatalf("ledger must record version %d", v)
		}
	}
	if len(ex.ledger) != 3 {
		t.Fatalf("ledger must hold exactly 3 rows, got %d", len(ex.ledger))
	}
}

func TestApplyMigrations_IdempotentReRun(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor()
	if _, err := applyMigrations(ctx, threeMigrations(), ex); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Second run against the same (now fully-migrated) ledger MUST apply nothing.
	done, err := applyMigrations(ctx, threeMigrations(), ex)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(done) != 0 {
		t.Fatalf("idempotent re-run must apply nothing, applied %v", done)
	}
	// applyOne must NOT have been called again — total calls stay at 3 (each
	// migration applied at most once across both runs).
	assertEqualInts(t, "total-call-order", ex.applyOrder, []int64{1, 2, 3})
}

func TestApplyMigrations_ResumesFromPartialLedger(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor()
	ex.ledger[1] = true // migration 1 already applied by a previous run
	done, err := applyMigrations(ctx, threeMigrations(), ex)
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	assertEqualInts(t, "resumed-applied", done, []int64{2, 3})
	assertEqualInts(t, "resumed-call-order", ex.applyOrder, []int64{2, 3})
}

func TestApplyMigrations_RejectsUnknownNewerLedgerVersion(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor()
	ex.ledger[99] = true // ledger recorded a version this binary does not know
	if _, err := applyMigrations(ctx, threeMigrations(), ex); err == nil {
		t.Fatal("applyMigrations must refuse a ledger with an unknown (newer) version")
	}
}

func TestApplyMigrations_StopsAtomicallyOnFailure(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor()
	ex.failOn = 2
	done, err := applyMigrations(ctx, threeMigrations(), ex)
	if err == nil {
		t.Fatal("applyMigrations must return the applyOne failure")
	}
	assertEqualInts(t, "applied-before-failure", done, []int64{1})
	// 1 applied+recorded, 2 attempted+failed (not recorded), 3 never attempted.
	assertEqualInts(t, "call-order-until-failure", ex.applyOrder, []int64{1, 2})
	if ex.ledger[2] {
		t.Fatal("a failed migration must NOT be recorded in the ledger (atomicity)")
	}
	if ex.ledger[3] {
		t.Fatal("migration 3 must not be applied after 2 failed")
	}
}

func assertEqualInts(t *testing.T, label string, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len=%d %v, want len=%d %v", label, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: at %d got %d, want %d (%v vs %v)", label, i, got[i], want[i], got, want)
		}
	}
}
