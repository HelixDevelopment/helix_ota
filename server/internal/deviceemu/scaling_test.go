package deviceemu_test

// Scaling / load test for the Helix OTA control plane.
//
// GOAL (anti-bluff, Constitution §11.4.27 + §11.4.85): drive N concurrent
// emulated devices against ONE real in-process control plane over the real OTA
// protocol (login -> register -> CheckUpdate -> telemetry) and MEASURE
// throughput + latency percentiles (p50/p95/p99). This proves the control plane
// + the emulator scale under genuine concurrent load — it is NOT a happy-path
// single-device smoke test. Every device speaks the real HTTP endpoints via the
// deviceemu.Device API against api.NewServer's real router (booted by the shared
// bootServer/stageDeployment fixture in emulator_test.go, same package).
//
// Scope honesty (§11.4.6): this is CONTROL-PLANE scaling under the real wire
// protocol, in-process over httptest (no kernel TCP stack, no container, no
// network RTT). It exercises the server's concurrent device-registration,
// update-resolution and telemetry-ingest paths under real goroutine contention
// (run under -race it also proves the server + repo are data-race-free under
// concurrent access). A network/container fleet load test (real sockets, real
// per-container overhead) is a SEPARATE concern covered elsewhere — this file
// does not claim to measure that.
//
// Host safety (§12.6): N defaults to 50 in-process emulated devices, each a
// lightweight goroutine doing a handful of HTTP round-trips against an
// in-memory server. Memory + CPU stay far under the 60% ceiling. N is
// overridable via HELIX_SCALING_N for heavier local probing but the committed
// default is bounded + fast so it passes in the normal `go test` run at
// -count=2 (§11.4.50 deterministic-consistency: the PASS / zero-error invariant
// is stable across runs; only the latency NUMBERS vary).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
)

// scalingN returns the number of concurrent devices to drive. Default 50
// (host-safe, §12.6); HELIX_SCALING_N overrides for local heavier probing.
func scalingN(t *testing.T) int {
	t.Helper()
	const def = 50
	v := os.Getenv("HELIX_SCALING_N")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		t.Fatalf("invalid HELIX_SCALING_N=%q: %v", v, err)
	}
	return n
}

// latencyStats holds the computed percentiles + throughput for one phase of a
// scaling run. Durations are nanoseconds (the JSON evidence artifact also
// renders human-readable milliseconds).
type latencyStats struct {
	Phase     string  `json:"phase"`
	Ops       int     `json:"ops"`
	Errors    int     `json:"errors"`
	WallNanos int64   `json:"wall_nanos"`
	OpsPerSec float64 `json:"ops_per_sec"`
	P50Nanos  int64   `json:"p50_nanos"`
	P95Nanos  int64   `json:"p95_nanos"`
	P99Nanos  int64   `json:"p99_nanos"`
	MaxNanos  int64   `json:"max_nanos"`
	P50Millis float64 `json:"p50_millis"`
	P95Millis float64 `json:"p95_millis"`
	P99Millis float64 `json:"p99_millis"`
	MaxMillis float64 `json:"max_millis"`
}

// computeStats sorts the per-op durations and computes p50/p95/p99/max plus the
// aggregate throughput (ops/sec) over the measured wall-clock window.
func computeStats(phase string, durs []time.Duration, errors int, wall time.Duration) latencyStats {
	st := latencyStats{Phase: phase, Ops: len(durs), Errors: errors, WallNanos: int64(wall)}
	if wall > 0 {
		st.OpsPerSec = float64(len(durs)) / wall.Seconds()
	}
	if len(durs) == 0 {
		return st
	}
	sorted := make([]time.Duration, len(durs))
	copy(sorted, durs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	pick := func(p float64) time.Duration {
		// Nearest-rank percentile: index = ceil(p/100 * N) - 1, clamped.
		idx := int(p/100.0*float64(len(sorted))+0.999999) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	}
	st.P50Nanos = int64(pick(50))
	st.P95Nanos = int64(pick(95))
	st.P99Nanos = int64(pick(99))
	st.MaxNanos = int64(sorted[len(sorted)-1])
	ms := func(n int64) float64 { return float64(n) / 1e6 }
	st.P50Millis = ms(st.P50Nanos)
	st.P95Millis = ms(st.P95Nanos)
	st.P99Millis = ms(st.P99Nanos)
	st.MaxMillis = ms(st.MaxNanos)
	return st
}

// collector is a tiny concurrency-safe sink for per-op latencies + an error
// count. Run under -race it proves the test harness itself is data-race-free.
type collector struct {
	mu   sync.Mutex
	durs []time.Duration
	errs int
}

func (c *collector) add(d time.Duration) {
	c.mu.Lock()
	c.durs = append(c.durs, d)
	c.mu.Unlock()
}

func (c *collector) addErr() {
	c.mu.Lock()
	c.errs++
	c.mu.Unlock()
}

// TestScalingConcurrentRegisterCheck drives N concurrent emulated devices each
// performing register -> update-check against ONE real control plane with NO
// deployment staged, so every CheckUpdate must answer 204 (on-target / nothing
// offered). It measures per-device end-to-end latency (register + check),
// computes p50/p95/p99 + throughput, and asserts ZERO errors. Run under -race
// this also proves the server's concurrent register + update-resolution paths
// are data-race-free.
func TestScalingConcurrentRegisterCheck(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	n := scalingN(t)

	coll := &collector{}
	var wg sync.WaitGroup
	wg.Add(n)

	start := time.Now()
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			dev, err := deviceemu.New(deviceemu.Config{
				BaseURL:        fx.baseURL,
				AdminUser:      fx.adminUser,
				AdminPass:      fx.adminPass,
				HardwareID:     fmt.Sprintf("rk3588-SCALE-%05d", i),
				Model:          fxModel,
				OSType:         string(otaprotocol.OSAndroid),
				CurrentVersion: "1.0.0",
			})
			if err != nil {
				t.Errorf("device %d: new: %v", i, err)
				coll.addErr()
				return
			}
			opStart := time.Now()
			if err := dev.Register(ctx); err != nil {
				t.Errorf("device %d: register: %v", i, err)
				coll.addErr()
				return
			}
			// No deployment staged -> CheckUpdate must report on-target (204).
			_, onTarget, err := dev.CheckUpdate(ctx)
			if err != nil {
				t.Errorf("device %d: check: %v", i, err)
				coll.addErr()
				return
			}
			if !onTarget {
				t.Errorf("device %d: expected on-target (204) with no deployment, got an offer", i)
				coll.addErr()
				return
			}
			coll.add(time.Since(opStart))
		}(i)
	}
	wg.Wait()
	wall := time.Since(start)

	st := computeStats("register+check", coll.durs, coll.errs, wall)
	printScalingEvidence(t, n, st)

	if coll.errs != 0 {
		t.Fatalf("register+check phase had %d errors (want 0)", coll.errs)
	}
	if st.Ops != n {
		t.Fatalf("recorded %d successful ops, want %d", st.Ops, n)
	}
}

// TestScalingConcurrentFullLifecycle stages ONE all-targets deployment, then
// drives N concurrent emulated devices each completing the FULL update
// lifecycle (register -> check -> apply + report the success telemetry batch).
// It asserts every device advances to the deployed version, telemetry is
// accepted (rejected=0) for every device, and ZERO errors occur — under real
// goroutine contention against the in-memory control plane. It records the
// per-device full-lifecycle latency and prints p50/p95/p99 + throughput.
func TestScalingConcurrentFullLifecycle(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	n := scalingN(t)

	opToken := fx.operatorLogin(t)
	const group = "scale-fleet"
	relVersion, deploymentID := fx.stageDeployment(t, opToken, "9.0.0", group)

	coll := &collector{}
	var advanced int64 // count of devices that reached relVersion
	var advMu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)

	start := time.Now()
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			dev, err := deviceemu.New(deviceemu.Config{
				BaseURL:        fx.baseURL,
				AdminUser:      fx.adminUser,
				AdminPass:      fx.adminPass,
				HardwareID:     fmt.Sprintf("rk3588-LIFE-%05d", i),
				Model:          fxModel,
				OSType:         string(otaprotocol.OSAndroid),
				CurrentVersion: "1.0.0",
				Group:          group,
				// DeploymentID intentionally omitted: each device self-serves the
				// id from its own update offer (§11.4.6), exercising the real
				// telemetry deployment_id resolution under concurrency.
			})
			if err != nil {
				t.Errorf("device %d: new: %v", i, err)
				coll.addErr()
				return
			}
			opStart := time.Now()
			out, err := dev.RunOnce(ctx)
			if err != nil {
				t.Errorf("device %d: run once: %v", i, err)
				coll.addErr()
				return
			}
			if !out.Applied {
				t.Errorf("device %d: update not applied: %+v", i, out)
				coll.addErr()
				return
			}
			if out.ToVersion != relVersion {
				t.Errorf("device %d: advanced to %q, want %q", i, out.ToVersion, relVersion)
				coll.addErr()
				return
			}
			if out.DeploymentID != deploymentID {
				t.Errorf("device %d: stamped deployment_id %q, want %q", i, out.DeploymentID, deploymentID)
				coll.addErr()
				return
			}
			if out.TelemetryAccepted != 1 || out.TelemetryRejected != 0 {
				t.Errorf("device %d: telemetry accepted=%d rejected=%d, want 1/0",
					i, out.TelemetryAccepted, out.TelemetryRejected)
				coll.addErr()
				return
			}
			if !out.Healthy {
				t.Errorf("device %d: not healthy after success: %+v", i, out)
				coll.addErr()
				return
			}
			advMu.Lock()
			advanced++
			advMu.Unlock()
			coll.add(time.Since(opStart))
		}(i)
	}
	wg.Wait()
	wall := time.Since(start)

	st := computeStats("full-lifecycle", coll.durs, coll.errs, wall)
	printScalingEvidence(t, n, st)

	if coll.errs != 0 {
		t.Fatalf("full-lifecycle phase had %d errors (want 0)", coll.errs)
	}
	if advanced != int64(n) {
		t.Fatalf("only %d/%d devices advanced to %s", advanced, n, relVersion)
	}
	if st.Ops != n {
		t.Fatalf("recorded %d successful full lifecycles, want %d", st.Ops, n)
	}
}

// printScalingEvidence emits the measured scaling numbers as a human-readable
// table AND a machine-parseable JSON line (the captured-evidence artifact per
// §11.4.5 / §11.4.85). The JSON is greppable from test output for downstream
// telemetry ingestion; the table is for the operator reading the log.
func printScalingEvidence(t *testing.T, n int, st latencyStats) {
	t.Helper()
	t.Logf("SCALING[%s] N=%d ops=%d errors=%d wall=%s throughput=%.1f ops/sec | "+
		"p50=%.3fms p95=%.3fms p99=%.3fms max=%.3fms",
		st.Phase, n, st.Ops, st.Errors, time.Duration(st.WallNanos).Round(time.Millisecond),
		st.OpsPerSec, st.P50Millis, st.P95Millis, st.P99Millis, st.MaxMillis)
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal scaling evidence: %v", err)
	}
	t.Logf("SCALING_EVIDENCE_JSON %s", raw)
}
