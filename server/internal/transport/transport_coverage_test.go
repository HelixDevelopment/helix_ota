package transport

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewWrapsHTTP3ConstructionError covers the h3server.New error branch in
// New (transport.go:64-66). The transport's own guards (Addr, Handler, TLSConf)
// only check for nil/empty; a non-nil TLSConf that carries neither Certificates
// nor GetCertificate passes those guards but is rejected by the underlying
// digital.vasic.http3 server. New must surface that as a wrapped
// "transport: http3:" error rather than returning a half-built Server.
func TestNewWrapsHTTP3ConstructionError(t *testing.T) {
	// Valid Addr + Handler, but a TLS config with no usable certificate.
	badTLS := &tls.Config{MinVersion: tls.VersionTLS13}

	srv, err := New(Config{
		Addr:    "127.0.0.1:0",
		Handler: http.NewServeMux(),
		TLSConf: badTLS,
	})
	if err == nil || srv != nil {
		t.Fatalf("New want error from http3 construction, got srv=%v err=%v", srv, err)
	}
	if !strings.Contains(err.Error(), "transport: http3:") {
		t.Fatalf("error %q missing the wrapped http3 prefix", err)
	}
	// Surfaces the underlying cert requirement. Assert on the two stable
	// cert-source terms independently rather than an exact phrasing: http3's
	// Validate message now also lists GetConfigForClient as a valid source
	// (submodules/http3 a56d040), so it reads "Certificates, GetCertificate, or
	// GetConfigForClient" — re-pinning the whole sentence would just break again
	// on the next legitimate reword, while these two terms robustly prove the
	// wrapped error names the cert requirement.
	if !strings.Contains(err.Error(), "Certificates") || !strings.Contains(err.Error(), "GetCertificate") {
		t.Fatalf("error %q does not surface the underlying cert requirement", err)
	}
}

// TestShutdownReturnsHTTP2Error covers the h2Err != nil branch in Shutdown
// (transport.go:98-100). http.Server.Shutdown returns ctx.Err() only when there
// are live listeners/connections still draining when the context expires (a
// never-started server short-circuits to nil). So we start the real dual
// transport, hold an in-flight HTTP/2 request open, then call Shutdown with a
// short deadline: the HTTP/2 server's graceful drain cannot complete in time
// and Shutdown surfaces that error (taking precedence over the HTTP/3 result
// per the documented ordering).
func TestShutdownReturnsHTTP2Error(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)

	handlerEntered := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/block", func(w http.ResponseWriter, r *http.Request) {
		select {
		case handlerEntered <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release // hold the connection open so graceful drain stalls
	})

	port := freePort(t)
	addr := "127.0.0.1:" + port
	srv, err := New(Config{Addr: addr, Handler: mux, TLSConf: selfSignedTLS(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = srv.Start() }()
	waitForListeners(t, addr)

	// Drive a real HTTP/2 request that parks in the blocking handler.
	h2cli := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2", "http/1.1"}},
		},
		Timeout: 10 * time.Second,
	}
	defer h2cli.CloseIdleConnections()

	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		resp, err := h2cli.Get("https://" + addr + "/block")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	select {
	case <-handlerEntered:
	case <-time.After(6 * time.Second):
		closeRelease()
		t.Fatalf("HTTP/2 handler never entered; cannot stall graceful drain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err == nil {
		closeRelease()
		t.Fatalf("Shutdown should return the HTTP/2 drain error while a request is held, got nil")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		closeRelease()
		t.Fatalf("Shutdown error %q is not the deadline-exceeded HTTP/2 error", err)
	}

	closeRelease()
	select {
	case <-reqDone:
	case <-time.After(4 * time.Second):
	}
}
