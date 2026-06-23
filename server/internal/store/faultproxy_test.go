//go:build integration

// faultproxy is a tiny in-process TCP fault-injection proxy that sits between a
// pgxpool and the real PostgreSQL booted on-demand via the containers brick. It
// forwards bytes in both directions until armed; once armed, it kills every
// active connection (closing both halves) and refuses or kills every new one.
// Pointing a pool's DSN at the proxy and arming it mid-query deterministically
// triggers the driver-level fault branches that are genuinely unreachable
// against a healthy DB (rows.Scan / rows.Err error returns, pool.Query
// connection-failure, tx Exec/Commit failure) — the residual sub-100% pgx
// coverage the integration push (store 92.4% / rollout 87.3%) flagged as honest
// driver-fault gaps (§11.4.6 — these are NOT bluffed shut, they are exercised
// with REAL captured connection faults per §11.4.69/§11.4.85 chaos).
//
// This is faithful to §11.4.74 (do not reimplement orchestration): the proxy is
// NOT a container runtime — it is a test-local network shim in front of the real
// brick-booted Postgres, the standard in-process toxiproxy-equivalent pattern.
package store

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// faultProxy forwards TCP between a local listener and an upstream addr. Calling
// Fault() (idempotent) closes all live conns and makes the proxy reject/kill
// everything thereafter, simulating an upstream/connection collapse mid-flight.
type faultProxy struct {
	ln       net.Listener
	upstream string

	mu      sync.Mutex
	faulted bool
	conns   map[net.Conn]struct{} // live client + upstream conns to slam shut
	wg      sync.WaitGroup
	closed  bool
}

// newFaultProxy starts a proxy listening on an ephemeral localhost port that
// forwards to upstream (e.g. "localhost:55432"). t.Cleanup stops it.
func newFaultProxy(t *testing.T, upstream string) *faultProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("faultproxy listen: %v", err)
	}
	p := &faultProxy{ln: ln, upstream: upstream, conns: map[net.Conn]struct{}{}}
	p.wg.Add(1)
	go p.acceptLoop()
	t.Cleanup(p.stop)
	return p
}

// addr is the host:port the pool should dial.
func (p *faultProxy) addr() string { return p.ln.Addr().String() }

// dsn builds a pgx DSN pointing at the proxy with the standard creds/db.
func (p *faultProxy) dsn() string {
	host, port, _ := net.SplitHostPort(p.addr())
	return "postgres://helix:helix@" + host + ":" + port + "/helix_ota?sslmode=disable"
}

// isFaulted reports the current armed state under lock.
func (p *faultProxy) isFaulted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.faulted
}

func (p *faultProxy) acceptLoop() {
	defer p.wg.Done()
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return // listener closed
		}
		if p.isFaulted() {
			_ = c.Close() // refuse new connections once faulted (pool-acquire failure)
			continue
		}
		p.wg.Add(1)
		go p.handle(c)
	}
}

func (p *faultProxy) handle(client net.Conn) {
	defer p.wg.Done()
	up, err := net.DialTimeout("tcp", p.upstream, 5*time.Second)
	if err != nil {
		_ = client.Close()
		return
	}
	p.track(client, up)
	defer p.untrack(client, up)

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, up); done <- struct{}{} }()
	<-done
	// Either side closed (real EOF or a Fault() slam). Tear both down.
	_ = client.Close()
	_ = up.Close()
}

func (p *faultProxy) track(a, b net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.faulted { // raced with Fault(): slam immediately
		_ = a.Close()
		_ = b.Close()
		return
	}
	p.conns[a] = struct{}{}
	p.conns[b] = struct{}{}
}

func (p *faultProxy) untrack(a, b net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.conns, a)
	delete(p.conns, b)
}

// Fault arms the proxy: closes every live connection (a hard mid-query
// connection kill) and makes all subsequent connections fail. Idempotent.
func (p *faultProxy) Fault() {
	p.mu.Lock()
	p.faulted = true
	live := make([]net.Conn, 0, len(p.conns))
	for c := range p.conns {
		live = append(live, c)
	}
	p.conns = map[net.Conn]struct{}{}
	p.mu.Unlock()
	for _, c := range live {
		_ = c.Close()
	}
}

func (p *faultProxy) stop() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.faulted = true
	live := make([]net.Conn, 0, len(p.conns))
	for c := range p.conns {
		live = append(live, c)
	}
	p.conns = map[net.Conn]struct{}{}
	p.mu.Unlock()
	_ = p.ln.Close()
	for _, c := range live {
		_ = c.Close()
	}
	p.wg.Wait()
}

// resetSchemaVia drops the helix_ota schema through an already-query-ready pool
// (avoids the resetSchema one-shot-connect race where the boot TCP healthcheck
// passes before Postgres accepts queries).
func resetSchemaVia(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS helix_ota CASCADE"); err != nil {
		t.Fatalf("drop schema via ready pool: %v", err)
	}
}

// newLazyPool constructs a pool from a DSN WITHOUT an eager ping, so the first
// real query is what hits the (now-faulted) proxy — exercising the pool-acquire
// failure path rather than failing at construction.
func newLazyPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// mustPoolDSN opens a pool against an arbitrary DSN (the proxy), retrying until
// it accepts a query. Mirrors mustPool but takes the DSN so the fault tests can
// point at the proxy rather than the fixed direct port.
func mustPoolDSN(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()
	var lastErr error
	for i := 0; i < 60; i++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if perr := pool.Ping(ctx); perr == nil {
				return pool
			} else {
				lastErr = perr
				pool.Close()
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for postgres via proxy: %v", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("postgres never became query-ready via proxy: %v", lastErr)
	return nil
}
