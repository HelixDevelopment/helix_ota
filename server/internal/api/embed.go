// Package api provides the Gin-based REST API handlers for the Helix OTA
// control plane.  This file embeds the built OTA Manager SPA so the Go server
// can serve it directly, enabling a single-binary deployment without a
// separate Nginx sidecar.
//
// Usage
//
//   1. Build the SPA:
//        cd clients/ota-manager && pnpm build
//
//   2. Copy the built assets into this package:
//        cp -r dist/ ../../server/internal/api/manager-dist/
//
//   3. Rebuild the server — the SPA is compiled into the binary via Go's
//      //go:embed directive:
//        cd server && go build ./cmd/ota-server
//
//   4. The SPA is served at the /manager path.  Visit:
//        http://localhost:8080/manager
//
// Route notes
//
//   - The embedded SPA is mounted at /manager with a fallback handler so that
//     client-side React routes (e.g. /manager/devices, /manager/deployments)
//     resolve correctly.
//   - API requests continue to use the configured API base path
//     (default /api/v1); they are NOT served from within /manager.
//   - When the SPA is deployed via the standalone nginx container (see
//     clients/ota-manager/docker/ota-manager.docker-compose.yml) this file is
//     unused — the Nginx sidecar handles static serving and API proxying.
//
// Security
//
//   - The manager-dist/ directory is gitignored (it is a build artifact, per
//     §11.4.30).  Only the //go:embed directive references it at build time.
//   - No CORS is needed for the embedded path because both the SPA and the API
//     are served from the same Go binary on the same origin.

package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed manager-dist/*
var managerFS embed.FS

// mountManagerUI registers the embedded OTA Manager SPA routes on the
// provided Gin engine.  Call it from Router() after registering the API
// routes.
func (s *Server) MountManagerUI(r *gin.Engine) {
	// Obtain a sub-filesystem rooted at manager-dist/ so that the embedded
	// paths are clean (e.g. "index.html" rather than "manager-dist/index.html").
	sub, err := fs.Sub(managerFS, "manager-dist")
	if err != nil {
		// This should never happen at runtime: the //go:embed directive
		// guarantees the directory exists at build time.
		panic("embed: cannot create fs.Sub for manager-dist: " + err.Error())
	}

	// Tier C (Item O): the SPA document CSP. Gated exactly like HSTS — the
	// `upgrade-insecure-requests` directive is emitted only when TLS is
	// configured, because over plaintext HTTP it would upgrade the same-origin
	// /manager/assets/* subresource loads to https and white-screen the UI on any
	// non-localhost HTTP deployment. See docs/qa/20260710-server-security-headers/
	// EVIDENCE.md for the bundle static-analysis that derived each directive.
	spaCSP := spaDocumentCSP(s.tlsEnabled())

	// Serve the SPA at /manager with a directory index. A group middleware
	// attaches the document CSP; the header is harmless on asset subresource
	// responses (browsers apply CSP only to documents/workers) and load-bearing
	// on the index.html document served at /manager/ and /manager/index.html.
	mgr := r.Group("/manager")
	mgr.Use(func(c *gin.Context) {
		c.Header("Content-Security-Policy", spaCSP)
		c.Next()
	})
	mgr.StaticFS("/", http.FS(sub))

	// SPA fallback: any GET request under /manager that does not match a file
	// serves the SPA's index.html so client-side routing works.
	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		// Only handle GET requests under /manager (or at root when the SPA is the
		// only frontend).  Skip non-GET methods and non-frontend paths.
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		if !strings.HasPrefix(reqPath, "/manager") && reqPath != "/" {
			c.Next()
			return
		}
		// Avoid catching API routes: anything matching the API base path passes
		// through to the next handler.
		if strings.HasPrefix(reqPath, s.cfg.APIBasePath) {
			c.Next()
			return
		}

		// Reaching here means StaticFS found no real embedded file for this GET.
		// Distinguish a genuinely-missing ASSET from an SPA CLIENT ROUTE, because
		// only client routes must fall back to index.html:
		//
		//   asset-like  := path is under /manager/assets/  OR  its last path
		//                  segment has a file extension (e.g. .js/.css/.png/.svg/
		//                  .map/.woff2).  path.Ext returns "" for extension-less
		//                  segments, so a bare client route (e.g. /manager/devices)
		//                  is NOT asset-like.
		//
		// An asset-like path that did not resolve to a real file 404s: serving
		// index.html (text/html) at a hashed .js/.css URL would make the browser
		// parse HTML as a script ("Unexpected token '<'") and mask the real
		// load failure.  A client route keeps the 200 + index.html SPA fallback
		// so the React router can take over.
		if strings.HasPrefix(reqPath, "/manager/assets/") || path.Ext(path.Base(reqPath)) != "" {
			c.Data(http.StatusNotFound, "text/plain; charset=utf-8", []byte("404 asset not found"))
			c.Abort()
			return
		}

		// Serve index.html from the embedded filesystem.  c.FileFromFS does not
		// exist in all Gin versions; we use c.File with the embedded path instead.
		indexData, statErr := managerFS.ReadFile("manager-dist/index.html")
		if statErr != nil {
			c.Next()
			return
		}
		// The SPA-fallback index.html is a DOCUMENT response for a client route
		// (e.g. /manager/devices) served outside the /manager static group, so set
		// the Tier-C document CSP here too.
		c.Header("Content-Security-Policy", spaCSP)
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
		c.Abort()
	})
}
