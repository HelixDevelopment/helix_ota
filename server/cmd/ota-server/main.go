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
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/rollout"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
	"github.com/HelixDevelopment/helix_ota/server/internal/transport"
)

func main() {
	// OTA-034: structured JSON logging (slog).
	slog.SetDefault(api.NewJSONLogger(slog.LevelInfo))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("ota-server: config", "err", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)

	// Persistence: pgx/PostgreSQL when HELIX_DATABASE_URL is set (production
	// target, architecture.md §4), else the in-memory implementations (dev/MVP).
	// store.Repository + the rollout StoragePort are the seams.
	var repo store.Repository
	var rolloutSvc *rollout.Service
	if cfg.DatabaseURL != "" {
		bootCtx := context.Background()
		// Startup connection retry (robustness): a freshly started PostgreSQL
		// reports its container "up" before it accepts connections, so the first
		// ping can hit "connection refused" on a boot-ordering race (compose /
		// k8s / systemd). Retry with bounded backoff (up to 60s) instead of
		// crashing the control plane; only a persistent failure is fatal.
		var pg *store.PostgresRepository
		var perr error
		deadline := time.Now().Add(60 * time.Second)
		for {
			if pg, perr = store.NewPostgresRepository(bootCtx, cfg.DatabaseURL); perr == nil {
				break
			}
			if time.Now().After(deadline) {
				slog.Error("ota-server: connect postgres after 60s of retries", "err", perr)
				os.Exit(1)
			}
			slog.Info("ota-server: postgres not ready yet, retrying in 2s", "err", perr)
			time.Sleep(2 * time.Second)
		}
		if perr := pg.Migrate(bootCtx); perr != nil {
			slog.Error("ota-server: migrate store schema", "err", perr)
			os.Exit(1)
		}
		rs, rerr := rollout.NewPostgresStore(bootCtx, cfg.DatabaseURL)
		if rerr != nil {
			slog.Error("ota-server: connect rollout store", "err", rerr)
			os.Exit(1)
		}
		if rerr := rs.Migrate(bootCtx); rerr != nil {
			slog.Error("ota-server: migrate rollout schema", "err", rerr)
			os.Exit(1)
		}
		repo = pg
		rolloutSvc = rollout.NewServiceWithStore(rs, time.Now)
		slog.Info("ota-server: persistence = PostgreSQL (pgx)")
	} else {
		repo = store.NewMemoryRepository()
		slog.Info("ota-server: persistence = in-memory (set HELIX_DATABASE_URL for PostgreSQL)")
	}

	// Readiness reflects real store health: /readyz reports ready only when a
	// cheap, bounded round-trip against the persistence store succeeds, and 503
	// otherwise so an orchestrator withholds traffic (SRV-NEW-2 / OTA-063). The
	// prior probe returned ready unconditionally, masking an unreachable store.
	checker := health.New(api.NewStoreReadinessProbe(repo))

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
		Config:  cfg,
		Repo:    repo,
		Rollout: rolloutSvc, // nil with the in-memory default => NewServer builds a memory rollout service
		Users:   api.NewStaticUserDirectory(users...),
		Health:  checker,
		Metrics: api.NewMetrics(nil), // OTA-034: default prometheus registry
	})

	router := srv.Router()

	// When TLS material is configured, serve the control plane over HTTP/3
	// (QUIC) with automatic HTTP/2 fallback via the transport package
	// (ADR-0004). Otherwise serve plain HTTP for local development.
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, certErr := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if certErr != nil {
			slog.Error("ota-server: load TLS keypair", "err", certErr)
			os.Exit(1)
		}
		addr := ":" + cfg.HTTPSPort
		tsrv, tErr := transport.New(transport.Config{
			Addr:    addr,
			Handler: router,
			TLSConf: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13},
		})
		if tErr != nil {
			slog.Error("ota-server: transport", "err", tErr)
			os.Exit(1)
		}
		slog.Info("ota-server: serving HTTP/3 (QUIC) + HTTP/2", "addr", addr, "base_path", cfg.APIBasePath)
		if err := tsrv.Start(); err != nil {
			slog.Error("ota-server: serve", "err", err)
			os.Exit(1)
		}
		return
	}

	addr := ":" + cfg.Port
	slog.Info("ota-server: listening (plain HTTP)", "addr", addr, "base_path", cfg.APIBasePath)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("ota-server: serve", "err", err)
			os.Exit(1)
	}
}

// getEnvDefault returns the env var or a fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
