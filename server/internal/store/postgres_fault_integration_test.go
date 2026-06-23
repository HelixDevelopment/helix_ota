//go:build integration

// Driver-fault coverage push (§11.4.85 chaos / §11.4.69 real captured fault
// evidence / §11.4.6 honest unreachable-branch closure): exercises the pgx
// driver-fault branches in store/postgres.go that the healthy-DB contract +
// coverage tests cannot reach — the `pool.Query(...)` connection-failure returns,
// the mid-iteration `rows.Scan` / `rows.Err()` error returns, and the
// pool-acquire-failure path. The fault is injected by the in-process faultProxy
// (faultproxy_test.go) that kills the TCP stream between the pool and the REAL
// brick-booted Postgres mid-flight. Every assertion proves the pgx call returns
// a CLEAN error (never a panic). Run: go test -tags integration ./internal/store/
package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/health"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/runtime"
)

// TestPostgresDriverFaults_Integration boots a real Postgres, seeds rows, then
// uses the faultProxy to kill connections and assert the production List* / scan
// paths surface clean driver errors (hitting the rows.Err / pool.Query failure
// branches).
func TestPostgresDriverFaults_Integration(t *testing.T) {
	lockPgIntegration(t) // §11.4.119 single resource owner — serialize shared Postgres
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
		WithComposeFile("postgres.compose.yml").WithServiceName("postgres").
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

	// Seed a populated schema via a direct pool (NOT through the proxy). Open the
	// pool FIRST (mustPool retries until the DB is query-ready — TCP-open from the
	// boot healthcheck != query-ready), then reset the schema through it.
	seedPool := mustPool(t, ctx)
	defer seedPool.Close()
	resetSchemaVia(t, ctx, seedPool)
	seedRepo := NewPostgresRepositoryFromPool(seedPool)
	if err := seedRepo.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	seedManyDevices(t, ctx, seedRepo, 500) // enough rows that a List* streams across reads

	// --- FAULT CASE 1: connection killed BEFORE query -> pool.Query / QueryRow
	// connection-failure branch (the early `if err != nil { return ..., err }`). ---
	t.Run("query_connection_failure", func(t *testing.T) {
		p := newFaultProxy(t, "localhost:"+pgHostPort)
		pool := mustPoolDSN(t, ctx, p.dsn())
		defer pool.Close()
		repo := NewPostgresRepositoryFromPool(pool)

		// Warm one real query so we know the proxy path works healthy.
		if _, _, e := repo.ListDevices(ctx, DeviceFilter{Limit: 1}); e != nil {
			t.Fatalf("pre-fault ListDevices via proxy must succeed: %v", e)
		}

		p.Fault() // kill all conns + refuse new ones

		// Every method that does pool.Query / QueryRow must now return a non-nil
		// error and NOT panic. These hit the connection-failure return branches.
		assertCleanErr(t, "ListDevices", func() error {
			_, _, e := repo.ListDevices(ctx, DeviceFilter{})
			return e
		})
		assertCleanErr(t, "ListReleases", func() error {
			_, _, e := repo.ListReleases(ctx, ReleaseFilter{})
			return e
		})
		assertCleanErr(t, "ListActiveDeployments", func() error {
			_, e := repo.ListActiveDeployments(ctx)
			return e
		})
		assertCleanErr(t, "LatestRelease", func() error {
			_, e := repo.LatestRelease(ctx, otaprotocol.OSAndroid, "OrangePi5Max")
			return e
		})
		assertCleanErr(t, "TelemetryEventCounts", func() error {
			_, e := repo.TelemetryEventCounts(ctx)
			return e
		})
		assertCleanErr(t, "DeviceStateCounts", func() error {
			_, e := repo.DeviceStateCounts(ctx)
			return e
		})
		assertCleanErr(t, "ListGroups", func() error {
			_, e := repo.ListGroups(ctx)
			return e
		})
		assertCleanErr(t, "ListProjects", func() error {
			_, e := repo.ListProjects(ctx)
			return e
		})
		assertCleanErr(t, "ListAudit", func() error {
			_, _, e := repo.ListAudit(ctx, AuditFilter{})
			return e
		})
		// QueryRow-based scanners: GetDevice/GetArtifact/etc must surface a clean
		// (non-ErrNoRows) driver error on the dead connection.
		assertCleanErr(t, "GetDevice", func() error {
			_, e := repo.GetDevice(ctx, "d-0")
			return e
		})
		assertCleanErr(t, "GetArtifact", func() error {
			_, e := repo.GetArtifact(ctx, "a-0")
			return e
		})
		// Exec-based writers must also surface clean errors (Create/Update paths).
		assertCleanErr(t, "CreateDevice", func() error {
			return repo.CreateDevice(ctx, Device{DeviceID: "x", HardwareID: "hx",
				OSType: otaprotocol.OSAndroid, RegisteredAt: time.Now()})
		})
		assertCleanErr(t, "AppendTelemetry", func() error {
			return repo.AppendTelemetry(ctx, TelemetryRecord{DeviceID: "d-0",
				Event: otaprotocol.TelemetryEvent("install_started")})
		})
	})

	// --- FAULT CASE 2: kill the connection MID-ITERATION so a streaming cursor's
	// next read fails -> the rows.Scan / rows.Err error-return branches inside the
	// List* loops. We drive a real cursor via pool.Query, read one row, arm the
	// fault, then keep reading: pgx surfaces the broken stream through rows.Err. ---
	t.Run("mid_iteration_rows_err", func(t *testing.T) {
		p := newFaultProxy(t, "localhost:"+pgHostPort)
		pool := mustPoolDSN(t, ctx, p.dsn())
		defer pool.Close()

		// Open a streaming cursor over the 500 seeded rows.
		rows, qerr := pool.Query(ctx, deviceSelect+` ORDER BY device_id`)
		if qerr != nil {
			t.Fatalf("open cursor via proxy: %v", qerr)
		}
		// Read a couple of rows to prove the stream is live.
		read := 0
		for read < 2 && rows.Next() {
			read++
		}
		if read < 1 {
			rows.Close()
			t.Fatalf("expected to read at least 1 row before fault, got %d", read)
		}

		p.Fault() // slam the TCP stream mid-cursor

		// Drain: subsequent Next() must stop and rows.Err() must report the broken
		// stream. This is exactly the `if err := rows.Err(); err != nil` branch the
		// production List* loops guard with.
		for rows.Next() {
			read++
		}
		rows.Close()
		if e := rows.Err(); e == nil {
			t.Fatalf("rows.Err() after mid-iteration kill must be non-nil (read=%d)", read)
		} else {
			t.Logf("mid-iteration rows.Err captured cleanly after %d rows: %v", read, e)
		}

		// And a fresh production List* on the now-faulted pool surfaces a clean
		// error (covers the same loop's error returns end-to-end).
		repo := NewPostgresRepositoryFromPool(pool)
		assertCleanErr(t, "ListDevices(post-mid-kill)", func() error {
			_, _, e := repo.ListDevices(ctx, DeviceFilter{})
			return e
		})
	})

	// --- FAULT CASE 3: pool-acquire failure — a pool whose proxy is faulted from
	// the very first dial cannot acquire a connection; Query returns the error. ---
	t.Run("pool_acquire_failure", func(t *testing.T) {
		p := newFaultProxy(t, "localhost:"+pgHostPort)
		// Build a pool WITHOUT pinging (lazy), arm fault, then first use fails.
		pool, perr := newLazyPool(ctx, p.dsn())
		if perr != nil {
			t.Fatalf("construct lazy pool: %v", perr)
		}
		defer pool.Close()
		p.Fault() // refuse every connection from now on

		repo := NewPostgresRepositoryFromPool(pool)
		assertCleanErr(t, "ListDevices(acquire-fail)", func() error {
			_, _, e := repo.ListDevices(ctx, DeviceFilter{})
			return e
		})
		assertCleanErr(t, "GetDevice(acquire-fail)", func() error {
			_, e := repo.GetDevice(ctx, "d-0")
			return e
		})
	})
}

// seedManyDevices inserts n devices so List* queries stream across multiple TCP
// reads (making the mid-iteration kill land inside the cursor, not after EOF).
func seedManyDevices(t *testing.T, ctx context.Context, repo *PostgresRepository, n int) {
	t.Helper()
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		id := "d-" + itoa(i)
		d := Device{
			DeviceID: id, HardwareID: "HW-" + itoa(i), Model: "OrangePi5Max",
			OSType: otaprotocol.OSAndroid, UpdateState: "idle", RegisteredAt: ts,
			Metadata: map[string]string{"seq": itoa(i)},
		}
		if err := repo.CreateDevice(ctx, d); err != nil {
			t.Fatalf("seed device %s: %v", id, err)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// assertCleanErr asserts fn() returns a non-nil error (the driver fault) and did
// not panic. ErrNotFound is NOT acceptable here — a dead connection must surface
// a real driver error, never be masked as not-found.
func assertCleanErr(t *testing.T, name string, fn func() (err error)) {
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
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("%s masked a driver fault as ErrNotFound: %v", name, err)
	}
	t.Logf("%s surfaced clean driver error: %v", name, err)
}
