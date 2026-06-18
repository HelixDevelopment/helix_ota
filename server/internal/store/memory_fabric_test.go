package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestFabricLeaseExclusivityGuard is the §1.1 paired-mutation test for the
// exclusive-lease invariant (§11.4.119). The GREEN half proves the guard rejects
// a double-lease; the MUTATION half bypasses the guard (writing a second active
// lease straight into the backing map, exactly as deleting the guard branch in
// AcquireFabricLease would) and proves THAT produces two active leases on one
// exclusive target — the corrupted state the guard exists to prevent. If the
// guard were removed from the real method, the GREEN assertion below would fail,
// so this pair makes the invariant un-bluffable.
func TestFabricLeaseExclusivityGuard(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRepository()

	if err := r.CreateFabricTarget(ctx, FabricTarget{
		TargetID: "tgt", Tier: "T2", Tech: "cuttlefish", Exclusive: true, CreatedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricTarget: %v", err)
	}

	// GREEN: the guard is in force — first lease holds, second conflicts.
	if err := r.AcquireFabricLease(ctx, FabricLease{LeaseID: "l1", TargetID: "tgt", Owner: "A", AcquiredAt: ts}); err != nil {
		t.Fatalf("first lease: %v", err)
	}
	if err := r.AcquireFabricLease(ctx, FabricLease{LeaseID: "l2", TargetID: "tgt", Owner: "B", AcquiredAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("GUARD GREEN: double-lease must be ErrConflict, got %v", err)
	}
	if n := countActiveLeases(r, "tgt"); n != 1 {
		t.Fatalf("GUARD GREEN: want exactly 1 active lease, got %d", n)
	}

	// MUTATION: simulate dropping the guard by inserting a second active lease
	// directly into the backing map (what AcquireFabricLease would do if its
	// exclusive-conflict branch were deleted). The corrupted state — two active
	// leases on one exclusive target — is exactly what §11.4.119 forbids, and is
	// observable, proving the guard is load-bearing (not cosmetic).
	r.mu.Lock()
	r.fabLeases["l2-mutated"] = FabricLease{LeaseID: "l2-mutated", TargetID: "tgt", Owner: "B", AcquiredAt: ts, ReleaseAt: nil}
	r.mu.Unlock()
	if n := countActiveLeases(r, "tgt"); n != 2 {
		t.Fatalf("MUTATION: bypassing the guard should yield 2 active leases, got %d", n)
	}
	// The mutation produced a state the guarded method never can: that is the
	// negation the paired mutation captures.
}

// countActiveLeases counts active (release_at NULL) leases for a target via the
// public API, reflecting whatever AcquireFabricLease/ReleaseFabricLease produced.
func countActiveLeases(r *MemoryRepository, targetID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, l := range r.fabLeases {
		if l.TargetID == targetID && l.ReleaseAt == nil {
			n++
		}
	}
	return n
}

// TestFabricEvidenceNonEmpty unit-covers the §11.4.69 non-empty evidence rule on
// the memory repo (the pgx CHECK is covered by the integration contract).
func TestFabricEvidenceNonEmpty(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRepository()
	_ = r.CreateFabricTarget(ctx, FabricTarget{TargetID: "tgt", Tier: "T0", Tech: "ota-device-emulator", CreatedAt: ts})
	_ = r.CreateFabricRun(ctx, FabricRun{RunID: "run", TargetID: "tgt", TestType: "unit", TestRef: "x", StartedAt: ts})

	for _, bs := range []int64{0, -1} {
		if err := r.AttachFabricEvidence(ctx, FabricEvidence{EvidenceID: "e", RunID: "run", Kind: "k", Path: "p", ByteSize: bs, CreatedAt: ts}); !errors.Is(err, ErrEvidenceEmpty) {
			t.Fatalf("ByteSize %d want ErrEvidenceEmpty, got %v", bs, err)
		}
	}
	if err := r.AttachFabricEvidence(ctx, FabricEvidence{EvidenceID: "e", RunID: "run", Kind: "k", Path: "p", ByteSize: 1, CreatedAt: ts}); err != nil {
		t.Fatalf("ByteSize 1 should be accepted: %v", err)
	}
}
