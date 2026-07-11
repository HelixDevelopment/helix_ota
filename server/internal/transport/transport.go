// Package transport serves the Helix OTA control-plane handler over the locked
// transport stack (master design §3, ADR-0004): HTTP/3 (QUIC) primary with an
// automatic HTTP/2 (+HTTP/1.1) fallback for clients and networks that cannot
// use QUIC/UDP. Both listeners share the same net/http.Handler and the same
// TLS 1.3 material; HTTP/3 is delivered via the reusable `digital.vasic.http3`
// submodule (a drop-in net/http.Handler server). Responses advertise
// `Alt-Svc: h3` so HTTP/2 clients can discover and upgrade to HTTP/3 — but
// ONLY once the HTTP/3 (QUIC/UDP) listener is actually confirmed bound; see
// h3Ready below. A client is never pointed at a QUIC endpoint that failed to
// come up (HA-2).
package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	h3server "digital.vasic.http3/pkg/server"
)

// h3ReadyGrace bounds how long Start waits, after launching the HTTP/3
// (QUIC/UDP) listener, before treating it as successfully bound.
//
// Honest §11.4.3/§11.4.6 boundary: the vendored h3server.Server exposes no
// positive "listener bound" signal separate from Start() returning — Start()
// blocks for the server's ENTIRE lifetime and only returns when the server
// stops (cleanly or with an error), so we cannot simply await "success" on
// that call. What IS true of the underlying quic-go http3.Server.ListenAndServe
// (which h3server.Server.Start calls into) is that a UDP-bind or TLS-config
// failure is returned SYNCHRONOUSLY — before the blocking serve loop is ever
// entered (see quic-go/http3.Server.setupListenerForConn: it calls
// quic.ListenAddrEarly and returns its error immediately, with no I/O wait).
// So any error surfacing within this short grace window is a genuine
// bind-time failure, not a later runtime one. This is therefore a BOUNDED
// HEURISTIC, not a positive confirmation from the vendored API — the best
// available signal given that API's shape, not a claim of certainty.
const h3ReadyGrace = 50 * time.Millisecond

// Config configures the dual-stack transport.
type Config struct {
	// Addr is the host:port used for BOTH the TCP (HTTP/2) and UDP (HTTP/3)
	// listeners, e.g. ":8443" or "127.0.0.1:8443".
	Addr string
	// Handler is the control-plane handler (the Gin engine).
	Handler http.Handler
	// TLSConf must carry a certificate; TLS 1.3 is enforced (HTTP/3 mandate).
	TLSConf *tls.Config
}

// Server runs the HTTP/2 and HTTP/3 listeners together.
type Server struct {
	addr string
	h2   *http.Server
	h3   *h3server.Server

	// h3Ready is set true once Start confirms (within h3ReadyGrace) that the
	// HTTP/3 (QUIC/UDP) listener bound successfully, and cleared back to
	// false the moment h3 stops (cleanly or with an error). altSvcHandler
	// consults it on every request and only advertises Alt-Svc while true —
	// never unconditionally.
	h3Ready *atomic.Bool
}

// New builds the dual transport. The HTTP/2 server and the HTTP/3 server each
// receive their own ALPN-scoped TLS config clone (h2/http1.1 over TCP, h3 over
// UDP) so neither advertises the other's protocol on the wrong socket.
func New(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		return nil, errors.New("transport: Addr is required")
	}
	if cfg.Handler == nil {
		return nil, errors.New("transport: Handler is required")
	}
	if cfg.TLSConf == nil {
		return nil, errors.New("transport: TLSConf is required (TLS 1.3 mandated)")
	}

	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("transport: bad Addr %q: %w", cfg.Addr, err)
	}
	h3Ready := new(atomic.Bool)
	handler := altSvcHandler(cfg.Handler, port, h3Ready)

	// HTTP/3 (UDP): the submodule clones + enforces TLS 1.3; we set the h3 ALPN.
	h3TLS := cfg.TLSConf.Clone()
	h3TLS.MinVersion = tls.VersionTLS13
	h3TLS.NextProtos = []string{"h3"}
	h3, err := h3server.New(h3server.Config{Addr: cfg.Addr, Handler: handler, TLSConf: h3TLS})
	if err != nil {
		return nil, fmt.Errorf("transport: http3: %w", err)
	}

	// HTTP/2 + HTTP/1.1 (TCP). Empty NextProtos lets the stdlib negotiate
	// h2/http1.1 via ALPN (http.Server auto-configures HTTP/2 over TLS).
	h2TLS := cfg.TLSConf.Clone()
	h2TLS.MinVersion = tls.VersionTLS13
	h2TLS.NextProtos = nil
	h2 := &http.Server{Addr: cfg.Addr, Handler: handler, TLSConfig: h2TLS}

	return &Server{addr: cfg.Addr, h2: h2, h3: h3, h3Ready: h3Ready}, nil
}

// Start runs both listeners concurrently and blocks until one returns. A clean
// Shutdown of the HTTP/2 server returns http.ErrServerClosed, which Start maps
// to nil so a graceful stop is not reported as a failure.
//
// The HTTP/3 (QUIC/UDP) bind happens in its own goroutine, racing the real
// h3.Start() error against the bounded h3ReadyGrace window (see its doc
// comment for why a race is the best available signal here): if h3.Start()
// fails first, s.h3Ready is never set — Alt-Svc is simply never advertised
// for a QUIC endpoint that never came up. If the grace window elapses first,
// s.h3Ready flips true so HTTP/2 responses start advertising HTTP/3; it is
// flipped back to false the moment h3 eventually stops, for any reason.
func (s *Server) Start() error {
	errCh := make(chan error, 2)

	h3Done := make(chan error, 1)
	go func() { h3Done <- s.h3.Start() }()
	go func() {
		select {
		case err := <-h3Done:
			// h3 failed before the grace window even elapsed — never ready.
			errCh <- err
			return
		case <-time.After(h3ReadyGrace):
			s.h3Ready.Store(true)
		}
		err := <-h3Done
		s.h3Ready.Store(false)
		errCh <- err
	}()

	go func() {
		if err := s.h2.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	return <-errCh
}

// Shutdown gracefully stops both listeners.
func (s *Server) Shutdown(ctx context.Context) error {
	h2Err := s.h2.Shutdown(ctx)
	h3Err := s.h3.Shutdown(ctx)
	if h2Err != nil {
		return h2Err
	}
	return h3Err
}

// altSvcHandler advertises HTTP/3 availability so HTTP/2 / HTTP/1.1 clients can
// discover the QUIC endpoint and upgrade on a subsequent request — but ONLY
// while h3Ready reports the HTTP/3 (QUIC/UDP) listener as actually up (HA-2:
// advertising a dead QUIC endpoint sends HTTP/3-capable clients into a
// repeated-failed-upgrade loop).
func altSvcHandler(next http.Handler, port string, h3Ready *atomic.Bool) http.Handler {
	altSvc := fmt.Sprintf(`h3=":%s"; ma=86400`, port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h3Ready.Load() {
			w.Header().Set("Alt-Svc", altSvc)
		}
		next.ServeHTTP(w, r)
	})
}
