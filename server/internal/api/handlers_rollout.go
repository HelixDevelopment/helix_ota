package api

import (
	"errors"
	"net/http"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
	"github.com/gin-gonic/gin"
)

// --- wire types (1.0.1-staged-rollout/rollout_engine.md §8) ---

// RolloutPhaseSpec is one phase in a create-rollout request.
type RolloutPhaseSpec struct {
	Percentage       int     `json:"percentage"`
	SuccessThreshold float64 `json:"success_threshold"`
	ErrorThreshold   float64 `json:"error_threshold"`
	DurationSeconds  int64   `json:"duration_seconds"`
	AutoProgress     bool    `json:"auto_progress"`
}

// RolloutCreate is the POST /deployments/{id}/rollout body.
type RolloutCreate struct {
	Phases []RolloutPhaseSpec `json:"phases"`
}

// RolloutVerdict is the POST /deployments/{id}/rollout/evaluate body — the
// telemetry-derived health summary for the current phase cohort.
type RolloutVerdict struct {
	SuccessRate          float64 `json:"success_rate"`
	ErrorRate            float64 `json:"error_rate"`
	PostBootHealthFailed bool    `json:"post_boot_health_failed"`
}

// RolloutState is a rollout-state response.
type RolloutState struct {
	DeploymentID string             `json:"deployment_id"`
	Status       string             `json:"status"`
	CurrentPhase int                `json:"current_phase"`
	Phases       []RolloutPhaseSpec `json:"phases"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// RolloutDecision is the response to an evaluation.
type RolloutDecision struct {
	Action string       `json:"action"`
	Reason string       `json:"reason"`
	State  RolloutState `json:"state"`
}

func toRolloutPhaseSpecs(phases []engine.Phase) []RolloutPhaseSpec {
	out := make([]RolloutPhaseSpec, 0, len(phases))
	for _, p := range phases {
		out = append(out, RolloutPhaseSpec{
			Percentage:       p.Percentage,
			SuccessThreshold: p.SuccessThreshold,
			ErrorThreshold:   p.ErrorThreshold,
			DurationSeconds:  int64(p.Duration / time.Second),
			AutoProgress:     p.AutoProgress,
		})
	}
	return out
}

func toRolloutState(st engine.State) RolloutState {
	return RolloutState{
		DeploymentID: st.DeploymentID,
		Status:       string(st.Status),
		CurrentPhase: st.CurrentPhase,
		Phases:       toRolloutPhaseSpecs(st.Phases),
		UpdatedAt:    st.UpdatedAt,
	}
}

// --- handlers ---

// handleCreateRollout creates + starts a staged rollout for a deployment. The
// brick validates the phase plan (strictly-increasing percentages ending at
// 100, thresholds in [0,1]); a plan violation maps to 400.
func (s *Server) handleCreateRollout(c *gin.Context) {
	deploymentID := c.Param("deploymentId")
	if _, err := s.repo.GetDeployment(c.Request.Context(), deploymentID); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "deployment not found")
		return
	}
	var req RolloutCreate
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed rollout body")
		return
	}
	if len(req.Phases) == 0 {
		respondValidation(c, "phases must contain at least one phase",
			ErrorDetail{Field: "phases", Issue: "must not be empty"})
		return
	}
	phases := make([]engine.Phase, 0, len(req.Phases))
	for _, p := range req.Phases {
		phases = append(phases, engine.Phase{
			Percentage:       p.Percentage,
			SuccessThreshold: p.SuccessThreshold,
			ErrorThreshold:   p.ErrorThreshold,
			Duration:         time.Duration(p.DurationSeconds) * time.Second,
			AutoProgress:     p.AutoProgress,
		})
	}
	st, err := s.rollout.CreateAndStart(c.Request.Context(), deploymentID, phases)
	if err != nil {
		// Plan-validation failures from the brick are client errors.
		respondValidation(c, "invalid rollout plan", ErrorDetail{Field: "phases", Issue: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toRolloutState(st))
}

// handleGetRollout returns the current rollout state for a deployment.
func (s *Server) handleGetRollout(c *gin.Context) {
	st, err := s.rollout.Get(c.Request.Context(), c.Param("deploymentId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "no rollout for this deployment")
		return
	}
	c.JSON(http.StatusOK, toRolloutState(st))
}

// handleEvaluateRollout applies a health verdict to the current phase and
// returns the engine decision (advance / hold / halt / complete).
func (s *Server) handleEvaluateRollout(c *gin.Context) {
	deploymentID := c.Param("deploymentId")
	var req RolloutVerdict
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed verdict body")
		return
	}
	dec, err := s.rollout.Evaluate(c.Request.Context(), deploymentID, engine.HealthVerdict{
		SuccessRate:          req.SuccessRate,
		ErrorRate:            req.ErrorRate,
		PostBootHealthFailed: req.PostBootHealthFailed,
	})
	if err != nil {
		if errors.Is(err, engine.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "no rollout for this deployment")
			return
		}
		respondValidation(c, "could not evaluate rollout", ErrorDetail{Issue: err.Error()})
		return
	}
	st, _ := s.rollout.Get(c.Request.Context(), deploymentID)
	c.JSON(http.StatusOK, RolloutDecision{
		Action: string(dec.Action),
		Reason: string(dec.Reason),
		State:  toRolloutState(st),
	})
}
