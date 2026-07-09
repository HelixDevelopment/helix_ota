// Package devclaim is a minimal Helix OTA dev-tooling prototype that models the
// project's real parallel-device-testing case on top of the session_orchestrator
// reusable engine's claim + scheduler primitives.
//
// # The Helix OTA problem it models
//
// Helix OTA runs parallel device tests (§11.4.58 / §11.4.103). Each physical
// target device (RK3588 / Orange Pi 5 Max, reached by USB/wireless-ADB serial)
// is an EXCLUSIVE resource: while one test worker is driving an update /
// flashing / running an on-device probe against a device, no second worker may
// touch that same device — two concurrent drivers of one device produce
// CROSS-CONTAMINATED evidence (a §11.4.119 single-resource-owner violation, a
// §11.4 evidence-integrity bluff). Helix therefore needs, per constitution, an
// exactly-once work-claim registry (§11.4.176) + a single-owner device-lock
// (§11.4.119) so that at most one worker owns each device at a time, releasing
// on done, and a dead worker's lock is reclaimable (TTL / liveness reap).
//
// # What this prototype does
//
// It wraps the engine's claim.Registry (the exactly-once, deadlock-free,
// single-owner TryClaim CAS + TTL/dead-holder reap) as a DEVICE-claim registry:
//   - the exclusive resource is a physical device addressed by its STABLE SERIAL
//     (§11.4.111 resolve-by-stable-name-not-by-enumeration-index — never an ADB
//     enumeration slot / `adb devices` ordinal, which is reassigned on hotplug);
//   - the holder is an opaque test-worker id.
//
// It composes, does not re-implement: the exactly-once / single-owner / non-
// blocking / TTL-reap semantics all come from claim.Registry — this package adds
// only the Helix-domain naming (device serial, worker) + the stable-serial guard
// + a claimable-device selection helper built on the engine's scheduler.
//
// Decoupling (§11.4.28): this is dev-tooling in the consumer tree; the engine
// stays project-unaware and is consumed BY REFERENCE via a go.mod replace.
//
// Honest boundary (§11.4.6 / adoption plan §2): the engine's same-session
// FAILOVER / re-homing spine (WS-C — detect a degraded worker → atomically move
// its work onto another) is UNCONFIRMED / NOT implemented in the engine. This
// prototype therefore models claim / release / single-owner / dead-worker-reap
// ONLY; automatic re-homing of a crashed worker's in-flight test remains
// conductor-driven (§11.4.147) on top of the dead-holder reap this exposes.
package devclaim

import (
	"errors"
	"time"

	"github.com/vasic-digital/session_orchestrator/alias"
	"github.com/vasic-digital/session_orchestrator/claim"
	"github.com/vasic-digital/session_orchestrator/scheduler"
)

// ErrEmptySerial is returned when a device serial is blank. A device MUST be
// addressed by a stable, non-empty serial (§11.4.111), never a positional index.
var ErrEmptySerial = errors.New("devclaim: device serial must be non-empty (stable id, not an enumeration index — §11.4.111)")

// ErrEmptyWorker is returned when a worker id is blank.
var ErrEmptyWorker = errors.New("devclaim: worker id must be non-empty")

// errDeviceUnreachable is the internal sentinel a PlaceWorkers probe returns for
// an unreachable device, so the engine classifies it UNREACHABLE (fail-closed,
// §11.4.69) instead of ever treating an unprobed device as healthy.
var errDeviceUnreachable = errors.New("devclaim: device unreachable")

// WorkerLiveness reports whether a test worker is PROVABLY still alive. It is
// injected (§11.4.28): the consumer supplies its own fast, non-blocking proof —
// e.g. `kill -0` on the worker's supervising pid, or a heartbeat-file mtime
// check. A nil WorkerLiveness means liveness cannot be proven, so a device lock
// is reaped by TTL only (§11.4.6 — a missing proof never becomes "assume dead").
type WorkerLiveness func(worker string) bool

// Config carries the injected dependencies. All fields are optional.
type Config struct {
	// Now is the injected clock (§11.4.50). Nil uses the real wall clock.
	Now func() time.Time
	// Liveness proves a holding worker is still alive (dead-worker reap). Nil ⇒
	// pure-TTL lease (the worker MUST Renew before its TTL to keep the device).
	Liveness WorkerLiveness
}

// Lease is one exclusive hold of a device by a worker. The ClaimID must be
// presented to Release/Renew so a stale worker cannot release a device that was
// already reaped and re-granted to a different worker (§9.2).
type Lease struct {
	Serial    string        // the device's stable serial (the exclusive resource)
	Worker    string        // the worker that owns it
	ClaimID   string        // unique, minted once per grant
	GrantedAt time.Time     // instant the lock was granted
	TTL       time.Duration // reap horizon relative to GrantedAt
}

// Outcome mirrors the engine's non-erroring TryClaim verdict.
type Outcome = claim.Outcome

const (
	// OutcomeGranted — the device was free and is now exclusively this worker's.
	OutcomeGranted = claim.OutcomeGranted
	// OutcomeGrantedExisting — this worker already held the device (idempotent).
	OutcomeGrantedExisting = claim.OutcomeGrantedExisting
	// OutcomeDenied — the device is held by a DIFFERENT worker (clean rejection).
	OutcomeDenied = claim.OutcomeDenied
)

// Registry is the single-owner device-claim registry. At most one worker holds
// each device serial at a time (§11.4.119). It is safe for concurrent use.
type Registry struct {
	cr *claim.Registry
}

// New returns a device-claim registry wrapping a fresh engine claim.Registry
// with the injected clock + worker-liveness proof.
func New(cfg Config) *Registry {
	return &Registry{cr: claim.New(claim.Config{
		Now:      cfg.Now,
		Liveness: liveness(cfg.Liveness),
	})}
}

// liveness adapts a WorkerLiveness (nil-safe) to the engine's claim.Liveness.
func liveness(w WorkerLiveness) claim.Liveness {
	if w == nil {
		return nil
	}
	return func(holder string) bool { return w(holder) }
}

// TryAcquire attempts to give worker exclusive ownership of the device with the
// given stable serial, for at most ttl (a live worker keeps it past ttl via
// Renew or a Liveness proof). It NEVER blocks (§11.4.176): it returns GRANTED
// (device was free), GRANTED_EXISTING (this worker already held it — exactly-
// once idempotent), or DENIED (a different worker holds it). Stale locks (TTL
// elapsed or holder provably dead) are reaped before the decision, so a device
// whose prior worker died is acquirable again.
func (r *Registry) TryAcquire(serial, worker string, ttl time.Duration) (Lease, Outcome, error) {
	if serial == "" {
		return Lease{}, "", ErrEmptySerial
	}
	if worker == "" {
		return Lease{}, "", ErrEmptyWorker
	}
	c, outcome, err := r.cr.TryClaim(serial, worker, ttl)
	if err != nil {
		return Lease{}, "", err
	}
	return leaseOf(c), outcome, nil
}

// Release frees worker's hold on the device. The claimID granted by TryAcquire
// MUST be passed; a mismatch is rejected (a stale worker cannot release a device
// already reaped and re-granted to another worker, §9.2).
func (r *Registry) Release(serial, claimID string) error {
	return r.cr.Release(serial, claimID)
}

// Renew is the explicit heartbeat: it extends the device lock's TTL window so a
// long test survives past the original horizon. The granted claimID MUST match.
func (r *Registry) Renew(serial, claimID string) (Lease, error) {
	c, err := r.cr.Renew(serial, claimID)
	if err != nil {
		return Lease{}, err
	}
	return leaseOf(c), nil
}

// Owner returns the worker currently holding the device (reaping any stale lock
// first, so a dead-worker/expired device reads as free), or ("", false).
func (r *Registry) Owner(serial string) (string, bool) {
	for _, c := range r.cr.Snapshot() {
		if c.ResourceID == serial {
			return c.Holder, true
		}
	}
	return "", false
}

// IsHeld reports whether the device currently has a live owner.
func (r *Registry) IsHeld(serial string) bool { return r.cr.IsClaimed(serial) }

// Reap sweeps every device and reclaims stale locks (TTL elapsed or worker
// provably dead), returning the reclaimed leases. A live-held device is never
// reaped.
func (r *Registry) Reap() []Lease {
	reaped := r.cr.Reap()
	out := make([]Lease, 0, len(reaped))
	for _, c := range reaped {
		out = append(out, leaseOf(c))
	}
	return out
}

// leaseOf projects an engine claim.Claim onto the Helix-domain Lease.
func leaseOf(c claim.Claim) Lease {
	return Lease{Serial: c.ResourceID, Worker: c.Holder, ClaimID: c.ClaimID, GrantedAt: c.GrantedAt, TTL: c.TTL}
}

// --- Multi-device placement (all-or-nothing, deadlock-free) ------------------

// PlaceWorkers assigns each worker (given in priority order — most-critical
// first) to exactly ONE free, reachable device from the pool, exclusively and
// exactly-once, using the engine scheduler. A device is a candidate only if it
// is REACHABLE (the consumer's reachable(serial) probe returns true — the
// device-testing analogue of alias health) AND not already claimed. A worker for
// which no reachable free device remains is returned Unassigned (never dropped,
// never double-assigned, never blocked — §11.4.176 / §11.4.69). It is a pure
// composition over the engine scheduler, so it inherits the scheduler's
// non-blocking, single-owner, deterministic-under-injected-clock guarantees.
//
// This is the "all-or-nothing multi-device claim" path: a storm of workers
// contending for a shared device pool completes without deadlock because every
// underlying acquire is the non-blocking TryClaim CAS (no worker ever waits
// holding a partial set of devices — the Coffman hold-and-wait condition is
// never entered).
func (r *Registry) PlaceWorkers(devices []string, workers []string, reachable func(serial string) bool, ttl time.Duration, now func() time.Time) (Placement, error) {
	reg := alias.NewRegistry()
	for i, serial := range devices {
		if serial == "" {
			return Placement{}, ErrEmptySerial
		}
		// Register each device as an alias keyed by its STABLE SERIAL (§11.4.111);
		// StableIndex gives a deterministic tie-break in registration order.
		if err := reg.Register(alias.Alias{Name: serial, StableIndex: i}); err != nil {
			return Placement{}, err
		}
	}
	res, err := scheduler.Schedule(reg, r.cr, workers, scheduler.Config{
		Now: now,
		TTL: ttl,
		Probe: func(a alias.Alias) alias.ProbeResult {
			// A reachable device is operable — expressed as the engine's positive-
			// evidence probe (HTTP 200 + VERIFY_OK token). An unreachable device is
			// fail-closed excluded (§11.4.69 no-fail-open — never assign an
			// unreachable device) via an error result that classifies UNREACHABLE.
			if reachable != nil && reachable(a.Name) {
				return alias.ProbeResult{HTTPStatus: 200, Body: alias.VerifyToken}
			}
			return alias.ProbeResult{Err: errDeviceUnreachable}
		},
	})
	if err != nil {
		return Placement{}, err
	}
	return placementOf(res), nil
}

// Assignment is the outcome for one worker in a PlaceWorkers pass.
type Assignment struct {
	Worker   string // the worker (echoed verbatim)
	Serial   string // the device serial claimed for it; "" when Unassigned
	ClaimID  string // the exclusive claim id; "" when Unassigned
	Assigned bool   // true when a reachable free device was claimed
}

// Placement is the full outcome of a PlaceWorkers pass — one Assignment per
// input worker, in input (priority) order.
type Placement struct {
	Assignments []Assignment
	Assigned    []string // worker ids that got a device (input order)
	Unassigned  []string // worker ids with no reachable free device (input order)
}

func placementOf(res scheduler.Result) Placement {
	p := Placement{
		Assignments: make([]Assignment, 0, len(res.Placements)),
		Assigned:    res.Assigned,
		Unassigned:  res.Unassigned,
	}
	for _, pl := range res.Placements {
		p.Assignments = append(p.Assignments, Assignment{
			Worker: pl.WorkUnit, Serial: pl.Alias, ClaimID: pl.ClaimID, Assigned: pl.Assigned,
		})
	}
	return p
}
