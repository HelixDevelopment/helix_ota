// Command ota-server is the Helix OTA 1.0.0-MVP control-plane entry point. It
// wires configuration, the persistence repository, and the Gin router, then
// starts the HTTP server.
//
// Transport note: this MVP serves over net/http via Gin. In deployment the
// vasic-digital/http3 wrapper fronts the same net/http.Handler to provide
// HTTP/3 (QUIC) with automatic HTTP/2 fallback, and the `middleware` brick adds
// Brotli/gzip negotiation for control-plane JSON (ADR-0004 / architecture.md
// §7). The artifact-byte download path is served separately, byte-identical and
// ZIP_STORED with Content-Encoding identity + HTTP Range, and is intentionally
// not mounted on this JSON control-plane router. The http3 wrapper is not pulled
// in here yet (per the MVP scope); these comments record the deployment seam.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("ota-server: config: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)

	// MVP wiring: in-memory repository (the pgx/PostgreSQL implementation is the
	// production target — store.Repository is the seam).
	repo := store.NewMemoryRepository()

	// Readiness consults the repository as a stand-in for the real
	// PostgreSQL/MinIO probes used in production.
	checker := health.New(func(ctx context.Context) bool {
		_, getErr := repo.GetIdempotent(ctx, "__readyz__")
		_ = getErr
		return true
	})

	// Admin/operator login directory. Credentials come from the environment so
	// no secret is hard-coded; an unset admin password disables the static user.
	var users []api.StaticUser
	if pw := os.Getenv("HELIX_ADMIN_PASSWORD"); pw != "" {
		users = append(users, api.StaticUser{
			Username: getEnvDefault("HELIX_ADMIN_USERNAME", "admin@helix.example"),
			Password: pw,
			Roles:    []string{api.RoleAdmin, api.RoleOperator, api.RoleViewer},
		})
	}

	srv := api.NewServer(api.Options{
		Config: cfg,
		Repo:   repo,
		Users:  api.NewStaticUserDirectory(users...),
		Health: checker,
	})

	router := srv.Router()

	addr := ":" + cfg.Port
	log.Printf("ota-server: listening on %s (base path %s)", addr, cfg.APIBasePath)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ota-server: serve: %v", err)
	}
}

// getEnvDefault returns the env var or a fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
