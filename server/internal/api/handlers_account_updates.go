package api

import (
	"net/http"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// AccountUpdateEntry is one available update for an account fleet target.
type AccountUpdateEntry struct {
	OSType      otaprotocol.OSType `json:"os_type"`
	TargetModel string             `json:"target_model"`
	Version     string             `json:"version"`
	ReleaseID   string             `json:"release_id"`
}

func (s *Server) handleListAccountUpdates(c *gin.Context) {
	accountID := c.Param("accountId")
	if accountID == "" {
		respondValidation(c, "account ID is required")
		return
	}

	ctx := c.Request.Context()
	devices, err := s.repo.ListDevicesForAccount(ctx, accountID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list account devices")
		return
	}

	type targetKey struct {
		os    otaprotocol.OSType
		model string
	}
	seen := map[targetKey]bool{}
	var updates []AccountUpdateEntry
	for _, dev := range devices {
		tk := targetKey{os: dev.OSType, model: dev.Model}
		if seen[tk] {
			continue
		}
		seen[tk] = true
		rel, err := s.repo.LatestRelease(ctx, dev.OSType, dev.Model)
		if err != nil {
			continue
		}
		updates = append(updates, AccountUpdateEntry{
			OSType:      dev.OSType,
			TargetModel: dev.Model,
			Version:     rel.Version,
			ReleaseID:   rel.ReleaseID,
		})
	}
	c.JSON(http.StatusOK, updates)
}

// DeviceRegistrationRequest is POST /accounts/:accountId/devices body.
type DeviceRegistrationRequest struct {
	HardwareID string              `json:"hardware_id"`
	Model      string              `json:"model"`
	OSType     otaprotocol.OSType  `json:"os_type"`
	OSVersion  string              `json:"os_version"`
	Group      string              `json:"group,omitempty"`
	Metadata   map[string]string   `json:"metadata,omitempty"`
}

// DeviceRegistrationResponse is the response for a registered device.
type DeviceRegistrationResponse struct {
	DeviceID     string    `json:"device_id"`
	HardwareID   string    `json:"hardware_id"`
	AccountID    string    `json:"account_id"`
	Model        string    `json:"model"`
	OSType       otaprotocol.OSType `json:"os_type"`
	OSVersion    string    `json:"os_version"`
	Group        string    `json:"group,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

func deviceRegistrationResponse(d store.Device) DeviceRegistrationResponse {
	return DeviceRegistrationResponse{
		DeviceID:     d.DeviceID,
		HardwareID:   d.HardwareID,
		AccountID:    d.AccountID,
		Model:        d.Model,
		OSType:       d.OSType,
		OSVersion:    d.OSVersion,
		Group:        d.Group,
		RegisteredAt: d.RegisteredAt,
	}
}

func (s *Server) handleRegisterDeviceForAccount(c *gin.Context) {
	accountID := c.Param("accountId")
	if accountID == "" {
		respondValidation(c, "account ID is required")
		return
	}

	var req DeviceRegistrationRequest
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed device registration body")
		return
	}
	if req.HardwareID == "" || req.Model == "" || req.OSType == "" {
		respondValidation(c, "hardware_id, model, and os_type are required",
			ErrorDetail{Field: "hardware_id", Issue: "required"},
			ErrorDetail{Field: "model", Issue: "required"},
			ErrorDetail{Field: "os_type", Issue: "required"})
		return
	}

	now := s.now()
	dev := store.Device{
		DeviceID:     s.newID(),
		HardwareID:   req.HardwareID,
		Model:        req.Model,
		OSType:       req.OSType,
		OSVersion:    req.OSVersion,
		Group:        req.Group,
		Metadata:     req.Metadata,
		RegisteredAt: now,
	}

	ctx := c.Request.Context()
	registered, err := s.repo.RegisterDeviceForAccount(ctx, accountID, dev)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not register device")
		return
	}

	status := http.StatusCreated
	if registered.DeviceID != dev.DeviceID {
		status = http.StatusOK
	}
	c.JSON(status, deviceRegistrationResponse(registered))
}
