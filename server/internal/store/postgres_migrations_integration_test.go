//go:build integration

// SRV-NEW-1 integration proof (§11.4.69 real captured evidence, §11.4.74 reuse
// the containers brick): drives the REAL pgx versioned-migration path against a
// live PostgreSQL booted on-demand through the digital.vasic.containers
// submodule (port 55432, serialized via lockPgIntegration per §11.4.119).
//
// Asserts the framework's ledger semantics that the in-suite fake-executor test
// (migrations_test.go) cannot: after a fresh Migrate the schema_migrations
// ledger holds exactly the baseline row (version 1), and a SECOND Migrate is a
// genuine no-op (ledger unchanged, applied_at not rewritten) — i.e. the whole
// schema DDL is no longer blindly re-exec'd on every boot. Run:
//
//	go test -tags integration ./internal/store/
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

// TestPostgresMigrationLedger_Integration proves the schema_migrations ledger is
// created, records the baseline, and makes a repeat Migrate a no-op.
func TestPostgresMigrationLedger_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 serialize shared Postgres across integration packages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}

	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Fatalf("no container runtime (podman/docker) available: %v", err)
	}
	t.Logf("container runtime: %s", rt.Name())

	orch, err := compose.NewDefaultOrchestrator(projectDir, logging.NopLogger{})
	if err != nil {
		t.Fatalf("compose orchestrator (need podman/docker compose): %v", err)
	}

	ep := endpoint.NewEndpoint().
		WithHost("localhost").WithPort(pgHostPort).
		WithHealthType("tcp").WithRequired(true).WithEnabled(true).
		WithComposeFile("postgres.compose.yml").WithServiceName("postgres").
		WithTimeout(120 * time.Second).WithRetryCount(60).
		Build()

	mgr := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{"postgres": ep},
		boot.WithRuntime(rt),
		boot.WithOrchestrator(orch),
		boot.WithHealthChecker(health.NewDefaultChecker()),
		boot.WithProjectDir(projectDir),
		boot.WithLogger(logging.NopLogger{}),
	)
	summary, err := mgr.BootAll(ctx)
	if err != nil {
		t.Fatalf("BootAll (required postgres) failed: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })
	if summary.Failed > 0 {
		t.Fatalf("boot summary reports %d failed service(s)", summary.Failed)
	}

	pool := mustPool(t, ctx)
	t.Cleanup(pool.Close)

	resetSchema(t, ctx) // clean baseline (DROP SCHEMA helix_ota CASCADE)
	repo := NewPostgresRepositoryFromPool(pool)

	// --- first Migrate: ledger created + baseline recorded ---
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM helix_ota.schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count ledger rows: %v", err)
	}
	if count != len(registeredMigrations) {
		t.Fatalf("ledger row count = %d, want %d (one per registered migration)", count, len(registeredMigrations))
	}

	var version int64
	var name string
	var appliedAt time.Time
	if err := pool.QueryRow(ctx,
		"SELECT version, name, applied_at FROM helix_ota.schema_migrations WHERE version=1").
		Scan(&version, &name, &appliedAt); err != nil {
		t.Fatalf("read baseline ledger row: %v", err)
	}
	if version != 1 || name != "baseline" {
		t.Fatalf("baseline ledger row = (version=%d, name=%q), want (1, \"baseline\")", version, name)
	}
	if appliedAt.IsZero() {
		t.Fatal("baseline applied_at must be set")
	}

	// The schema itself must actually be there (baseline really ran, not just
	// recorded) — a core table must be queryable.
	if _, err := pool.Exec(ctx, "SELECT 1 FROM helix_ota.devices WHERE FALSE"); err != nil {
		t.Fatalf("baseline schema not applied (devices table absent): %v", err)
	}

	// --- second Migrate: idempotent no-op, ledger unchanged ---
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate (must be a no-op): %v", err)
	}
	var count2 int
	var appliedAt2 time.Time
	if err := pool.QueryRow(ctx,
		"SELECT count(*), max(applied_at) FROM helix_ota.schema_migrations").
		Scan(&count2, &appliedAt2); err != nil {
		t.Fatalf("re-count ledger rows: %v", err)
	}
	if count2 != count {
		t.Fatalf("second Migrate changed ledger row count: %d -> %d", count, count2)
	}
	if !appliedAt2.Equal(appliedAt) {
		t.Fatalf("second Migrate rewrote applied_at (%s -> %s); a no-op must not re-record", appliedAt, appliedAt2)
	}
}
