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

	// Serve the SPA at /manager with a directory index.
	r.StaticFS("/manager", http.FS(sub))

	// SPA fallback: any GET request under /manager that does not match a file
	// serves the SPA's index.html so client-side routing works.
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Only handle GET requests under /manager (or at root when the SPA is the
		// only frontend).  Skip non-GET methods and non-frontend paths.
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		if !strings.HasPrefix(path, "/manager") && path != "/" {
			c.Next()
			return
		}
		// Avoid catching API routes: anything matching the API base path passes
		// through to the next handler.
		if strings.HasPrefix(path, s.cfg.APIBasePath) {
			c.Next()
			return
		}

		// Serve index.html from the embedded filesystem.  c.FileFromFS does not
		// exist in all Gin versions; we use c.File with the embedded path instead.
		indexData, statErr := managerFS.ReadFile("manager-dist/index.html")
		if statErr != nil {
			c.Next()
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
		c.Abort()
	})
}
