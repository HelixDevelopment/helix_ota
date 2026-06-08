package api

import (
	"compress/gzip"
	"io"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

// compressionMiddleware negotiates response content compression per the locked
// stack (master design §3): Brotli when the client offers `br`, gzip as the
// fallback for clients that do not support Brotli, and identity (no encoding)
// when neither is acceptable. `Vary: Accept-Encoding` is always set so caches
// key on the negotiated encoding.
//
// The wrapping writer is LAZY: it only sets Content-Encoding and engages the
// compressor on the first real body Write, so empty-body responses (204) are
// never content-encoded.
func compressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Vary", "Accept-Encoding")
		ae := c.GetHeader("Accept-Encoding")

		var enc string
		var newW func(io.Writer) io.WriteCloser
		switch {
		case acceptsEncoding(ae, "br"):
			enc = "br"
			newW = func(w io.Writer) io.WriteCloser { return brotli.NewWriter(w) }
		case acceptsEncoding(ae, "gzip"):
			enc = "gzip"
			newW = func(w io.Writer) io.WriteCloser { return gzip.NewWriter(w) }
		default:
			c.Next() // identity — no compression
			return
		}

		cw := &compressWriter{ResponseWriter: c.Writer, enc: enc, newW: newW}
		c.Writer = cw
		c.Next()
		_ = cw.Close()
	}
}

// compressWriter wraps a gin.ResponseWriter and pipes the body through a
// compressor, initialised lazily on first Write.
type compressWriter struct {
	gin.ResponseWriter
	enc  string
	newW func(io.Writer) io.WriteCloser
	cw   io.WriteCloser
}

func (w *compressWriter) ensure() {
	if w.cw == nil {
		h := w.ResponseWriter.Header()
		h.Set("Content-Encoding", w.enc)
		// Length changes under compression; let the transport chunk it.
		h.Del("Content-Length")
		w.cw = w.newW(w.ResponseWriter)
	}
}

func (w *compressWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	w.ensure()
	return w.cw.Write(b)
}

func (w *compressWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Close flushes the compressor if it was engaged.
func (w *compressWriter) Close() error {
	if w.cw != nil {
		return w.cw.Close()
	}
	return nil
}

// acceptsEncoding reports whether the Accept-Encoding header offers enc with a
// non-zero q-value. A `*` wildcard with non-zero q also matches.
func acceptsEncoding(header, enc string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := part
		q := 1.0
		if i := strings.Index(part, ";"); i >= 0 {
			name = strings.TrimSpace(part[:i])
			for _, p := range strings.Split(part[i+1:], ";") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "q=") {
					if v, err := strconv.ParseFloat(strings.TrimPrefix(p, "q="), 64); err == nil {
						q = v
					}
				}
			}
		}
		if (name == enc || name == "*") && q > 0 {
			return true
		}
	}
	return false
}
