package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/fabric"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// FabricNodeRequest is POST /fabric/nodes body.
type FabricNodeRequest struct {
	NodeID string            `json:"node_id"`
	Kind   string            `json:"kind"`
	Arch   string            `json:"arch"`
	HasKVM bool              `json:"has_kvm,omitempty"`
	HasHVF bool              `json:"has_hvf,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// FabricNodeResponse is a fabric node response.
type FabricNodeResponse struct {
	NodeID     string            `json:"node_id"`
	Kind       string            `json:"kind"`
	Arch       string            `json:"arch"`
	HasKVM     bool              `json:"has_kvm"`
	HasHVF     bool              `json:"has_hvf"`
	Labels     map[string]string `json:"labels,omitempty"`
	LastSeenAt string            `json:"last_seen_at,omitempty"`
	CreatedAt  string            `json:"created_at"`
}

// FabricTargetRequest is POST /fabric/targets body.
type FabricTargetRequest struct {
	TargetID  string `json:"target_id"`
	Tier      string `json:"tier"`
	Tech      string `json:"tech"`
	Model     string `json:"model,omitempty"`
	OSType    string `json:"os_type,omitempty"`
	Exclusive bool   `json:"exclusive,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
}

// FabricTargetResponse is a fabric target response.
type FabricTargetResponse struct {
	TargetID  string `json:"target_id"`
	Tier      string `json:"tier"`
	Tech      string `json:"tech"`
	Model     string `json:"model,omitempty"`
	OSType    string `json:"os_type"`
	Exclusive bool   `json:"exclusive"`
	NodeID    string `json:"node_id,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// handleRegisterFabricNode registers a CI/QA build-farm node in the fabric
// registry (fabric.md). A fabric node is a physical or virtual build host
// with known architecture, virtualisation capabilities (KVM/HVF), and
// allocation labels. Nodes are the capacity layer beneath targets.
func (s *Server) handleRegisterFabricNode(c *gin.Context) {
	var req FabricNodeRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed fabric node body")
		return
	}
	if req.NodeID == "" || req.Kind == "" || req.Arch == "" {
		respondValidation(c, "node_id, kind, and arch are required",
			ErrorDetail{Field: "node_id", Issue: "required"},
			ErrorDetail{Field: "kind", Issue: "required"},
			ErrorDetail{Field: "arch", Issue: "required"})
		return
	}
	reg := fabric.New(s.repo, nil)
	if err := reg.RegisterNode(c.Request.Context(), store.FabricNode{
		NodeID: req.NodeID, Kind: req.Kind, Arch: req.Arch,
		HasKVM: req.HasKVM, HasHVF: req.HasHVF, Labels: req.Labels,
	}); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not register fabric node")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"node_id": req.NodeID, "registered": true})
}

// handleGetFabricNode returns a single fabric node by its node_id.
func (s *Server) handleGetFabricNode(c *gin.Context) {
	nodeID := c.Param("nodeId")
	n, err := s.repo.GetFabricNode(c.Request.Context(), nodeID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "fabric node not found")
		return
	}
	c.JSON(http.StatusOK, FabricNodeResponse{
		NodeID: n.NodeID, Kind: n.Kind, Arch: n.Arch,
		HasKVM: n.HasKVM, HasHVF: n.HasHVF, Labels: n.Labels,
		LastSeenAt: n.LastSeenAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:  n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// handleRegisterFabricTarget registers a test/QA target in the fabric
// registry (fabric.md). A target is a testable device or emulator instance
// classified by tier (1/2/3), technology (qemu/cuttlefish/avd/physical),
// model, OS type, and optional exclusive-access flag. Each target is
// optionally assigned to a fabric node for execution scheduling.
func (s *Server) handleRegisterFabricTarget(c *gin.Context) {
	var req FabricTargetRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed fabric target body")
		return
	}
	if req.TargetID == "" || req.Tier == "" || req.Tech == "" {
		respondValidation(c, "target_id, tier, and tech are required",
			ErrorDetail{Field: "target_id", Issue: "required"},
			ErrorDetail{Field: "tier", Issue: "required"},
			ErrorDetail{Field: "tech", Issue: "required"})
		return
	}
	reg := fabric.New(s.repo, nil)
	if err := reg.RegisterTarget(c.Request.Context(), store.FabricTarget{
		TargetID: req.TargetID, Tier: req.Tier, Tech: req.Tech,
		Model: req.Model, OSType: req.OSType, Exclusive: req.Exclusive, NodeID: req.NodeID,
	}); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not register fabric target")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"target_id": req.TargetID, "registered": true})
}

// handleListFabricTargets returns all registered fabric targets, used by the
// test scheduler and dashboard fleet view.
func (s *Server) handleListFabricTargets(c *gin.Context) {
	targets, err := s.repo.ListFabricTargets(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list fabric targets")
		return
	}
	out := make([]FabricTargetResponse, 0, len(targets))
	for _, t := range targets {
		out = append(out, FabricTargetResponse{
			TargetID: t.TargetID, Tier: t.Tier, Tech: t.Tech,
			Model: t.Model, OSType: t.OSType, Exclusive: t.Exclusive,
			NodeID: t.NodeID, Status: t.Status,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
