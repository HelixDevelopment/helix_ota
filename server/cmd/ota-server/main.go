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
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("ota-server: config: %v", err)
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
				log.Fatalf("ota-server: connect postgres (after 60s of retries): %v", perr)
			}
			log.Printf("ota-server: postgres not ready yet: %v — retrying in 2s", perr)
			time.Sleep(2 * time.Second)
		}
		if perr := pg.Migrate(bootCtx); perr != nil {
			log.Fatalf("ota-server: migrate store schema: %v", perr)
		}
		rs, rerr := rollout.NewPostgresStore(bootCtx, cfg.DatabaseURL)
		if rerr != nil {
			log.Fatalf("ota-server: connect rollout store: %v", rerr)
		}
		if rerr := rs.Migrate(bootCtx); rerr != nil {
			log.Fatalf("ota-server: migrate rollout schema: %v", rerr)
		}
		repo = pg
		rolloutSvc = rollout.NewServiceWithStore(rs, time.Now)
		log.Printf("ota-server: persistence = PostgreSQL (pgx)")
	} else {
		repo = store.NewMemoryRepository()
		log.Printf("ota-server: persistence = in-memory (set HELIX_DATABASE_URL for PostgreSQL)")
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
		Users:   api.MustNewStaticUserDirectory(users...),
		Health:  checker,
	})

	router := srv.Router()

	// OTA-032: drain middleware returns 503 while the server is gracefully
	// stopping so an orchestrator/load-balancer can retry another instance.
	var draining atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "server draining — please retry", http.StatusServiceUnavailable)
			return
		}
		router.ServeHTTP(w, r)
	})

	const drainTimeout = 30 * time.Second

	// When TLS material is configured, serve the control plane over HTTP/3
	// (QUIC) with automatic HTTP/2 fallback via the transport package
	// (ADR-0004). Otherwise serve plain HTTP for local development.
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, certErr := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if certErr != nil {
			log.Fatalf("ota-server: load TLS keypair: %v", certErr)
		}
		addr := ":" + cfg.HTTPSPort
		tsrv, tErr := transport.New(transport.Config{
			Addr:    addr,
			Handler: handler,
			TLSConf: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13},
		})
		if tErr != nil {
			log.Fatalf("ota-server: transport: %v", tErr)
		}
		log.Printf("ota-server: serving HTTP/3 (QUIC) + HTTP/2 on %s (base path %s)", addr, cfg.APIBasePath)

		startErr := make(chan error, 1)
		go func() { startErr <- tsrv.Start() }()

		graceful(tsrv, &draining, drainTimeout, startErr)
		return
	}

	addr := ":" + cfg.Port
	log.Printf("ota-server: listening on %s (plain HTTP, base path %s)", addr, cfg.APIBasePath)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	startErr := make(chan error, 1)
	go func() {
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		startErr <- err
	}()

	graceful(httpServer, &draining, drainTimeout, startErr)
}

// getEnvDefault returns the env var or a fallback.
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// serverStopper is the minimal interface both *http.Server and
// *transport.Server satisfy (each has Shutdown(context.Context) error).
type serverStopper interface {
	Shutdown(context.Context) error
}

// graceful blocks until SIGTERM or SIGINT, then drains the server with a
// bounded timeout. During the drain new requests receive 503 (via the draining
// middleware) while in-flight requests complete.
func graceful(s serverStopper, draining *atomic.Bool, timeout time.Duration, startErr <-chan error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		log.Printf("ota-server: received %v, draining (%v timeout)…", sig, timeout)
	case err := <-startErr:
		if err != nil {
			log.Fatalf("ota-server: serve: %v", err)
		}
		return
	}

	draining.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("ota-server: shutdown: %v", err)
	}
	log.Println("ota-server: stopped")
}
