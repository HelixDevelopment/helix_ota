//go:build integration

// faultproxy: in-process TCP fault-injection proxy between a pgxpool and the
// REAL brick-booted PostgreSQL, used to deterministically trigger the driver
// fault branches in rollout/postgres.go (tx Begin/Exec/Commit failure, Load's
// QueryRow.Scan failure, the phases-cursor rows.Err return) that a healthy DB
// cannot reach. Same pattern as store/faultproxy_test.go (kept per-package to
// preserve §11.4.28 decoupling — neither package imports the other's test code).
package rollout

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type faultProxy struct {
	ln       net.Listener
	upstream string

	mu      sync.Mutex
	faulted bool
	conns   map[net.Conn]struct{}
	wg      sync.WaitGroup
	closed  bool
}

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

func (p *faultProxy) addr() string { return p.ln.Addr().String() }

func (p *faultProxy) dsn() string {
	host, port, _ := net.SplitHostPort(p.addr())
	return "postgres://helix:helix@" + host + ":" + port + "/helix_ota?sslmode=disable"
}

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
			return
		}
		if p.isFaulted() {
			_ = c.Close()
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
	_ = client.Close()
	_ = up.Close()
}

func (p *faultProxy) track(a, b net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.faulted {
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

// Fault arms the proxy: closes every live connection and rejects new ones.
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
// (avoids the resetSchema one-shot-connect race against the boot healthcheck).
func resetSchemaVia(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS helix_ota CASCADE"); err != nil {
		t.Fatalf("drop schema via ready pool: %v", err)
	}
}

func newLazyPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

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
