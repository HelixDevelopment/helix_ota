package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
)

// selfSignedTLS builds a TLS 1.3 config with a self-signed cert valid for
// 127.0.0.1, for the local dual-transport test.
func selfSignedTLS(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "helix-ota-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS13,
	}
}

// freePort returns a port free for the test (grabbed via an ephemeral TCP bind).
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

// TestDualTransportServesH3AndH2 is the cross-backend proof: the SAME handler is
// reachable over a real HTTP/3 (QUIC) client AND a real HTTP/2 client (the
// fallback), both returning 200 with the expected body, and the HTTP/2 response
// advertises HTTP/3 via Alt-Svc.
func TestDualTransportServesH3AndH2(t *testing.T) {
	port := freePort(t)
	addr := "127.0.0.1:" + port

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

	waitForListeners(t, addr)

	url := "https://" + addr + "/probe"

	// --- HTTP/3 (QUIC) path ---
	h3cli := &http.Client{
		Transport: &http3.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}},
		},
		Timeout: 5 * time.Second,
	}
	defer h3cli.CloseIdleConnections()
	h3resp, body := getOK(t, h3cli, url, "h3")
	if h3resp.ProtoMajor != 3 {
		t.Fatalf("h3 request used proto %d (want 3)", h3resp.ProtoMajor)
	}
	if body != "helix-ok" {
		t.Fatalf("h3 body = %q", body)
	}

	// --- HTTP/2 (TCP) fallback path ---
	h2cli := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2", "http/1.1"}},
		},
		Timeout: 5 * time.Second,
	}
	defer h2cli.CloseIdleConnections()
	h2resp, body2 := getOK(t, h2cli, url, "h2")
	if h2resp.ProtoMajor != 2 {
		t.Fatalf("h2 fallback used proto %d (want 2)", h2resp.ProtoMajor)
	}
	if body2 != "helix-ok" {
		t.Fatalf("h2 body = %q", body2)
	}
	// The fallback response advertises HTTP/3 for discovery/upgrade.
	if alt := h2resp.Header.Get("Alt-Svc"); alt == "" {
		t.Fatalf("h2 response missing Alt-Svc (HTTP/3 advertisement)")
	}
}

func getOK(t *testing.T, cli *http.Client, url, label string) (*http.Response, string) {
	t.Helper()
	resp, err := cli.Get(url)
	if err != nil {
		t.Fatalf("%s GET: %v", label, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s status %d", label, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// waitForListeners blocks until both the TCP and UDP sockets accept dials.
func waitForListeners(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for _, network := range []string{"tcp", "udp"} {
		for {
			c, err := net.DialTimeout(network, addr, 200*time.Millisecond)
			if err == nil {
				c.Close()
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s listener at %s never came up: %v", network, addr, err)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
}
