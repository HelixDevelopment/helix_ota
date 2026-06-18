package fabric

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func fixedClock(ts time.Time) Clock { return func() time.Time { return ts } }

func newTestRegistry(ts time.Time) *Registry {
	return New(store.NewMemoryRepository(), fixedClock(ts))
}

func TestRegistryRegisterAndList(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)

	if err := g.RegisterNode(ctx, store.FabricNode{NodeID: "n1", Kind: "ci-linux-kvm", Arch: "x86_64", HasKVM: true}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if err := g.RegisterNode(ctx, store.FabricNode{Kind: "x"}); err == nil {
		t.Fatalf("RegisterNode missing fields should error")
	}
	if err := g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T2", Tech: "cuttlefish", Exclusive: true, NodeID: "n1"}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	if err := g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t2", Tier: "T0", Tech: "ota-device-emulator", Exclusive: true}); err != nil {
		t.Fatalf("RegisterTarget t2: %v", err)
	}
	targets, err := g.Targets(ctx)
	if err != nil || len(targets) != 2 {
		t.Fatalf("Targets want 2: %d err=%v", len(targets), err)
	}
	// Defaults applied.
	if targets[0].OSType != "android" || targets[0].Status != "idle" || targets[0].CreatedAt != ts {
		t.Fatalf("RegisterTarget defaults not applied: %+v", targets[0])
	}
}

func TestRegistryLeaseExclusivity(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)
	_ = g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T2", Tech: "cuttlefish", Exclusive: true})

	if _, err := g.AcquireLease(ctx, "l1", "t1", "run-A"); err != nil {
		t.Fatalf("first AcquireLease: %v", err)
	}
	// Second lease on the same exclusive target -> ErrTargetLeased (§11.4.119).
	if _, err := g.AcquireLease(ctx, "l2", "t1", "run-B"); !errors.Is(err, ErrTargetLeased) {
		t.Fatalf("double lease want ErrTargetLeased, got %v", err)
	}
	// Unknown target -> store.ErrNotFound surfaced.
	if _, err := g.AcquireLease(ctx, "lx", "nope", "run-X"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown target want ErrNotFound, got %v", err)
	}
	al, err := g.ActiveLease(ctx, "t1")
	if err != nil || al.Owner != "run-A" {
		t.Fatalf("ActiveLease: %+v err=%v", al, err)
	}
	// Release frees the slot; re-acquire succeeds.
	if err := g.ReleaseLease(ctx, "l1"); err != nil {
		t.Fatalf("ReleaseLease: %v", err)
	}
	if _, err := g.AcquireLease(ctx, "l3", "t1", "run-C"); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
}

func TestRegistryRunAndEvidence(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)
	_ = g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T0", Tech: "ota-device-emulator", Exclusive: true})

	run, err := g.RecordRun(ctx, "r1", "t1", "e2e", "tests/emulator/lifecycle.sh")
	if err != nil || run.Verdict != "PENDING" {
		t.Fatalf("RecordRun: %+v err=%v", run, err)
	}
	if _, err := g.RecordRun(ctx, "rbad", "no-target", "unit", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("RecordRun unknown target want ErrNotFound, got %v", err)
	}

	// AttachEvidence rejects empty, accepts non-empty.
	if err := g.AttachEvidence(ctx, "e0", "r1", "transcript", "docs/qa/r1/t.log", 0, ""); !errors.Is(err, ErrEmptyEvidence) {
		t.Fatalf("0-byte evidence want ErrEmptyEvidence, got %v", err)
	}
	if err := g.AttachEvidence(ctx, "e1", "r1", "ab-slot", "docs/qa/r1/slot.json", 2048, "sha"); err != nil {
		t.Fatalf("AttachEvidence: %v", err)
	}
	evs, err := g.Evidence(ctx, "r1")
	if err != nil || len(evs) != 1 || evs[0].ByteSize != 2048 {
		t.Fatalf("Evidence: %+v err=%v", evs, err)
	}

	// CompleteRun with evidence-backed PASS.
	done, err := g.CompleteRun(ctx, "r1", "PASS", "")
	if err != nil || done.Verdict != "PASS" || done.EndedAt == nil {
		t.Fatalf("CompleteRun PASS: %+v err=%v", done, err)
	}
	// SKIP requires a reason (§11.4.3).
	_, _ = g.RecordRun(ctx, "r2", "t1", "unit", "x")
	if _, err := g.CompleteRun(ctx, "r2", "SKIP", ""); err == nil {
		t.Fatalf("CompleteRun SKIP without reason should error")
	}
	if sk, err := g.CompleteRun(ctx, "r2", "SKIP", "topology_unsupported"); err != nil || sk.SkipReason != "topology_unsupported" {
		t.Fatalf("CompleteRun SKIP with reason: %+v err=%v", sk, err)
	}
	if _, err := g.CompleteRun(ctx, "ghost", "PASS", ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("CompleteRun unknown run want ErrNotFound, got %v", err)
	}
}
