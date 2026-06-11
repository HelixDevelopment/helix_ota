//go:build integration

// Package store integration test: proves the pgx/PostgreSQL Repository satisfies
// the exact same behavioural contract as the in-memory one, against a REAL
// PostgreSQL that is booted on-demand through the containers submodule
// (digital.vasic.containers) — never a manual `podman`/`compose` step, never a
// fake. Run with:  go test -tags integration ./internal/store/
//
// Constitution: anti-bluff §11.4 (real infra, captured evidence), §11.4.74
// (reuse the containers catalogue brick, never reimplement orchestration).
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
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgHostPort = "55432"
	pgDSN      = "postgres://helix:helix@localhost:55432/helix_ota?sslmode=disable"
)

func TestPostgresRepositoryContract_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 serialize shared Postgres across integration packages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}

	// --- boot PostgreSQL on-demand via the containers submodule ---
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
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})
	if summary.Failed > 0 {
		t.Fatalf("boot summary reports %d failed service(s)", summary.Failed)
	}
	t.Logf("boot summary: started=%d discovered=%d failed=%d in %s",
		summary.Started, summary.Discovered, summary.Failed, summary.TotalDuration)

	// --- wait for the DB to accept queries (TCP-open != query-ready) ---
	repo := mustConnectPostgres(t, ctx)
	t.Cleanup(repo.Close)

	// --- clean slate, then apply the store schema ---
	resetSchema(t, ctx)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// --- the SAME contract the in-memory repo satisfies ---
	runRepositoryContract(t, repo)
}

// mustConnectPostgres retries until the freshly-booted Postgres accepts queries.
func mustConnectPostgres(t *testing.T, ctx context.Context) *PostgresRepository {
	t.Helper()
	var lastErr error
	for i := 0; i < 60; i++ {
		repo, err := NewPostgresRepository(ctx, pgDSN)
		if err == nil {
			return repo
		}
		lastErr = err
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for postgres: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("postgres never became query-ready: %v", lastErr)
	return nil
}

// resetSchema drops the helix_ota schema for a deterministic clean slate.
func resetSchema(t *testing.T, ctx context.Context) {
	t.Helper()
	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		t.Fatalf("reset connect: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS helix_ota CASCADE"); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
}
