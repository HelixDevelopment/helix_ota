package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// runFabricContract exercises the emulation test-fabric registry contract every
// Repository implementation MUST satisfy (docs/design/emulation_fabric/SCHEMA.sql):
// node/target registration, the exclusive-lease invariant (§11.4.119 — at most
// one ACTIVE lease per exclusive target), run lifecycle, and the non-empty
// evidence rule (§11.4.69 — a 0-byte artefact is rejected). It runs against
// memory here and pgx in the integration test, proving parity.
func runFabricContract(t *testing.T, repo Repository, ts time.Time) {
	t.Helper()
	ctx := context.Background()

	// --- node ---
	node := FabricNode{
		NodeID: "node-1", Kind: "ci-linux-kvm", Arch: "x86_64", HasKVM: true,
		Labels: map[string]string{"pool": "ci"}, LastSeenAt: ts, CreatedAt: ts,
	}
	if err := repo.CreateFabricNode(ctx, node); err != nil {
		t.Fatalf("CreateFabricNode: %v", err)
	}
	if gn, err := repo.GetFabricNode(ctx, "node-1"); err != nil ||
		gn.Kind != "ci-linux-kvm" || !gn.HasKVM || gn.Labels["pool"] != "ci" {
		t.Fatalf("GetFabricNode round-trip: %+v err=%v", gn, err)
	}
	if _, err := repo.GetFabricNode(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFabricNode unknown want ErrNotFound, got %v", err)
	}

	// --- exclusive target ---
	tgt := FabricTarget{
		TargetID: "tgt-cf-1", Tier: "T2", Tech: "cuttlefish", Model: "aosp_cf_arm64",
		OSType: "android", Exclusive: true, NodeID: "node-1", CreatedAt: ts,
	}
	if err := repo.CreateFabricTarget(ctx, tgt); err != nil {
		t.Fatalf("CreateFabricTarget: %v", err)
	}
	if gt, err := repo.GetFabricTarget(ctx, "tgt-cf-1"); err != nil ||
		gt.Tier != "T2" || gt.Tech != "cuttlefish" || !gt.Exclusive || gt.NodeID != "node-1" {
		t.Fatalf("GetFabricTarget round-trip: %+v err=%v", gt, err)
	}
	if _, err := repo.GetFabricTarget(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFabricTarget unknown want ErrNotFound, got %v", err)
	}
	// A target with an empty (unbound) node id round-trips with NodeID == "".
	if err := repo.CreateFabricTarget(ctx, FabricTarget{
		TargetID: "tgt-t0", Tier: "T0", Tech: "ota-device-emulator", Exclusive: true,
		CreatedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricTarget unbound: %v", err)
	}
	if gt, err := repo.GetFabricTarget(ctx, "tgt-t0"); err != nil || gt.NodeID != "" {
		t.Fatalf("unbound target NodeID want empty: %+v err=%v", gt, err)
	}
	if targets, err := repo.ListFabricTargets(ctx); err != nil || len(targets) != 2 {
		t.Fatalf("ListFabricTargets want 2, got %d err=%v", len(targets), err)
	}

	// --- exclusive lease invariant (§11.4.119) ---
	if err := repo.AcquireFabricLease(ctx, FabricLease{
		LeaseID: "lease-1", TargetID: "tgt-cf-1", Owner: "run-A", AcquiredAt: ts,
	}); err != nil {
		t.Fatalf("AcquireFabricLease first: %v", err)
	}
	// A SECOND active lease on the same exclusive target MUST conflict.
	if err := repo.AcquireFabricLease(ctx, FabricLease{
		LeaseID: "lease-2", TargetID: "tgt-cf-1", Owner: "run-B", AcquiredAt: ts,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("double-lease on exclusive target want ErrConflict, got %v", err)
	}
	if al, err := repo.ActiveFabricLease(ctx, "tgt-cf-1"); err != nil || al.LeaseID != "lease-1" || al.Owner != "run-A" {
		t.Fatalf("ActiveFabricLease: %+v err=%v", al, err)
	}
	// A lease against an unknown target is ErrNotFound.
	if err := repo.AcquireFabricLease(ctx, FabricLease{
		LeaseID: "lease-x", TargetID: "no-target", Owner: "run-X", AcquiredAt: ts,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("lease on unknown target want ErrNotFound, got %v", err)
	}
	// Release then re-acquire succeeds (the slot frees).
	if err := repo.ReleaseFabricLease(ctx, "lease-1", ts.Add(time.Minute)); err != nil {
		t.Fatalf("ReleaseFabricLease: %v", err)
	}
	if _, err := repo.ActiveFabricLease(ctx, "tgt-cf-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ActiveFabricLease after release want ErrNotFound, got %v", err)
	}
	// Releasing again is idempotent.
	if err := repo.ReleaseFabricLease(ctx, "lease-1", ts.Add(2*time.Minute)); err != nil {
		t.Fatalf("ReleaseFabricLease idempotent: %v", err)
	}
	if err := repo.ReleaseFabricLease(ctx, "no-lease", ts); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReleaseFabricLease unknown want ErrNotFound, got %v", err)
	}
	if err := repo.AcquireFabricLease(ctx, FabricLease{
		LeaseID: "lease-3", TargetID: "tgt-cf-1", Owner: "run-C", AcquiredAt: ts.Add(3 * time.Minute),
	}); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}

	// --- run lifecycle ---
	if err := repo.CreateFabricRun(ctx, FabricRun{
		RunID: "run-1", TargetID: "tgt-cf-1", TestType: "e2e",
		TestRef: "tests/emulator/lifecycle.sh", StartedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricRun: %v", err)
	}
	if gr, err := repo.GetFabricRun(ctx, "run-1"); err != nil || gr.Verdict != "PENDING" || gr.TestType != "e2e" {
		t.Fatalf("GetFabricRun default verdict: %+v err=%v", gr, err)
	}
	if err := repo.CreateFabricRun(ctx, FabricRun{
		RunID: "run-bad", TargetID: "no-target", TestType: "unit", TestRef: "x", StartedAt: ts,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateFabricRun unknown target want ErrNotFound, got %v", err)
	}
	ended := ts.Add(5 * time.Minute)
	if err := repo.UpdateFabricRun(ctx, FabricRun{
		RunID: "run-1", TargetID: "tgt-cf-1", TestType: "e2e",
		TestRef: "tests/emulator/lifecycle.sh", Verdict: "PASS", StartedAt: ts, EndedAt: &ended,
	}); err != nil {
		t.Fatalf("UpdateFabricRun: %v", err)
	}
	if gr, err := repo.GetFabricRun(ctx, "run-1"); err != nil || gr.Verdict != "PASS" ||
		gr.EndedAt == nil || !gr.EndedAt.Equal(ended) {
		t.Fatalf("UpdateFabricRun not applied: %+v err=%v", gr, err)
	}
	if err := repo.UpdateFabricRun(ctx, FabricRun{RunID: "ghost", TargetID: "tgt-cf-1",
		TestType: "unit", TestRef: "x", StartedAt: ts}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateFabricRun unknown want ErrNotFound, got %v", err)
	}

	// --- evidence ledger (§11.4.69 non-empty rule) ---
	if err := repo.AttachFabricEvidence(ctx, FabricEvidence{
		EvidenceID: "ev-1", RunID: "run-1", Kind: "transcript",
		Path: "docs/qa/run-1/transcript.log", ByteSize: 0, CreatedAt: ts,
	}); !errors.Is(err, ErrEvidenceEmpty) {
		t.Fatalf("0-byte evidence want ErrEvidenceEmpty, got %v", err)
	}
	if err := repo.AttachFabricEvidence(ctx, FabricEvidence{
		EvidenceID: "ev-2", RunID: "run-1", Kind: "ab-slot",
		Path: "docs/qa/run-1/slot.json", ByteSize: 4096, SHA256: "abc", CreatedAt: ts,
	}); err != nil {
		t.Fatalf("AttachFabricEvidence non-empty: %v", err)
	}
	if err := repo.AttachFabricEvidence(ctx, FabricEvidence{
		EvidenceID: "ev-3", RunID: "ghost-run", Kind: "x", Path: "p", ByteSize: 1, CreatedAt: ts,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("evidence on unknown run want ErrNotFound, got %v", err)
	}
	evs, err := repo.ListFabricEvidence(ctx, "run-1")
	if err != nil || len(evs) != 1 || evs[0].EvidenceID != "ev-2" || evs[0].ByteSize != 4096 {
		t.Fatalf("ListFabricEvidence want 1 non-empty row: %+v err=%v", evs, err)
	}
	if evs, err := repo.ListFabricEvidence(ctx, "no-run"); err != nil || len(evs) != 0 {
		t.Fatalf("ListFabricEvidence empty want 0, got %d err=%v", len(evs), err)
	}
}
