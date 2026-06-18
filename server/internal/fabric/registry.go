// Package fabric is the Helix OTA emulation test-fabric REGISTRY: a small API
// over the store.Repository seam for registering fabric nodes/targets, leasing
// targets exclusively (§11.4.119), recording test runs, and attaching captured
// evidence (§11.4.69). It is derived from docs/design/emulation_fabric/SCHEMA.sql
// + DESIGN.md §6.
//
// DESIGN REFINEMENT (§11.4.28): this registry is PROJECT-SPECIFIC to Helix OTA
// (its tier/tech/target vocabulary — T0/T1/T2/T3/Tfw/Tcp, rk3588, OrangePi5Max —
// and the ota-protocol PASS-evidence rule are OTA-domain knowledge), so it lives
// in this project, NOT in the reusable vasic-digital/containers submodule's
// pkg/fabric. The generic scheduler/lease façade may still live upstream in
// containers; the OTA-shaped registry is modelled here on the existing pgx/memory
// Repository. See the matching note in server/internal/store/store.go.
//
// The Registry is pure-ish: it owns no I/O of its own — every read/write goes
// through the injected store.Repository (so it is testable against the in-memory
// repo and runs unchanged against pgx).
package fabric

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// Clock returns the current time; injectable for deterministic tests (§11.4.50).
type Clock func() time.Time

// Registry is the fabric target registry over a store.Repository.
type Registry struct {
	repo store.Repository
	now  Clock
}

// New constructs a Registry backed by repo. now defaults to time.Now when nil.
func New(repo store.Repository, now Clock) *Registry {
	if now == nil {
		now = time.Now
	}
	return &Registry{repo: repo, now: now}
}

// Sentinel errors surfaced by the registry (wrap the store sentinels so callers
// can errors.Is on either layer).
var (
	// ErrTargetLeased indicates an exclusive target is already held (§11.4.119).
	ErrTargetLeased = errors.New("fabric: target already leased")
	// ErrEmptyEvidence indicates a rejected 0-byte evidence artefact (§11.4.69).
	ErrEmptyEvidence = errors.New("fabric: evidence artefact is empty")
)

// RegisterNode upserts a fabric execution node. node_id/kind/arch are required.
func (g *Registry) RegisterNode(ctx context.Context, n store.FabricNode) error {
	if n.NodeID == "" || n.Kind == "" || n.Arch == "" {
		return fmt.Errorf("fabric: node requires node_id, kind, arch")
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = g.now()
	}
	if n.LastSeenAt.IsZero() {
		n.LastSeenAt = n.CreatedAt
	}
	return g.repo.CreateFabricNode(ctx, n)
}

// RegisterTarget upserts a fabric target. target_id/tier/tech are required.
func (g *Registry) RegisterTarget(ctx context.Context, t store.FabricTarget) error {
	if t.TargetID == "" || t.Tier == "" || t.Tech == "" {
		return fmt.Errorf("fabric: target requires target_id, tier, tech")
	}
	if t.OSType == "" {
		t.OSType = "android"
	}
	if t.Status == "" {
		t.Status = "idle"
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = g.now()
	}
	return g.repo.CreateFabricTarget(ctx, t)
}

// Targets lists every registered target (fleet-of-fabric view, DESIGN.md §6).
func (g *Registry) Targets(ctx context.Context) ([]store.FabricTarget, error) {
	return g.repo.ListFabricTargets(ctx)
}

// AcquireLease leases targetID exclusively to owner and returns the lease id
// (§11.4.119). It fails with ErrTargetLeased if an exclusive target is already
// held, and store.ErrNotFound if the target is unknown.
func (g *Registry) AcquireLease(ctx context.Context, leaseID, targetID, owner string) (store.FabricLease, error) {
	if leaseID == "" || targetID == "" || owner == "" {
		return store.FabricLease{}, fmt.Errorf("fabric: lease requires lease_id, target_id, owner")
	}
	l := store.FabricLease{LeaseID: leaseID, TargetID: targetID, Owner: owner, AcquiredAt: g.now()}
	if err := g.repo.AcquireFabricLease(ctx, l); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return store.FabricLease{}, ErrTargetLeased
		}
		return store.FabricLease{}, err
	}
	return l, nil
}

// ReleaseLease releases a previously-acquired lease (idempotent).
func (g *Registry) ReleaseLease(ctx context.Context, leaseID string) error {
	if leaseID == "" {
		return fmt.Errorf("fabric: release requires lease_id")
	}
	return g.repo.ReleaseFabricLease(ctx, leaseID, g.now())
}

// ActiveLease returns the lease currently holding a target, or store.ErrNotFound.
func (g *Registry) ActiveLease(ctx context.Context, targetID string) (store.FabricLease, error) {
	return g.repo.ActiveFabricLease(ctx, targetID)
}

// RecordRun creates a test run against targetID and returns it. verdict defaults
// to PENDING; complete it later with CompleteRun.
func (g *Registry) RecordRun(ctx context.Context, runID, targetID, testType, testRef string) (store.FabricRun, error) {
	if runID == "" || targetID == "" || testType == "" || testRef == "" {
		return store.FabricRun{}, fmt.Errorf("fabric: run requires run_id, target_id, test_type, test_ref")
	}
	run := store.FabricRun{
		RunID: runID, TargetID: targetID, TestType: testType, TestRef: testRef,
		Verdict: "PENDING", StartedAt: g.now(),
	}
	if err := g.repo.CreateFabricRun(ctx, run); err != nil {
		return store.FabricRun{}, err
	}
	return run, nil
}

// CompleteRun sets a run's terminal verdict + ended_at. verdict is one of the
// closed §11.4.45 vocabulary; skipReason is required (non-empty) when verdict is
// SKIP (§11.4.3/§11.4.69) and ignored otherwise.
func (g *Registry) CompleteRun(ctx context.Context, runID, verdict, skipReason string) (store.FabricRun, error) {
	run, err := g.repo.GetFabricRun(ctx, runID)
	if err != nil {
		return store.FabricRun{}, err
	}
	if verdict == "SKIP" && skipReason == "" {
		return store.FabricRun{}, fmt.Errorf("fabric: SKIP verdict requires a skip_reason (§11.4.3)")
	}
	end := g.now()
	run.Verdict = verdict
	run.SkipReason = skipReason
	run.EndedAt = &end
	if err := g.repo.UpdateFabricRun(ctx, run); err != nil {
		return store.FabricRun{}, err
	}
	return run, nil
}

// AttachEvidence attaches a captured evidence artefact to a run (§11.4.69). It
// rejects an empty/0-byte artefact with ErrEmptyEvidence — a PASS may never cite
// a 0-byte proof. byteSize MUST be the real captured size of path.
func (g *Registry) AttachEvidence(ctx context.Context, evidenceID, runID, kind, path string, byteSize int64, sha256 string) error {
	if evidenceID == "" || runID == "" || kind == "" || path == "" {
		return fmt.Errorf("fabric: evidence requires evidence_id, run_id, kind, path")
	}
	if byteSize <= 0 {
		return ErrEmptyEvidence
	}
	e := store.FabricEvidence{
		EvidenceID: evidenceID, RunID: runID, Kind: kind, Path: path,
		ByteSize: byteSize, SHA256: sha256, CreatedAt: g.now(),
	}
	if err := g.repo.AttachFabricEvidence(ctx, e); err != nil {
		if errors.Is(err, store.ErrEvidenceEmpty) {
			return ErrEmptyEvidence
		}
		return err
	}
	return nil
}

// Evidence lists a run's captured evidence in insertion order.
func (g *Registry) Evidence(ctx context.Context, runID string) ([]store.FabricEvidence, error) {
	return g.repo.ListFabricEvidence(ctx, runID)
}
