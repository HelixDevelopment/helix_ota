package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// handleRegisterDevice provisions a device and mints its device-scoped bearer
// token (endpoints.md §8.1). Honors an optional Idempotency-Key replay.
func (s *Server) handleRegisterDevice(c *gin.Context) {
	var req DeviceRegistration
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed device registration body")
		return
	}
	if req.HardwareID == "" || req.Model == "" || req.OS == "" {
		respondValidation(c, "hardware_id, model, and os are required",
			ErrorDetail{Field: "hardware_id", Issue: "required"},
			ErrorDetail{Field: "model", Issue: "required"},
			ErrorDetail{Field: "os", Issue: "required"})
		return
	}
	if !req.OS.Valid() {
		respondValidation(c, "unsupported os", ErrorDetail{Field: "os", Issue: "must be a known OS type"})
		return
	}

	ctx := c.Request.Context()

	// Idempotent replay: same key returns the original device (200).
	idemKey := c.GetHeader("Idempotency-Key")
	if idemKey != "" {
		if devID, ok := s.repo.GetIdempotent(ctx, idemKey); ok {
			if dev, err := s.repo.GetDevice(ctx, devID); err == nil {
				token, terr := s.mintDeviceToken(dev.DeviceID)
				if terr != nil {
					respondError(c, http.StatusInternalServerError, CodeInternal, "could not mint device token")
					return
				}
				c.JSON(http.StatusOK, toDeviceRegistered(dev, token, int(s.cfg.DeviceTokenTTL.Seconds())))
				return
			}
		}
	}

	dev := store.Device{
		DeviceID:       s.newID(),
		HardwareID:     req.HardwareID,
		Model:          req.Model,
		OSType:         req.OS,
		OSVersion:      req.OSVersion,
		CurrentVersion: req.CurrentVersion,
		Group:          req.Group,
		Metadata:       req.Metadata,
		RegisteredAt:   s.now(),
		UpdateState:    "idle",
		HealthOK:       true,
	}
	if err := s.repo.CreateDevice(ctx, dev); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "hardware_id already registered with a different identity")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not register device")
		return
	}

	token, err := s.mintDeviceToken(dev.DeviceID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not mint device token")
		return
	}

	if idemKey != "" {
		s.repo.PutIdempotent(ctx, idemKey, dev.DeviceID)
	}

	c.JSON(http.StatusCreated, toDeviceRegistered(dev, token, int(s.cfg.DeviceTokenTTL.Seconds())))
}

// handleDeviceStatus returns a device's registry + last-known runtime status
// (endpoints.md §8.2). A device token may read only its own id.
func (s *Server) handleDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")
	claims, _ := claimsFrom(c)

	// Resource ownership: a device-only principal may read only its own id.
	if claims.HasRole(RoleDevice) && !isPrivileged(claims) && claims.Subject != deviceID {
		respondError(c, http.StatusForbidden, CodeForbidden, "a device may read only its own status")
		return
	}

	dev, err := s.repo.GetDevice(c.Request.Context(), deviceID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "device not found")
		return
	}
	c.JSON(http.StatusOK, toDeviceStatus(dev))
}

// mintDeviceToken issues a device-scoped bearer JWT (role=device, sub=deviceId).
func (s *Server) mintDeviceToken(deviceID string) (string, error) {
	return s.signer.Mint(deviceID, []string{RoleDevice}, s.cfg.DeviceTokenTTL, s.now())
}

// isPrivileged reports whether the principal carries a non-device admin role.
func isPrivileged(claims Claims) bool {
	return claims.HasRole(RoleAdmin) || claims.HasRole(RoleOperator) || claims.HasRole(RoleViewer)
}

// toDeviceRegistered maps a stored device to the 201/200 response body.
func toDeviceRegistered(d store.Device, token string, expiresIn int) DeviceRegistered {
	return DeviceRegistered{
		DeviceID:     d.DeviceID,
		HardwareID:   d.HardwareID,
		DeviceToken:  token,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RegisteredAt: d.RegisteredAt,
	}
}

// toDeviceStatus maps a stored device to the DeviceStatus body.
func toDeviceStatus(d store.Device) DeviceStatus {
	st := DeviceStatus{
		DeviceID:       d.DeviceID,
		HardwareID:     d.HardwareID,
		CurrentVersion: d.CurrentVersion,
		UpdateState:    d.UpdateState,
		ActiveSlot:     d.ActiveSlot,
		Health:         DeviceHealth{OK: d.HealthOK},
	}
	if st.UpdateState == "" {
		st.UpdateState = "idle"
	}
	if d.TargetVersion != "" {
		tv := d.TargetVersion
		st.TargetVersion = &tv
	}
	if !d.LastSeen.IsZero() {
		ls := d.LastSeen
		st.LastSeen = &ls
	}
	if d.LastErrorCode != "" {
		ec := d.LastErrorCode
		st.Health.LastErrorCode = &ec
	}
	return st
}
