package transport

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestNewValidation covers every guard in New: missing Addr, missing Handler,
// missing TLSConf, and a malformed Addr that fails net.SplitHostPort.
func TestNewValidation(t *testing.T) {
	okHandler := http.NewServeMux()
	tlsConf := selfSignedTLS(t)

	cases := []struct {
		name    string
		cfg     Config
		wantSub string
	}{
		{"no-addr", Config{Handler: okHandler, TLSConf: tlsConf}, "Addr is required"},
		{"no-handler", Config{Addr: "127.0.0.1:0", TLSConf: tlsConf}, "Handler is required"},
		{"no-tls", Config{Addr: "127.0.0.1:0", Handler: okHandler}, "TLSConf is required"},
		{"bad-addr", Config{Addr: "not-a-host-port", Handler: okHandler, TLSConf: tlsConf}, "bad Addr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, err := New(c.cfg)
			if err == nil || srv != nil {
				t.Fatalf("New(%s) want error, got srv=%v err=%v", c.name, srv, err)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("New(%s) error %q missing %q", c.name, err, c.wantSub)
			}
		})
	}
}

// TestStartReturnsH2BindError proves Start surfaces a real HTTP/2 (TCP) bind
// failure rather than swallowing it: when the TCP port is already held by another
// listener, ListenAndServeTLS fails with a non-ErrServerClosed error and Start
// returns it (covering the error-propagation branch in Start, not the
// ErrServerClosed->nil mapping). The HTTP/3 (UDP) socket on the same numeric port
// binds independently, so the failure observed is the H2 one.
func TestStartReturnsH2BindError(t *testing.T) {
	// Hold the TCP port so the server's H2 listener cannot bind it.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("blocker listen: %v", err)
	}
	defer blocker.Close()
	_, port, _ := net.SplitHostPort(blocker.Addr().String())
	addr := "127.0.0.1:" + port

	srv, err := New(Config{Addr: addr, Handler: http.NewServeMux(), TLSConf: selfSignedTLS(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("Start should have returned the H2 bind error, got nil")
		}
		if !strings.Contains(err.Error(), "address already in use") &&
			!strings.Contains(err.Error(), "bind") {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Start did not return the H2 bind error within timeout")
	}
}

// TestShutdownBeforeStart proves Shutdown is safe (returns nil) on a freshly-built
// server whose listeners never started — both h2.Shutdown and h3.Shutdown report
// clean, exercising the success path of both branches in Shutdown.
func TestShutdownBeforeStart(t *testing.T) {
	port := freePort(t)
	srv, err := New(Config{Addr: "127.0.0.1:" + port, Handler: http.NewServeMux(), TLSConf: selfSignedTLS(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown before Start should be nil, got %v", err)
	}
}

// TestAltSvcHandlerSetsHeader directly exercises altSvcHandler: it must set an
// Alt-Svc header advertising HTTP/3 on the configured port and still delegate to
// the wrapped handler. A regression dropping the header or the delegation fails.
func TestAltSvcHandlerSetsHeader(t *testing.T) {
	var inner bool
	wrapped := altSvcHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner = true
		w.WriteHeader(http.StatusTeapot)
	}), "9443")
	rec := newRecorder()
	wrapped.ServeHTTP(rec, &http.Request{})
	if !inner {
		t.Fatalf("altSvcHandler did not call the wrapped handler")
	}
	if got := rec.Header().Get("Alt-Svc"); got != `h3=":9443"; ma=86400` {
		t.Fatalf("Alt-Svc = %q", got)
	}
	if rec.code != http.StatusTeapot {
		t.Fatalf("wrapped status not propagated: %d", rec.code)
	}
}

// minimal ResponseWriter recorder (avoids importing httptest just for this).
type recorder struct {
	hdr  http.Header
	code int
}

func newRecorder() *recorder { return &recorder{hdr: make(http.Header)} }

func (r *recorder) Header() http.Header         { return r.hdr }
func (r *recorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *recorder) WriteHeader(c int)           { r.code = c }
