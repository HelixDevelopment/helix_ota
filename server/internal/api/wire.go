package api

import (
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// This file holds the REST request/response wire shapes for the MVP endpoints
// (endpoints.md / openapi.yaml). Where the ota-protocol module already provides
// the canonical wire type (UpdateAvailable, PayloadProperties) it is reused
// directly; the remaining shapes are declared here with json tags matching the
// OpenAPI schema field names (snake_case), since the protocol structs use a
// slightly different field set (e.g. Board vs target_model).

// --- auth ---

// LoginRequest is POST /auth/login (LoginRequest schema).
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RefreshRequest is POST /auth/refresh (RefreshRequest schema).
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse is the 200 body for login/refresh (TokenResponse schema).
type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	RefreshToken string   `json:"refresh_token"`
	Roles        []string `json:"roles,omitempty"`
}

// --- devices ---

// DeviceRegistration is POST /devices/register (DeviceRegistration schema).
type DeviceRegistration struct {
	HardwareID     string             `json:"hardware_id"`
	Model          string             `json:"model"`
	OS             otaprotocol.OSType `json:"os"`
	OSVersion      string             `json:"os_version,omitempty"`
	CurrentVersion string             `json:"current_version,omitempty"`
	Group          string             `json:"group,omitempty"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
}

// DeviceRegistered is the 201 body (DeviceRegistered schema).
type DeviceRegistered struct {
	DeviceID     string    `json:"device_id"`
	HardwareID   string    `json:"hardware_id"`
	DeviceToken  string    `json:"device_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RegisteredAt time.Time `json:"registered_at"`
}

// DeviceHealth is the nested health object of DeviceStatus.
type DeviceHealth struct {
	OK            bool    `json:"ok"`
	LastErrorCode *string `json:"last_error_code"`
}

// DeviceStatus is the 200 body for GET /devices/{id}/status (DeviceStatus
// schema).
type DeviceStatus struct {
	DeviceID       string       `json:"device_id"`
	HardwareID     string       `json:"hardware_id"`
	CurrentVersion string       `json:"current_version,omitempty"`
	TargetVersion  *string      `json:"target_version"`
	LastSeen       *time.Time   `json:"last_seen,omitempty"`
	UpdateState    string       `json:"update_state"`
	ActiveSlot     string       `json:"active_slot,omitempty"`
	Health         DeviceHealth `json:"health"`
}

// DeviceListItem is a single device in a paginated list (DeviceListItem schema).
type DeviceListItem struct {
	DeviceID       string             `json:"device_id"`
	HardwareID     string             `json:"hardware_id"`
	Model          string             `json:"model"`
	OS             otaprotocol.OSType `json:"os"`
	OSVersion      string             `json:"os_version,omitempty"`
	CurrentVersion string             `json:"current_version,omitempty"`
	TargetVersion  *string            `json:"target_version"`
	Group          string             `json:"group,omitempty"`
	UpdateState    string             `json:"update_state"`
	ActiveSlot     string             `json:"active_slot,omitempty"`
	HealthOK       bool               `json:"health_ok"`
	LastSeen       *time.Time         `json:"last_seen,omitempty"`
	RegisteredAt   time.Time          `json:"registered_at"`
}

// DeviceList is the paginated list body (DeviceList schema).
type DeviceList struct {
	Items      []DeviceListItem `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

// --- artifacts ---

// ArtifactUploadMetadata is the JSON metadata part of the multipart upload
// (ArtifactUploadMetadata schema).
type ArtifactUploadMetadata struct {
	SHA256        string             `json:"sha256"`
	Signature     string             `json:"signature"`
	Version       string             `json:"version"`
	OS            otaprotocol.OSType `json:"os"`
	TargetModel   string             `json:"target_model"`
	FileHash      string             `json:"file_hash,omitempty"`
	FileSize      int64              `json:"file_size,omitempty"`
	MetadataHash  string             `json:"metadata_hash,omitempty"`
	MetadataSize  int64              `json:"metadata_size,omitempty"`
	PayloadOffset int64              `json:"payload_offset,omitempty"`
	PayloadSize   int64              `json:"payload_size,omitempty"`
}

// Artifact is the 200/201 artifact metadata body (Artifact schema).
type Artifact struct {
	ArtifactID  string             `json:"artifact_id"`
	SHA256      string             `json:"sha256"`
	Size        int64              `json:"size"`
	OS          otaprotocol.OSType `json:"os"`
	TargetModel string             `json:"target_model"`
	Version     string             `json:"version"`
	StorageRef  string             `json:"storage_ref,omitempty"`
	Verified    bool               `json:"verified"`
	UploadedAt  time.Time          `json:"uploaded_at"`
}

// --- releases ---

// ReleaseCreate is POST /releases (ReleaseCreate schema).
type ReleaseCreate struct {
	ArtifactID        string             `json:"artifact_id"`
	Version           string             `json:"version"`
	OS                otaprotocol.OSType `json:"os"`
	TargetModel       string             `json:"target_model"`
	Notes             string             `json:"notes,omitempty"`
	MinCurrentVersion string             `json:"min_current_version,omitempty"`
}

// Release is the release body (Release schema).
type Release struct {
	ReleaseID   string             `json:"release_id"`
	ArtifactID  string             `json:"artifact_id"`
	Version     string             `json:"version"`
	OS          otaprotocol.OSType `json:"os"`
	TargetModel string             `json:"target_model"`
	Status      string             `json:"status"`
	Notes       string             `json:"notes,omitempty"`
	// SRV-1: echo the stored eligibility floor so an operator can confirm the
	// min_current_version they submitted actually stuck (the value enforced in
	// handleClientUpdate). Omitempty keeps legacy releases (no floor) byte-identical.
	MinCurrentVersion string    `json:"min_current_version,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// ReleaseList is the paginated list body (ReleaseList schema).
type ReleaseList struct {
	Items      []Release `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// --- deployments ---

// DeploymentCreate is POST /deployments (DeploymentCreate schema).
type DeploymentCreate struct {
	ReleaseID string `json:"release_id"`
	Strategy  string `json:"strategy"`
	Group     string `json:"group,omitempty"`
}

// Deployment is the deployment body (Deployment schema).
type Deployment struct {
	DeploymentID string    `json:"deployment_id"`
	ReleaseID    string    `json:"release_id"`
	Strategy     string    `json:"strategy"`
	Group        string    `json:"group,omitempty"`
	Status       string    `json:"status"`
	TargetCount  int       `json:"target_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// DeploymentList is the GET /deployments list body (DeploymentList schema): the
// set of active deployments. NextCursor is reserved for future pagination parity
// with ReleaseList (the MVP returns all active deployments in one page).
type DeploymentList struct {
	Items      []Deployment `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

// DeploymentProgress is the aggregate progress block of DeploymentStatus.
type DeploymentProgress struct {
	Pending     int `json:"pending"`
	Downloading int `json:"downloading"`
	Installed   int `json:"installed"`
	Succeeded   int `json:"succeeded"`
	Failed      int `json:"failed"`
}

// DeploymentStatus is GET /deployments/{id} (DeploymentStatus schema).
type DeploymentStatus struct {
	Deployment
	Progress DeploymentProgress `json:"progress"`
}

// --- client (device) ---

// TelemetryEventWire is one event in a TelemetryReport (TelemetryEvent schema).
type TelemetryEventWire struct {
	Event     otaprotocol.TelemetryEvent `json:"event"`
	Version   string                     `json:"version,omitempty"`
	Timestamp time.Time                  `json:"timestamp"`
	ErrorCode *string                    `json:"error_code"`
	Detail    *string                    `json:"detail"`
	// DurationMS / BytesTransferred are the optional per-event annotations the
	// device may report (spec_impl_alignment.md row 4). Pointers + omitempty:
	// absent on legacy device payloads, byte-identical to before this addition.
	DurationMS       *int64 `json:"duration_ms,omitempty"`
	BytesTransferred *int64 `json:"bytes_transferred,omitempty"`
}

// TelemetryHealth is the optional device health block in a TelemetryReport.
type TelemetryHealth struct {
	BatteryPct    *int   `json:"battery_pct,omitempty"`
	StorageFreeMB *int   `json:"storage_free_mb,omitempty"`
	ActiveSlot    string `json:"active_slot,omitempty"`
}

// TelemetryReport is POST /client/telemetry (TelemetryReport schema): a batch.
type TelemetryReport struct {
	DeviceID     string               `json:"device_id"`
	DeploymentID string               `json:"deployment_id,omitempty"`
	Events       []TelemetryEventWire `json:"events"`
	Health       *TelemetryHealth     `json:"health,omitempty"`
}

// TelemetryAck is the 202 body (TelemetryAck schema).
type TelemetryAck struct {
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected"`
	RequestID string `json:"request_id,omitempty"`
}

// --- projects ---

// CreateProjectRequest is POST /projects (admin/operator).
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateProjectRequest is PATCH /projects/{projectId} (admin). Description is a
// pointer so the handler can distinguish "field entirely absent from the JSON
// body" (nil -- leave the stored description untouched) from "field present
// with an explicit empty string" (non-nil pointing to "" -- clear the
// description). A plain string cannot make that distinction: its zero value is
// indistinguishable from an explicitly-empty field, which previously caused
// any partial PATCH that only changed `name` to silently wipe the existing
// description.
type UpdateProjectRequest struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ProjectResponse is a project response body.
type ProjectResponse struct {
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
