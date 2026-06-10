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
	// ErrEvidenceEmpty indicates a fabric evidence artefact with a non-positive
	// byte size was rejected (§11.4.69 — a 0-byte artefact can never satisfy a
	// PASS). The pgx layer maps the CHECK violation to this same sentinel.
	ErrEvidenceEmpty = errors.New("store: evidence artefact is empty")
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
	// DurationMS / BytesTransferred are the optional per-event telemetry
	// annotations from the spec (spec_impl_alignment.md row 4), nil when the
	// device did not report them. Persisted nullable so a legacy event that omits
	// them round-trips as nil (never as a misleading 0).
	DurationMS       *int64
	BytesTransferred *int64
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

// GroupMember is one device-group membership with its join time
// (device_group_members; operational_endpoints.md §6).
type GroupMember struct {
	DeviceID string
	AddedAt  time.Time
}

// --- emulation test-fabric registry (docs/design/emulation_fabric/SCHEMA.sql) ---
//
// DESIGN REFINEMENT (§11.4.28): the fabric REGISTRY persistence lives in
// helix_ota (this store seam), NOT in the reusable containers submodule's
// pkg/fabric. The schema's tier/tech/target vocabulary (T0/T1/T2/T3/Tfw/Tcp,
// rk3588, OrangePi5Max, the ota-protocol PASS-evidence rule) is PROJECT-SPECIFIC
// to Helix OTA, so per §11.4.28(B) decoupling it must not be injected into the
// project-agnostic containers brick. The generic scheduler/lease façade may still
// live upstream in containers/pkg/fabric; the OTA-shaped registry tables are
// modelled here, on the existing pgx/memory Repository seam.

// FabricNode is a distributable execution node in the test fabric
// (helix_ota.fabric_nodes; SCHEMA.sql). Capability flags gate which tiers it can
// run (HVF vs KVM, arch).
type FabricNode struct {
	NodeID     string
	Kind       string // 'dev-mac' | 'ci-linux-kvm' | 'lava-worker' | 'hil-host'
	Arch       string // 'arm64' | 'x86_64'
	HasKVM     bool
	HasHVF     bool
	Labels     map[string]string
	LastSeenAt time.Time
	CreatedAt  time.Time
}

// FabricTarget is an emulated/virtual/real target the fabric can run a job
// against (helix_ota.fabric_targets). Tier mirrors DESIGN.md §2. An exclusive
// target MUST be leased to exactly one run at a time (§11.4.119).
type FabricTarget struct {
	TargetID  string
	Tier      string // 'T0'|'T1'|'T2'|'T3'|'Tfw'|'Tcp'
	Tech      string // 'ota-device-emulator'|'avd-arm64'|'cuttlefish'|'rk3588'|...
	Model     string // e.g. 'OrangePi5Max'
	OSType    string // 'android' by default
	Exclusive bool
	NodeID    string // optional binding to a fabric node ("" == unbound)
	Status    string // 'idle'|'leased'|'offline'
	CreatedAt time.Time
}

// FabricLease is an exclusive hold on a target (§11.4.119). ReleaseAt zero/nil
// means the lease is currently held; at most one ACTIVE lease may exist per
// exclusive target (the SCHEMA.sql UNIQUE partial index + the memory guard).
type FabricLease struct {
	LeaseID    string
	TargetID   string
	Owner      string // the run/stream holding it
	AcquiredAt time.Time
	ReleaseAt  *time.Time // nil while held
}

// FabricRun is one test run on a target (helix_ota.fabric_runs). Verdict uses
// the closed §11.4.45 vocabulary.
type FabricRun struct {
	RunID      string
	TargetID   string
	TestType   string // 'unit'|'integration'|'e2e'|'security'|'chaos'|'stress'|'challenge'|...
	TestRef    string // script/path/bank-id dispatched
	Verdict    string // 'PASS'|'FAIL'|'SKIP'|'PENDING_FORENSICS'|'OPERATOR-BLOCKED'|'PENDING'
	SkipReason string
	StartedAt  time.Time
	EndedAt    *time.Time
}

// FabricEvidence is one evidence-ledger row (helix_ota.fabric_evidence). A PASS
// run MUST link >=1 non-empty artefact (§11.4.69): ByteSize MUST be > 0, enforced
// by a CHECK in pgx and a guard in memory.
type FabricEvidence struct {
	EvidenceID string
	RunID      string
	Kind       string // 'transcript'|'screencap'|'ab-slot'|'dm-verity'|'latency'|'sink-probe'|...
	Path       string // docs/qa/<run-id>/... (§11.4.83)
	ByteSize   int64  // non-empty by construction (> 0)
	SHA256     string
	CreatedAt  time.Time
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
	// ReleaseByVersion resolves an exact os+target+version to its release (delta
	// base-artifact lookup), or ErrNotFound.
	ReleaseByVersion(ctx context.Context, os otaprotocol.OSType, targetModel, version string) (Release, error)
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
	AddGroupMember(ctx context.Context, groupID, deviceID string, addedAt time.Time) error
	ListGroupMembers(ctx context.Context, groupID string) ([]string, error)
	// ListGroupMembersDetailed returns members with their join time, oldest-first.
	ListGroupMembersDetailed(ctx context.Context, groupID string) ([]GroupMember, error)
	RemoveGroupMember(ctx context.Context, groupID, deviceID string) error

	// Idempotency support for register/deployment replay (endpoints.md §2).
	GetIdempotent(ctx context.Context, key string) (string, bool)
	PutIdempotent(ctx context.Context, key, resultID string)

	// Emulation test-fabric registry (docs/design/emulation_fabric/SCHEMA.sql).
	// A node/target is upsert-by-id; AcquireFabricLease enforces the exclusive
	// single-active-lease invariant (§11.4.119) and returns ErrConflict when the
	// exclusive target is already leased; AttachFabricEvidence rejects a 0-byte
	// artefact with ErrEvidenceEmpty (§11.4.69).
	CreateFabricNode(ctx context.Context, n FabricNode) error
	GetFabricNode(ctx context.Context, nodeID string) (FabricNode, error)
	CreateFabricTarget(ctx context.Context, t FabricTarget) error
	GetFabricTarget(ctx context.Context, targetID string) (FabricTarget, error)
	ListFabricTargets(ctx context.Context) ([]FabricTarget, error)
	// AcquireFabricLease records an exclusive hold. For an exclusive target with
	// an already-active (release_at NULL) lease it returns ErrConflict.
	AcquireFabricLease(ctx context.Context, l FabricLease) error
	// ReleaseFabricLease marks the lease released (release_at set). Idempotent;
	// ErrNotFound when the lease id is unknown.
	ReleaseFabricLease(ctx context.Context, leaseID string, releaseAt time.Time) error
	ActiveFabricLease(ctx context.Context, targetID string) (FabricLease, error)
	CreateFabricRun(ctx context.Context, r FabricRun) error
	GetFabricRun(ctx context.Context, runID string) (FabricRun, error)
	UpdateFabricRun(ctx context.Context, r FabricRun) error
	// AttachFabricEvidence appends a non-empty evidence artefact to a run.
	// ByteSize <= 0 returns ErrEvidenceEmpty; an unknown run id returns ErrNotFound.
	AttachFabricEvidence(ctx context.Context, e FabricEvidence) error
	ListFabricEvidence(ctx context.Context, runID string) ([]FabricEvidence, error)
}
