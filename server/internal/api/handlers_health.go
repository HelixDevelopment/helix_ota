package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleHealthz is the liveness probe (endpoints.md / architecture.md). It is
// unauthenticated and returns 200 while the process is up.
func (s *Server) handleHealthz(c *gin.Context) {
	if s.health.Live() {
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
