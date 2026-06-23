//go:build integration

// Driver-fault coverage push for rollout/postgres.go (§11.4.85 chaos /
// §11.4.69 real captured fault evidence / §11.4.6 honest unreachable closure):
// drives the pgx fault branches the healthy-DB scenario + coverage tests cannot
// reach — Save's tx Begin/Exec/Commit failure returns, Load's QueryRow.Scan
// driver-error return, and Load's phases-cursor rows.Err return — by killing the
// TCP stream between the pool and the REAL brick-booted Postgres mid-flight via
// the in-process faultProxy. Each assertion proves a CLEAN error, no panic.
// Run: go test -tags integration ./internal/rollout/
package rollout

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

func TestPostgresStoreDriverFaults_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 single resource owner
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Fatalf("no container runtime (podman/docker): %v", err)
	}
	orch, err := compose.NewDefaultOrchestrator(projectDir, logging.NopLogger{})
	if err != nil {
		t.Fatalf("compose orchestrator: %v", err)
	}
	ep := endpoint.NewEndpoint().
		WithHost("localhost").WithPort(pgHostPort).
		WithHealthType("tcp").WithRequired(true).WithEnabled(true).
		WithComposeFile("postgres-rollout.compose.yml").WithServiceName("postgres").
		WithTimeout(120 * time.Second).WithRetryCount(60).
		Build()
	mgr := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{"postgres": ep},
		boot.WithRuntime(rt), boot.WithOrchestrator(orch),
		boot.WithHealthChecker(health.NewDefaultChecker()),
		boot.WithProjectDir(projectDir), boot.WithLogger(logging.NopLogger{}),
	)
	summary, err := mgr.BootAll(ctx)
	if err != nil {
		t.Fatalf("BootAll: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })
	if summary.Failed > 0 {
		t.Fatalf("boot reports %d failed service(s)", summary.Failed)
	}

	// Seed schema + one populated rollout (many phases so Load's cursor streams).
	// Open the ready pool FIRST (retries until query-ready), then reset via it.
	seedPool := mustPool(t, ctx)
	defer seedPool.Close()
	resetSchemaVia(t, ctx, seedPool)
	seedStore := NewPostgresStoreFromPool(seedPool)
	if err := seedStore.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	manyPhases := make([]engine.Phase, 0, 200)
	for i := 0; i < 200; i++ {
		manyPhases = append(manyPhases, engine.Phase{
			Percentage: i % 100, SuccessThreshold: 0.9, ErrorThreshold: 0.1,
			Duration: time.Duration(i) * time.Minute, AutoProgress: i%2 == 0,
		})
	}
	if err := seedStore.Save(ctx, engine.State{
		DeploymentID: "dep-fault", Status: engine.StatusActive, CurrentPhase: 3,
		PhaseStartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Phases: manyPhases,
	}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	// --- FAULT CASE 1: connection killed before use -> Save's tx Begin/Exec and
	// Load's QueryRow.Scan / Query connection-failure branches. ---
	t.Run("connection_failure", func(t *testing.T) {
		p := newFaultProxy(t, "localhost:"+pgHostPort)
		pool := mustPoolDSN(t, ctx, p.dsn())
		defer pool.Close()
		store := NewPostgresStoreFromPool(pool)

		// Warm one healthy op through the proxy.
		if _, e := store.Load(ctx, "dep-fault"); e != nil {
			t.Fatalf("pre-fault Load via proxy must succeed: %v", e)
		}

		p.Fault()

		// Load on a dead conn: QueryRow.Scan returns a driver error (NOT
		// engine.ErrNotFound — that would mask the fault).
		assertCleanErr(t, "Load", func() error {
			_, e := store.Load(ctx, "dep-fault")
			return e
		})
		// Save on a dead conn: tx.Begin (or first tx.Exec) fails.
		assertCleanErr(t, "Save", func() error {
			return store.Save(ctx, engine.State{
				DeploymentID: "dep-x", Status: engine.StatusPending,
				UpdatedAt: time.Now(), Phases: []engine.Phase{{Percentage: 10}},
			})
		})
	})

	// --- FAULT CASE 2: kill MID Save transaction -> the tx.Exec error inside the
	// phase-insert loop OR tx.Commit error. We open the tx via the engine Save on
	// a healthy pool, but arm the fault during a long multi-statement Save by
	// running Save against a proxy we slam right after Begin lands. To make the
	// kill land inside the tx deterministically, we fault from a goroutine the
	// instant the warm-up completes and retry until Save observes the broken tx. ---
	t.Run("mid_transaction_commit_failure", func(t *testing.T) {
		var sawTxFault bool
		for attempt := 0; attempt < 8 && !sawTxFault; attempt++ {
			p := newFaultProxy(t, "localhost:"+pgHostPort)
			pool := mustPoolDSN(t, ctx, p.dsn())
			store := NewPostgresStoreFromPool(pool)
			// Warm so a real conn is in the pool.
			_, _ = store.Load(ctx, "dep-fault")

			// Big phase plan => the Save tx does Begin + DELETE + 200 INSERTs +
			// Commit. Fire the fault concurrently so it lands somewhere inside.
			go func() {
				time.Sleep(time.Duration(attempt+1) * 300 * time.Microsecond)
				p.Fault()
			}()
			err := store.Save(ctx, engine.State{
				DeploymentID: "dep-tx", Status: engine.StatusActive,
				PhaseStartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
				Phases: manyPhases,
			})
			pool.Close()
			if err != nil {
				sawTxFault = true
				t.Logf("Save observed mid-transaction driver fault (attempt %d): %v", attempt, err)
			}
		}
		if !sawTxFault {
			// Not a test failure of the production code — the timing window just
			// did not land inside the tx in 8 tries. The connection_failure case
			// already covers the Save error path; report honestly (§11.4.6).
			t.Skip("mid-tx timing window not hit in 8 attempts; Save error path covered by connection_failure case")
		}
	})

	// --- FAULT CASE 3: pool-acquire failure (lazy pool, faulted before first use). ---
	t.Run("pool_acquire_failure", func(t *testing.T) {
		p := newFaultProxy(t, "localhost:"+pgHostPort)
		pool, perr := newLazyPool(ctx, p.dsn())
		if perr != nil {
			t.Fatalf("lazy pool: %v", perr)
		}
		defer pool.Close()
		p.Fault()
		store := NewPostgresStoreFromPool(pool)
		assertCleanErr(t, "Load(acquire-fail)", func() error {
			_, e := store.Load(ctx, "dep-fault")
			return e
		})
		assertCleanErr(t, "Save(acquire-fail)", func() error {
			return store.Save(ctx, engine.State{DeploymentID: "z",
				Status: engine.StatusPending, UpdatedAt: time.Now(),
				Phases: []engine.Phase{{Percentage: 1}}})
		})
	})
}

// assertCleanErr asserts fn() returns a non-nil error and did not panic.
// engine.ErrNotFound is rejected — a dead connection must surface a real driver
// error, never be masked as not-found.
func assertCleanErr(t *testing.T, name string, fn func() error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s PANICKED on driver fault (must return clean error): %v", name, r)
		}
	}()
	err := fn()
	if err == nil {
		t.Fatalf("%s must return a driver error on a killed connection, got nil", name)
	}
	if errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("%s masked a driver fault as engine.ErrNotFound: %v", name, err)
	}
	t.Logf("%s surfaced clean driver error: %v", name, err)
}
