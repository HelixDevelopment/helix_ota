package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

// newCompressionEngine builds a minimal gin engine with only the compression
// middleware and a fixed-body handler, isolating the negotiation behaviour.
func newCompressionEngine(body string) *gin.Engine {
	r := gin.New()
	r.Use(compressionMiddleware())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, body) })
	r.GET("/empty", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func doEnc(r *gin.Engine, acceptEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCompressionNegotiatesBrotli(t *testing.T) {
	body := "helix ota control-plane response body that is worth compressing"
	w := doEnc(newCompressionEngine(body), "br")
	if got := w.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("want Content-Encoding br, got %q", got)
	}
	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("Vary: Accept-Encoding must be set")
	}
	dec, err := io.ReadAll(brotli.NewReader(w.Body))
	if err != nil {
		t.Fatalf("brotli decode: %v", err)
	}
	if string(dec) != body {
		t.Fatalf("brotli body mismatch: %q", string(dec))
	}
}

func TestCompressionFallsBackToGzip(t *testing.T) {
	body := "fallback body for clients that speak gzip but not brotli"
	// br explicitly disabled (q=0); gzip offered -> gzip chosen.
	w := doEnc(newCompressionEngine(body), "br;q=0, gzip")
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("want Content-Encoding gzip, got %q", got)
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	dec, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip decode: %v", err)
	}
	if string(dec) != body {
		t.Fatalf("gzip body mismatch: %q", string(dec))
	}
}

func TestCompressionGzipWhenNoBrotli(t *testing.T) {
	body := "gzip-only client"
	w := doEnc(newCompressionEngine(body), "gzip, deflate")
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("want gzip, got %q", got)
	}
}

func TestCompressionIdentityFallback(t *testing.T) {
	body := "client supports no recognised encoding"
	for _, ae := range []string{"", "identity", "deflate", "br;q=0, gzip;q=0"} {
		w := doEnc(newCompressionEngine(body), ae)
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("AE=%q: want no Content-Encoding (identity), got %q", ae, got)
		}
		if w.Body.String() != body {
			t.Fatalf("AE=%q: identity body mismatch: %q", ae, w.Body.String())
		}
		if w.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("AE=%q: Vary must still be set", ae)
		}
	}
}

// TestCompressionPanicErrorBodyDecodable reproduces the panic-path resource/
// silent-failure defect in compressionMiddleware. The middleware installs a
// body-compressing writer, then closes the compressor with a PLAIN statement
// AFTER c.Next(). When a downstream handler panics, that unwind jumps over the
// close, and the outer recoveryMiddleware (which wraps compression in the real
// server chain, server.go) then writes the 500 error envelope THROUGH the
// still-open compressor. The compressor is never flushed/closed, so a client
// that negotiated gzip receives a `Content-Encoding: gzip` response whose body
// is a truncated, undecodable gzip stream — the error envelope never reaches the
// client (a §11.4 silent failure at the middleware layer).
//
// This drives the REAL production wiring order (recovery -> compression ->
// panicking handler) and asserts the invariant every client relies on: the 500
// error body is decodable per its declared Content-Encoding and carries the
// INTERNAL error code. RED on the pre-fix code (truncated gzip → decode fails);
// GREEN once the compressor is closed on every exit path and the plain writer is
// restored for the recovery-written error.
func TestCompressionPanicErrorBodyDecodable(t *testing.T) {
	r := gin.New()
	// Production order: recovery is OUTER, compression is INNER (see server.go
	// r.Use(recoveryMiddleware(), ..., compressionMiddleware())).
	r.Use(recoveryMiddleware(), compressionMiddleware())
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic must surface as 500, got %d", w.Code)
	}

	// The client MUST be able to decode the error body per whatever encoding the
	// server declared (identity or a valid, fully-closed gzip stream).
	var body io.Reader = w.Body
	if enc := w.Header().Get("Content-Encoding"); enc == "gzip" {
		zr, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("Content-Encoding: gzip but body is not a decodable gzip stream: %v", err)
		}
		body = zr
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("500 error body could not be read/decoded (truncated compressor): %v", err)
	}
	if !strings.Contains(string(raw), CodeInternal) {
		t.Fatalf("500 error body must carry the %q code; got %q", CodeInternal, string(raw))
	}
}

func TestCompression204NoBodyNotEncoded(t *testing.T) {
	// A 204 (no body) must not gain a Content-Encoding or a compressed empty
	// stream — the lazy writer only engages on a real Write.
	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	req.Header.Set("Accept-Encoding", "br")
	w := httptest.NewRecorder()
	newCompressionEngine("unused").ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("204 must not be content-encoded, got %q", got)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("204 body must be empty, got %d bytes", w.Body.Len())
	}
}
