//go:build integration

// Captures REAL-Postgres evidence (§11.4.69) that the emulation test-fabric
// registry tables exist and the §11.4.119 exclusive-lease invariant + the
// §11.4.69 non-empty-evidence CHECK are enforced by the database itself (not
// just the Go layer). Boots PG on-demand via the containers submodule, then
// asserts + logs the DB-side proofs. Run:  go test -tags integration ./internal/store/
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

func TestPostgresFabricRegistry_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 serialize shared Postgres across integration packages
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir, err := filepath.Abs("../../deploy")
	if err != nil {
		t.Fatalf("resolve deploy dir: %v", err)
	}
	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		t.Fatalf("no container runtime: %v", err)
	}
	t.Logf("container runtime: %s", rt.Name())
	orch, err := compose.NewDefaultOrchestrator(projectDir, logging.NopLogger{})
	if err != nil {
		t.Fatalf("compose orchestrator: %v", err)
	}
	ep := endpoint.NewEndpoint().
		WithHost("localhost").WithPort(pgHostPort).
		WithHealthType("tcp").WithRequired(true).WithEnabled(true).
		WithComposeFile("postgres.compose.yml").WithServiceName("postgres").
		WithTimeout(120 * time.Second).WithRetryCount(60).
		Build()
	mgr := boot.NewBootManager(
		map[string]endpoint.ServiceEndpoint{"postgres": ep},
		boot.WithRuntime(rt), boot.WithOrchestrator(orch),
		boot.WithHealthChecker(health.NewDefaultChecker()),
		boot.WithProjectDir(projectDir), boot.WithLogger(logging.NopLogger{}),
	)
	if _, err := mgr.BootAll(ctx); err != nil {
		t.Fatalf("BootAll: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Shutdown(context.Background()) })

	repo := mustConnectPostgres(t, ctx)
	t.Cleanup(repo.Close)
	resetSchema(t, ctx)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// The five fabric tables + the lease UNIQUE partial index exist after migrate.
	for _, tbl := range []string{"fabric_nodes", "fabric_targets", "fabric_leases", "fabric_runs", "fabric_evidence"} {
		var reg string
		if err := repo.pool.QueryRow(ctx,
			`SELECT to_regclass($1)::text`, "helix_ota."+tbl).Scan(&reg); err != nil {
			t.Fatalf("regclass %s: %v", tbl, err)
		}
		if reg == "" {
			t.Fatalf("fabric table helix_ota.%s NOT created", tbl)
		}
		t.Logf("fabric table present: helix_ota.%s", reg)
	}
	var idx string
	if err := repo.pool.QueryRow(ctx,
		`SELECT indexname FROM pg_indexes WHERE schemaname='helix_ota' AND indexname='uq_fabric_lease_active'`).Scan(&idx); err != nil {
		t.Fatalf("lease unique partial index missing: %v", err)
	}
	t.Logf("exclusive-lease UNIQUE partial index present: %s", idx)

	ts := time.Now().UTC()
	if err := repo.CreateFabricNode(ctx, FabricNode{NodeID: "n1", Kind: "ci-linux-kvm", Arch: "x86_64", HasKVM: true, LastSeenAt: ts, CreatedAt: ts}); err != nil {
		t.Fatalf("CreateFabricNode: %v", err)
	}
	if err := repo.CreateFabricTarget(ctx, FabricTarget{TargetID: "t1", Tier: "T2", Tech: "cuttlefish", Exclusive: true, NodeID: "n1", CreatedAt: ts}); err != nil {
		t.Fatalf("CreateFabricTarget: %v", err)
	}

	// §11.4.119 proof — DB-enforced single ACTIVE lease per exclusive target.
	if err := repo.AcquireFabricLease(ctx, FabricLease{LeaseID: "L1", TargetID: "t1", Owner: "run-A", AcquiredAt: ts}); err != nil {
		t.Fatalf("first lease: %v", err)
	}
	t.Logf("LEASE-UNIQUENESS: first lease L1 acquired (target=t1, owner=run-A)")
	if err := repo.AcquireFabricLease(ctx, FabricLease{LeaseID: "L2", TargetID: "t1", Owner: "run-B", AcquiredAt: ts}); err == nil {
		t.Fatalf("LEASE-UNIQUENESS VIOLATED: DB allowed a SECOND active lease on exclusive target t1")
	} else {
		t.Logf("LEASE-UNIQUENESS: second lease L2 on t1 REJECTED by DB -> %v (mapped to ErrConflict)", err)
	}
	// Count active leases in the DB directly = exactly 1.
	var active int
	if err := repo.pool.QueryRow(ctx,
		`SELECT count(*) FROM helix_ota.fabric_leases WHERE target_id='t1' AND release_at IS NULL`).Scan(&active); err != nil {
		t.Fatalf("count active leases: %v", err)
	}
	if active != 1 {
		t.Fatalf("LEASE-UNIQUENESS: want exactly 1 active lease in DB, got %d", active)
	}
	t.Logf("LEASE-UNIQUENESS: DB row count of active leases on t1 = %d (exactly one)", active)

	// §11.4.69 proof — DB CHECK rejects a 0-byte evidence artefact.
	if err := repo.CreateFabricRun(ctx, FabricRun{RunID: "r1", TargetID: "t1", TestType: "e2e", TestRef: "tests/emulator/lifecycle.sh", StartedAt: ts}); err != nil {
		t.Fatalf("CreateFabricRun: %v", err)
	}
	if _, err := repo.pool.Exec(ctx,
		`INSERT INTO helix_ota.fabric_evidence (evidence_id, run_id, kind, path, byte_size, created_at) VALUES ('bad','r1','k','p',0,now())`); err == nil {
		t.Fatalf("EVIDENCE-CHECK VIOLATED: DB accepted a 0-byte evidence row")
	} else {
		t.Logf("EVIDENCE-CHECK: raw 0-byte INSERT REJECTED by DB CHECK -> %v", err)
	}
	if err := repo.AttachFabricEvidence(ctx, FabricEvidence{EvidenceID: "e1", RunID: "r1", Kind: "ab-slot", Path: "docs/qa/r1/slot.json", ByteSize: 4096, CreatedAt: ts}); err != nil {
		t.Fatalf("AttachFabricEvidence non-empty: %v", err)
	}
	t.Logf("EVIDENCE-CHECK: non-empty (4096-byte) evidence accepted")
}
