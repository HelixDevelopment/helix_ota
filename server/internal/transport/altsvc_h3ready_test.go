package transport

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestAltSvcNotAdvertisedWhenH3FailsToBind is the RED/GREEN regression proof
// (§11.4.115 polarity) for HA-2: the Alt-Svc h3 header MUST NOT be advertised
// when the HTTP/3 (QUIC/UDP) listener failed to bind, even though the
// TCP/H2 listener on the SAME numeric port binds and serves fine — the exact
// scenario the finding describes (UDP port conflict / container net policy /
// no permission, while TCP works). The UDP port is deliberately pre-held so
// quic-go's own bind (inside s.h3.Start()) fails with "address already in
// use", the same synchronous, fast failure mode a real port conflict
// produces.
//
// Anti-tautology (§11.4.115): reverting the h3Ready gate in altSvcHandler
// (writing the Alt-Svc header unconditionally again) makes this test FAIL;
// restoring the gate makes it PASS. See the fix commit/report for the
// captured `go test -run` output both ways.
func TestAltSvcNotAdvertisedWhenH3FailsToBind(t *testing.T) {
	port := freePort(t)
	addr := "127.0.0.1:" + port

	// Pre-bind the UDP port so the HTTP/3 (QUIC) listener cannot bind it.
	udpBlocker, err := net.ListenPacket("udp", addr)
	if err != nil {
		t.Fatalf("pre-bind UDP port %s: %v", addr, err)
	}
	defer udpBlocker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "helix-ok")
	})

	srv, err := New(Config{Addr: addr, Handler: mux, TLSConf: selfSignedTLS(t)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go func() { _ = srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	// Wait for the TCP (H2) listener only — the UDP one is deliberately dead
	// for this test, and a UDP "dial" (connect()) succeeds locally even when
	// nothing is listening on the other end, so it would prove nothing here.
	waitForTCPListener(t, addr)
	// Let the h3Ready grace window (and the h3 bind failure racing inside
	// it) fully settle so the assertion below reflects steady state, not a
	// race against Start()'s internal goroutines.
	time.Sleep(h3ReadyGrace + 200*time.Millisecond)

	h2cli := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2", "http/1.1"}},
		},
		Timeout: 5 * time.Second,
	}
	defer h2cli.CloseIdleConnections()

	resp, err := h2cli.Get("https://" + addr + "/probe")
	if err != nil {
		t.Fatalf("h2 GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("h2 status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "helix-ok" {
		t.Fatalf("h2 body = %q", body)
	}
	if resp.ProtoMajor != 2 {
		t.Fatalf("h2 request used proto %d (want 2)", resp.ProtoMajor)
	}
	if alt := resp.Header.Get("Alt-Svc"); alt != "" {
		t.Fatalf("h2 response advertised Alt-Svc %q while HTTP/3 (QUIC) failed to bind (UDP port held by %s)", alt, udpBlocker.LocalAddr())
	}
}

// waitForTCPListener blocks until the TCP socket at addr accepts dials. This
// is a TCP-only variant of transport_test.go's waitForListeners, needed here
// because that helper also dials UDP — and a UDP "dial" (connect()) succeeds
// locally even when nothing is listening on the other end, so it would not
// actually wait for anything in a test that deliberately keeps HTTP/3 down.
func waitForTCPListener(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tcp listener at %s never came up: %v", addr, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
