// Package store defines the persistence seam for the Helix OTA control plane: a
// Repository interface covering devices, artifacts, releases, deployments, and
// telemetry, plus an in-memory implementation. Per architecture.md §4 the
// production target is a pgx-backed PostgreSQL implementation; the in-memory
// implementation here keeps the api and validation seams testable without a
// database (and is wired by default in the MVP skeleton).
//
// No transport types cross this seam (architecture.md §6): the repository takes
// and returns plain domain structs only.
package store

import (
	"context"
	"errors"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// Sentinel errors returned by Repository implementations so callers can branch
// with errors.Is and map to HTTP status codes in the api layer.
var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrConflict indicates a uniqueness/state conflict (e.g. a hardware_id
	// already registered with a different identity, or an overlapping active
	// deployment).
	ErrConflict = errors.New("store: conflict")
)

// Device is the registry record for a provisioned device.
type Device struct {
	DeviceID       string
	HardwareID     string
	Model          string
	OSType         otaprotocol.OSType
	OSVersion      string
	CurrentVersion string
	Group          string
	Metadata       map[string]string
	RegisteredAt   time.Time

	// Runtime status (last-known), updated from telemetry.
	LastSeen      time.Time
	UpdateState   string
	ActiveSlot    string
	LastErrorCode string
	HealthOK      bool
	// TargetVersion is the version a deployment currently assigns to this device
	// (empty when none applies).
	TargetVersion string
}

// Artifact is a stored-and-verified OTA artifact record.
type Artifact struct {
	ArtifactID  string
	SHA256      string
	Size        int64
	OSType      otaprotocol.OSType
	TargetModel string
	Version     string
	StorageRef  string
	Verified    bool
	UploadedAt  time.Time

	// Signature is the base64 detached signature carried through to the device
	// update-check contract (endpoints.md §12.1).
	Signature string
	// PayloadProperties carries the four AOSP applyPayload headers.
	PayloadProperties otaprotocol.PayloadProperties
}

// Release binds a validated artifact to a published, deployable version.
type Release struct {
	ReleaseID         string
	ArtifactID        string
	Version           string
	OSType            otaprotocol.OSType
	TargetModel       string
	Status            string
	Notes             string
	MinCurrentVersion string
	CreatedAt         time.Time
}

// Deployment assigns a release to a target set (all-targets for MVP).
type Deployment struct {
	DeploymentID string
	ReleaseID    string
	Strategy     string
	Group        string
	Status       string
	TargetCount  int
	CreatedAt    time.Time
}

// TelemetryRecord is one persisted device lifecycle event (telemetry_events).
type TelemetryRecord struct {
	DeviceID     string
	DeploymentID string
	Event        otaprotocol.TelemetryEvent
	Version      string
	ErrorCode    string
	Detail       string
	Timestamp    time.Time
	ReceivedAt   time.Time
}

// AuditEntry is one persisted admin/operator action (audit_logs;
// operational_endpoints.md §4.2). No transport types cross the seam. UserID is
// empty when the actor does not resolve to a users row (nullable by design);
// the token subject is then preserved in ActorSubject.
type AuditEntry struct {
	ID           string
	UserID       string
	ActorSubject string
	Action       string // SCREAMING_SNAKE_CASE verb, e.g. DEVICE_REGISTER
	ResourceType string // artifact|release|deployment|device|group|group_member
	ResourceID   string
	Details      map[string]string
	IPAddress    string
	UserAgent    string
	CreatedAt    time.Time
}

// DeltaArtifact links a base artifact + a target artifact to a generated delta
// payload (migration 004 delta_artifacts; delta_updates_design.md §4). A device
// on the base can fetch the small delta instead of the full target payload.
type DeltaArtifact struct {
	ID               string
	BaseArtifactID   string
	TargetArtifactID string
	SHA256           string
	Size             int64
	StorageRef       string
	CreatedAt        time.Time
}

// RollbackRecord is one append-only rollback/abort audit row (migration 002
// rollback_history; rollback_ux.md). Kind is "abort" (halt forward progress) or
// "rollback" (server-driven recall to a previous release).
type RollbackRecord struct {
	ID                 string
	DeploymentID       string
	Kind               string // abort | rollback
	FromReleaseID      string
	ToReleaseID        string
	RecallDeploymentID string
	Reason             string
	TriggeredBy        string
	Details            map[string]string
	CreatedAt          time.Time
}

// Group is a named device cohort (device_groups; operational_endpoints.md §6).
type Group struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
}

// AuditFilter narrows an audit list query (operational_endpoints.md §4.3).
// Since/Until are inclusive time bounds (zero value = unbounded).
type AuditFilter struct {
	Action       string
	ResourceType string
	Since        time.Time
	Until        time.Time
	Limit        int
	Cursor       string
}

// ReleaseFilter narrows a release list query (endpoints.md §10.2).
type ReleaseFilter struct {
	OSType      otaprotocol.OSType
	TargetModel string
	Status      string
	Limit       int
	Cursor      string
}

// Repository is the persistence port for the control plane. Implementations are
// the in-memory MemoryRepository (MVP/testing) and a future pgx/PostgreSQL one.
type Repository interface {
	// Devices.
	CreateDevice(ctx context.Context, d Device) error
	GetDevice(ctx context.Context, deviceID string) (Device, error)
	GetDeviceByHardwareID(ctx context.Context, hardwareID string) (Device, error)
	UpdateDevice(ctx context.Context, d Device) error

	// Artifacts.
	CreateArtifact(ctx context.Context, a Artifact) error
	GetArtifact(ctx context.Context, artifactID string) (Artifact, error)

	// Releases.
	CreateRelease(ctx context.Context, r Release) error
	GetRelease(ctx context.Context, releaseID string) (Release, error)
	LatestRelease(ctx context.Context, os otaprotocol.OSType, targetModel string) (Release, error)
	ListReleases(ctx context.Context, f ReleaseFilter) ([]Release, string, error)

	// Deployments.
	CreateDeployment(ctx context.Context, d Deployment) error
	GetDeployment(ctx context.Context, deploymentID string) (Deployment, error)
	UpdateDeployment(ctx context.Context, d Deployment) error
	ActiveDeploymentForTarget(ctx context.Context, os otaprotocol.OSType, targetModel, group string) (Deployment, error)
	ListActiveDeployments(ctx context.Context) ([]Deployment, error)

	// Telemetry.
	AppendTelemetry(ctx context.Context, rec TelemetryRecord) error
	TelemetryForDeployment(ctx context.Context, deploymentID string) ([]TelemetryRecord, error)
	// TelemetryForDevice returns a device's event history in insertion order
	// (operational_endpoints.md §5).
	TelemetryForDevice(ctx context.Context, deviceID string) ([]TelemetryRecord, error)
	// TelemetryEventCounts returns fleet-wide counts keyed by event type, for the
	// /telemetry/overview aggregate (operational_endpoints.md §5).
	TelemetryEventCounts(ctx context.Context) (map[string]int64, error)
	// DeviceStateCounts returns fleet device counts keyed by last-known update
	// state (operational_endpoints.md §5 by_state).
	DeviceStateCounts(ctx context.Context) (map[string]int64, error)

	// Audit (operational_endpoints.md §4): append-only admin/operator action log.
	AppendAudit(ctx context.Context, e AuditEntry) error
	ListAudit(ctx context.Context, f AuditFilter) ([]AuditEntry, string, error)

	// Rollback history (rollback_ux.md): append-only abort/recall audit.
	AppendRollback(ctx context.Context, r RollbackRecord) error
	ListRollbacks(ctx context.Context, deploymentID string) ([]RollbackRecord, error)

	// Delta artifacts (delta_updates_design.md §4): base->target delta lookup.
	CreateDelta(ctx context.Context, d DeltaArtifact) error
	FindDelta(ctx context.Context, baseArtifactID, targetArtifactID string) (DeltaArtifact, error)

	// Device groups (operational_endpoints.md §6). A duplicate group name is a
	// conflict; membership add/remove is idempotent and requires the group to
	// exist. Members are device ids.
	CreateGroup(ctx context.Context, g Group) error
	GetGroup(ctx context.Context, groupID string) (Group, error)
	ListGroups(ctx context.Context) ([]Group, error)
	UpdateGroup(ctx context.Context, g Group) error
	DeleteGroup(ctx context.Context, groupID string) error
	AddGroupMember(ctx context.Context, groupID, deviceID string) error
	ListGroupMembers(ctx context.Context, groupID string) ([]string, error)
	RemoveGroupMember(ctx context.Context, groupID, deviceID string) error

	// Idempotency support for register/deployment replay (endpoints.md §2).
	GetIdempotent(ctx context.Context, key string) (string, bool)
	PutIdempotent(ctx context.Context, key, resultID string)
}
