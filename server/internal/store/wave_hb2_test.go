package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestAcquireFabricLease_HB2_UniformOneLeasePerTarget is the permanent HB-2
// guard (§11.4.115 GREEN polarity). Per the operator decision (2026-07-11) the
// in-memory backend now enforces one active lease per target UNIFORMLY —
// matching the production PostgresRepository — even for a NON-exclusive target,
// which previously allowed concurrent leases (a dev/prod divergence: code that
// passed against the memory store failed against pgx).
//
// Anti-tautology anchor: re-gating the conflict loop on `if tgt.Exclusive` lets
// a 2nd lease on a non-exclusive target succeed -> RED; restore -> GREEN.
func TestAcquireFabricLease_HB2_UniformOneLeasePerTarget(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	r := NewMemoryRepository()

	if err := r.CreateFabricTarget(ctx, FabricTarget{
		TargetID: "tgt-non-excl", Tier: "T0", Tech: "ota-device-emulator",
		Exclusive: false, Status: "idle", CreatedAt: ts,
	}); err != nil {
		t.Fatalf("CreateFabricTarget: %v", err)
	}

	if err := r.AcquireFabricLease(ctx, FabricLease{LeaseID: "l1", TargetID: "tgt-non-excl", Owner: "A", AcquiredAt: ts}); err != nil {
		t.Fatalf("first lease on non-exclusive target: %v", err)
	}
	// Second lease on the SAME non-exclusive target must ErrConflict (uniform
	// one-lease-per-target, matching production Postgres), NOT succeed.
	if err := r.AcquireFabricLease(ctx, FabricLease{LeaseID: "l2", TargetID: "tgt-non-excl", Owner: "B", AcquiredAt: ts}); !errors.Is(err, ErrConflict) {
		t.Fatalf("HB-2: a 2nd active lease on a non-exclusive target must be ErrConflict, got %v", err)
	}
}
