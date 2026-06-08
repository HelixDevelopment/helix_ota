package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
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
