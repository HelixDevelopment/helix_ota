package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// handleHealthz is the liveness probe (endpoints.md / architecture.md). It is
// unauthenticated and returns 200 while the process is up. When the server is
// degraded (started with in-memory fallback after a PostgreSQL connection
// failure), the response includes isDegraded: true so operators can detect the
// fallback state.
func (s *Server) handleHealthz(c *gin.Context) {
	if s.health.Live() {
		if s.Degraded {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "isDegraded": true})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
}

// handleReadyz is the readiness probe. It returns 200 when dependencies are
// reachable, else 503 so an orchestrator withholds traffic.
func (s *Server) handleReadyz(c *gin.Context) {
	if s.health.Ready(c.Request.Context()) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
}

// storeReadyProbeTimeout bounds the readiness round-trip so a slow or hung store
// never blocks the probe — the orchestrator gets a prompt 503 rather than a
// stalled request.
const storeReadyProbeTimeout = 2 * time.Second

// StoreReadinessChecker is the minimal persistence surface the readiness probe
// exercises: one cheap, bounded round-trip that returns a non-nil error when the
// store cannot serve. store.Repository satisfies it.
type StoreReadinessChecker interface {
	ListProjects(ctx context.Context) ([]store.Project, error)
}

// NewStoreReadinessProbe builds the readiness probe wired into /readyz. It reports
// ready only when a cheap, bounded round-trip against the store succeeds
// (ListProjects over the small projects table). Any error — store unreachable,
// query failure, or the bounded deadline elapsing — reports NOT ready, so /readyz
// returns 503 and an orchestrator withholds traffic. This replaces the earlier
// probe that reported ready unconditionally regardless of real store health
// (SRV-NEW-2 / OTA-063).
func NewStoreReadinessProbe(repo StoreReadinessChecker) health.ReadyFunc {
	return func(ctx context.Context) bool {
		ctx, cancel := context.WithTimeout(ctx, storeReadyProbeTimeout)
		defer cancel()
		_, err := repo.ListProjects(ctx)
		return err == nil
	}
}
