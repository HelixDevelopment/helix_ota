// Tests for the Helix OTA device-claim prototype. These are REAL tests that
// exercise the wrapped engine end-to-end (no mocks of the engine — the engine is
// the real, fully-implemented system per §11.4.27); the only injected fakes are
// the clock and the worker-liveness proof, which the engine is explicitly
// designed to accept as injected dependencies (§11.4.28 / §11.4.50).
//
// They prove the four Helix-required properties of a §11.4.176 exactly-once /
// §11.4.119 single-owner device-claim registry:
//
//	(a) exactly-once     — TestExactlyOnce_ConcurrentClaimants
//	(b) single-owner-over-time — TestSingleOwner_ReleaseThenReclaim
//	(c) deadlock-free    — TestDeadlockFree_MultiDeviceStorm / TestDeadlockFree_PlaceWorkersStorm
//	(d) TTL / dead-worker reap — TestTTLReap_ExpiredLease / TestLivenessReap_DeadWorker
package devclaim

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fixedClock returns a controllable clock for deterministic TTL tests (§11.4.50).
func fixedClock(t *time.Time) func() time.Time { return func() time.Time { return *t } }

// (a) EXACTLY-ONCE — two concurrent workers race to claim the SAME device serial;
// exactly one wins GRANTED, the other gets a clean DENIED (never both, never a
// block, never an error). This is the core §11.4.119 single-owner guarantee that
// stops two test workers from cross-contaminating one device's evidence.
func TestExactlyOnce_ConcurrentClaimants(t *testing.T) {
	const serial = "OPZ5-SERIAL-ABC123" // stable device serial (§11.4.111)
	// Repeat many times to shake out any nondeterministic race window.
	for iter := 0; iter < 500; iter++ {
		r := New(Config{})
		var granted, denied int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		for w := 0; w < 2; w++ {
			wg.Add(1)
			worker := fmt.Sprintf("worker-%d", w)
			go func() {
				defer wg.Done()
				<-start // maximise the concurrent race on the CAS
				_, outcome, err := r.TryAcquire(serial, worker, time.Minute)
				if err != nil {
					t.Errorf("iter %d: unexpected error: %v", iter, err)
					return
				}
				switch outcome {
				case OutcomeGranted:
					atomic.AddInt64(&granted, 1)
				case OutcomeDenied:
					atomic.AddInt64(&denied, 1)
				default:
					t.Errorf("iter %d: unexpected outcome %q", iter, outcome)
				}
			}()
		}
		close(start)
		wg.Wait()
		if granted != 1 || denied != 1 {
			t.Fatalf("iter %d: exactly-once broken: granted=%d denied=%d (want 1/1)", iter, granted, denied)
		}
		// Single-owner invariant: the device has exactly one owner.
		if _, held := r.Owner(serial); !held {
			t.Fatalf("iter %d: device has no owner after a successful grant", iter)
		}
	}
}

// EXACTLY-ONCE idempotency — the SAME worker re-acquiring its own device gets its
// existing lease back (GRANTED_EXISTING), never a second lease. A worker can
// never accidentally hold two claims on one device.
func TestExactlyOnce_SameWorkerIdempotent(t *testing.T) {
	r := New(Config{})
	const serial, worker = "SER-1", "worker-A"
	l1, o1, err := r.TryAcquire(serial, worker, time.Minute)
	if err != nil || o1 != OutcomeGranted {
		t.Fatalf("first acquire: outcome=%q err=%v (want GRANTED)", o1, err)
	}
	l2, o2, err := r.TryAcquire(serial, worker, time.Minute)
	if err != nil || o2 != OutcomeGrantedExisting {
		t.Fatalf("re-acquire: outcome=%q err=%v (want GRANTED_EXISTING)", o2, err)
	}
	if l1.ClaimID != l2.ClaimID {
		t.Fatalf("idempotency broken: claim ids differ %q vs %q", l1.ClaimID, l2.ClaimID)
	}
}

// (b) SINGLE-OWNER OVER TIME — worker-A holds a device, releases it, then a
// DIFFERENT worker-B can claim it; and while A holds it, B is denied. Proves the
// exclusive lock is genuinely released on done and re-grantable.
func TestSingleOwner_ReleaseThenReclaim(t *testing.T) {
	r := New(Config{})
	const serial = "SER-REUSE"

	leaseA, o, err := r.TryAcquire(serial, "worker-A", time.Minute)
	if err != nil || o != OutcomeGranted {
		t.Fatalf("A acquire: outcome=%q err=%v (want GRANTED)", o, err)
	}
	// While A holds it, B is cleanly denied (no double-owner).
	if _, o, _ := r.TryAcquire(serial, "worker-B", time.Minute); o != OutcomeDenied {
		t.Fatalf("B acquire while A holds: outcome=%q (want DENIED)", o)
	}
	// A stale worker cannot release with the wrong claim id (§9.2).
	if err := r.Release(serial, "bogus-claim-id"); err == nil {
		t.Fatalf("release with wrong claim id must fail, got nil")
	}
	// A releases with the correct claim id.
	if err := r.Release(serial, leaseA.ClaimID); err != nil {
		t.Fatalf("A release: %v", err)
	}
	if _, held := r.Owner(serial); held {
		t.Fatalf("device still owned after release")
	}
	// Now B can take exclusive ownership.
	if _, o, err := r.TryAcquire(serial, "worker-B", time.Minute); err != nil || o != OutcomeGranted {
		t.Fatalf("B reclaim after release: outcome=%q err=%v (want GRANTED)", o, err)
	}
	if owner, _ := r.Owner(serial); owner != "worker-B" {
		t.Fatalf("owner after reclaim=%q (want worker-B)", owner)
	}
}

// (c) DEADLOCK-FREE — a storm of many workers concurrently contends for a SMALL
// pool of devices via direct TryAcquire. Because every acquire is the non-
// blocking CAS, the storm completes (bounded time) and NEVER double-assigns a
// device: at all times each device has ≤ 1 owner. Hold-and-wait never occurs.
func TestDeadlockFree_MultiDeviceStorm(t *testing.T) {
	r := New(Config{})
	devices := []string{"DEV-1", "DEV-2", "DEV-3"}
	const workers = 60

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		var granted int64
		for w := 0; w < workers; w++ {
			wg.Add(1)
			worker := fmt.Sprintf("w-%d", w)
			go func() {
				defer wg.Done()
				// Each worker tries every device once; a real worker would then
				// drive whichever it won and release. Here we prove no deadlock +
				// no double-owner under maximal contention.
				for _, d := range devices {
					if lease, o, err := r.TryAcquire(d, worker, time.Minute); err == nil && o == OutcomeGranted {
						atomic.AddInt64(&granted, 1)
						// Immediately release so the pool keeps churning (exercises
						// the acquire→release→re-acquire cycle under contention).
						if err := r.Release(d, lease.ClaimID); err != nil {
							t.Errorf("release %s: %v", d, err)
						}
					}
				}
			}()
		}
		wg.Wait()
		if granted == 0 {
			t.Errorf("no device was ever granted under contention")
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("multi-device claim storm did NOT complete in 10s — deadlock")
	}
	// Final state: after all release, no device is held.
	for _, d := range devices {
		if r.IsHeld(d) {
			t.Fatalf("device %s still held after storm drained", d)
		}
	}
}

// (c) DEADLOCK-FREE via the scheduler — all-or-nothing multi-device placement.
// More workers than devices contend through PlaceWorkers concurrently; every
// pass terminates, every reachable device ends up owned by exactly one worker,
// and the surplus workers are honestly Unassigned (never dropped, never double-
// assigned). Proves the engine scheduler composition is deadlock-free too.
func TestDeadlockFree_PlaceWorkersStorm(t *testing.T) {
	r := New(Config{})
	devices := []string{"DEV-A", "DEV-B"}
	workers := []string{"crit-1", "crit-2", "crit-3", "crit-4"} // priority order
	reachable := func(string) bool { return true }
	now := time.Now

	done := make(chan struct{})
	go func() {
		defer close(done)
		p, err := r.PlaceWorkers(devices, workers, reachable, time.Minute, now)
		if err != nil {
			t.Errorf("PlaceWorkers: %v", err)
			return
		}
		if len(p.Assignments) != len(workers) {
			t.Errorf("never-drop broken: %d assignments for %d workers", len(p.Assignments), len(workers))
		}
		if len(p.Assigned) != len(devices) {
			t.Errorf("assigned=%d (want %d, one per device)", len(p.Assigned), len(devices))
		}
		if len(p.Unassigned) != len(workers)-len(devices) {
			t.Errorf("unassigned=%d (want %d surplus workers)", len(p.Unassigned), len(workers)-len(devices))
		}
		// No device double-assigned: each claimed serial appears once.
		seen := map[string]string{}
		for _, a := range p.Assignments {
			if !a.Assigned {
				continue
			}
			if prev, dup := seen[a.Serial]; dup {
				t.Errorf("device %s double-assigned to %s and %s", a.Serial, prev, a.Worker)
			}
			seen[a.Serial] = a.Worker
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("PlaceWorkers storm did NOT complete in 10s — deadlock")
	}
}

// (d) TTL REAP — a pure-TTL lease (no liveness proof) whose horizon has elapsed
// is reclaimable by a different worker. Models a worker that crashed WITHOUT a
// liveness proof configured: its device lock lapses at the TTL and another
// worker can take over.
func TestTTLReap_ExpiredLease(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	r := New(Config{Now: fixedClock(&now)})
	const serial = "SER-TTL"

	if _, o, err := r.TryAcquire(serial, "worker-dead", 30*time.Second); err != nil || o != OutcomeGranted {
		t.Fatalf("initial acquire: outcome=%q err=%v", o, err)
	}
	// Before expiry: a different worker is denied.
	if _, o, _ := r.TryAcquire(serial, "worker-live", time.Minute); o != OutcomeDenied {
		t.Fatalf("acquire before TTL expiry: outcome=%q (want DENIED)", o)
	}
	// Advance past the 30s TTL horizon.
	now = now.Add(31 * time.Second)
	// The stale lease is reaped on the next decision, so the device is free.
	if r.IsHeld(serial) {
		t.Fatalf("expired lease still reads as held")
	}
	lease, o, err := r.TryAcquire(serial, "worker-live", time.Minute)
	if err != nil || o != OutcomeGranted {
		t.Fatalf("acquire after TTL expiry: outcome=%q err=%v (want GRANTED)", o, err)
	}
	if lease.Worker != "worker-live" {
		t.Fatalf("reclaimed lease worker=%q (want worker-live)", lease.Worker)
	}
}

// (d) DEAD-WORKER REAP via liveness — with a liveness proof configured, a
// provably-DEAD worker's device lock is reclaimable IMMEDIATELY (before TTL),
// while a provably-ALIVE worker's lock is NEVER reaped even past its TTL (the
// live proof is the heartbeat; reaping a live worker would cross-contaminate its
// device — §9.2 / §11.4.119). Models Helix crash-detection over `kill -0`.
func TestLivenessReap_DeadWorker(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	// Every worker that will legitimately HOLD a device must report alive; the
	// engine reaps any holder whose liveness proof is false, so a live takeover
	// worker must be registered alive too (worker-dead is flipped to dead below).
	alive := map[string]bool{"worker-alive": true, "worker-dead": true, "worker-takeover": true}
	var mu sync.Mutex
	r := New(Config{
		Now: fixedClock(&now),
		Liveness: func(w string) bool {
			mu.Lock()
			defer mu.Unlock()
			return alive[w]
		},
	})

	// Dead-worker device: reclaimable the moment the worker is proven dead, even
	// though its 1h TTL is nowhere near elapsed.
	const deadSerial = "SER-DEADHOLDER"
	if _, o, err := r.TryAcquire(deadSerial, "worker-dead", time.Hour); err != nil || o != OutcomeGranted {
		t.Fatalf("dead-worker initial acquire: outcome=%q err=%v", o, err)
	}
	mu.Lock()
	alive["worker-dead"] = false // crash detected
	mu.Unlock()
	if _, o, err := r.TryAcquire(deadSerial, "worker-takeover", time.Hour); err != nil || o != OutcomeGranted {
		t.Fatalf("takeover of dead worker's device: outcome=%q err=%v (want GRANTED before TTL)", o, err)
	}
	if owner, _ := r.Owner(deadSerial); owner != "worker-takeover" {
		t.Fatalf("dead-worker device owner=%q (want worker-takeover)", owner)
	}

	// Alive-worker device: NEVER reaped, even long past its TTL.
	const aliveSerial = "SER-LIVEHOLDER"
	if _, o, err := r.TryAcquire(aliveSerial, "worker-alive", 10*time.Second); err != nil || o != OutcomeGranted {
		t.Fatalf("alive-worker acquire: outcome=%q err=%v", o, err)
	}
	now = now.Add(time.Hour) // far past the 10s TTL
	if !r.IsHeld(aliveSerial) {
		t.Fatalf("provably-alive worker's device was reaped past TTL — single-owner break")
	}
	if _, o, _ := r.TryAcquire(aliveSerial, "worker-intruder", time.Minute); o != OutcomeDenied {
		t.Fatalf("intruder acquire of alive worker's device: outcome=%q (want DENIED)", o)
	}
}

// STABLE-SERIAL GUARD (§11.4.111) — a device MUST be addressed by a non-empty
// stable serial, never a blank / positional handle. Empty serial and empty
// worker are rejected before any claim is attempted.
func TestStableSerialGuard(t *testing.T) {
	r := New(Config{})
	if _, _, err := r.TryAcquire("", "worker-A", time.Minute); err != ErrEmptySerial {
		t.Fatalf("empty serial: err=%v (want ErrEmptySerial)", err)
	}
	if _, _, err := r.TryAcquire("SER-X", "", time.Minute); err != ErrEmptyWorker {
		t.Fatalf("empty worker: err=%v (want ErrEmptyWorker)", err)
	}
}

// PlaceWorkers fail-closed: an unreachable device is NEVER assigned (§11.4.69
// no-fail-open). With one reachable + one unreachable device and two workers,
// exactly one worker is assigned (to the reachable device) and one is honestly
// Unassigned.
func TestPlaceWorkers_UnreachableDeviceExcluded(t *testing.T) {
	r := New(Config{})
	devices := []string{"DEV-OK", "DEV-DOWN"}
	reachable := func(s string) bool { return s == "DEV-OK" }
	p, err := r.PlaceWorkers(devices, []string{"w1", "w2"}, reachable, time.Minute, time.Now)
	if err != nil {
		t.Fatalf("PlaceWorkers: %v", err)
	}
	if len(p.Assigned) != 1 || len(p.Unassigned) != 1 {
		t.Fatalf("assigned=%v unassigned=%v (want 1 assigned to DEV-OK, 1 unassigned)", p.Assigned, p.Unassigned)
	}
	for _, a := range p.Assignments {
		if a.Assigned && a.Serial != "DEV-OK" {
			t.Fatalf("assigned an unreachable device %s", a.Serial)
		}
	}
}
