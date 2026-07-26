package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	engine "github.com/HelixDevelopment/ota-rollout-engine"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/rollout"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// UserDirectory authenticates admin/operator/viewer credentials for the login
// stub (endpoints.md §7.1). In production this is the `auth` brick / identity
// store; the MVP wires a static directory. It returns the roles for the
// authenticated user, or ok=false on bad credentials.
type UserDirectory interface {
	Authenticate(username, password string) (roles []string, ok bool)
}

// Server holds the wired dependencies for the REST API handlers.
type Server struct {
	cfg     config.Config
	repo    store.Repository
	signer  *TokenSigner
	users   UserDirectory
	health  health.Checker
	pubKey         ed25519.PublicKey
	prevPubKey     ed25519.PublicKey
	target         otavalidator.TargetPolicy
	refresh *refreshStore
	rollout *rollout.Service
	metrics *Metrics
	nowFn   func() time.Time
	newIDFn func() string

	// Degraded indicates the server started with a fallback persistence layer
	// (e.g. in-memory store after PostgreSQL connection failure). When true the
	// /healthz endpoint reports {"status":"degraded"} so an operator can detect
	// the degraded state programmatically.
	Degraded bool

	// deployMu serializes the deployment-creation critical section (endpoints.md
	// §11.1's check-then-act invariant "at most one active deployment per
	// os+target_model+group"). store.Repository.ActiveDeploymentForTarget (the
	// check) and CreateDeployment (the act) each lock the store independently but
	// atomically only within themselves — two concurrent POST /deployments
	// requests targeting the same release can both observe "no active
	// deployment" via ActiveDeploymentForTarget before either has called
	// CreateDeployment, so both proceed to create an active deployment for the
	// same target (a TOCTOU business-invariant violation, not a data race the
	// race detector flags, since every individual store call is itself
	// correctly locked). deployMu makes the whole check+create+idempotency-key
	// sequence atomic relative to itself, closing that window.
	deployMu sync.Mutex

	// releaseMu serializes the release-creation critical section (endpoints.md
	// §10.1's check-then-act invariant "version must be strictly greater than
	// the latest published release for this target"). The exact same TOCTOU
	// shape as deployMu above: store.Repository.LatestRelease (the check) and
	// CreateRelease (the act) are two separate, individually-locked store
	// calls — two concurrent POST /releases requests for the same
	// os+target_model (including two requests carrying the IDENTICAL version)
	// can both observe the same "latest" before either has called
	// CreateRelease, so both pass the monotonicity check and both create a
	// release, defeating the documented "strictly greater than latest"
	// invariant (and, when the two submitted versions are equal, silently
	// storing a duplicate (os, target_model, version) release row). releaseMu
	// makes the whole check+create sequence atomic relative to itself.
	releaseMu sync.Mutex

	// roleVersionMu guards the roleVersions map. When a user's role changes
	// (SetAccountMembership), the version is incremented so any existing token
	// carrying the old version is rejected at the next authenticated request
	// (T040 — session invalidation on role change).
	roleVersionMu  sync.Mutex
	roleVersions   map[string]int64
}

// Options configures a Server. Fields left nil/zero fall back to sensible
// defaults (real clock, random UUID-ish ids, always-ready health).
type Options struct {
	Config       config.Config
	Repo         store.Repository
	Users        UserDirectory
	Health       health.Checker
	TargetPolicy otavalidator.TargetPolicy
	ArtifactKey  ed25519.PublicKey
	Now          func() time.Time
	NewID        func() string
	// Rollout, when non-nil, is the staged-rollout service the server uses (e.g.
	// a pgx-backed one in production). Nil falls back to an in-memory service.
	Rollout *rollout.Service
	// Metrics, when non-nil, is the registered Prometheus metric set the server
	// exposes at GET /metrics and records on every request. Nil (the typical
	// production path) creates the default-process-registry metric set.
	Metrics *Metrics
}

// NewServer builds a Server from the given options.
func NewServer(opts Options) *Server {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	newID := opts.NewID
	if newID == nil {
		newID = newRandomID
	}
	checker := opts.Health
	if checker == nil {
		checker = health.New(nil)
	}
	policy := opts.TargetPolicy
	if policy == nil {
		// Default Phase-1 policy: Android on any board is known + supported.
		// Production supplies a database-backed policy (artifact_validation.md
		// §5.5).
		policy = defaultAndroidPolicy{}
	}
	pubKey := opts.ArtifactKey
	if pubKey == nil && len(opts.Config.ArtifactPublicKey) == ed25519.PublicKeySize {
		pubKey = ed25519.PublicKey(opts.Config.ArtifactPublicKey)
	}
	prevPubKey := ed25519.PublicKey(nil)
	if len(opts.Config.PreviousArtifactPublicKey) == ed25519.PublicKeySize &&
		opts.Config.SigningKeyRotationInterval > 0 {
		prevPubKey = ed25519.PublicKey(opts.Config.PreviousArtifactPublicKey)
	}
	rolloutSvc := opts.Rollout
	if rolloutSvc == nil {
		rolloutSvc = rollout.NewService(now)
	}
	return &Server{
		cfg:          opts.Config,
		repo:         opts.Repo,
		signer:       NewTokenSigner(opts.Config.TokenSecret),
		users:        opts.Users,
		health:       checker,
		pubKey:       pubKey,
		prevPubKey:   prevPubKey,
		target:       policy,
		refresh:      newRefreshStore(),
		rollout:      rolloutSvc,
		metrics:      opts.Metrics, // nil when not provided — Router skips metrics then
		nowFn:        now,
		newIDFn:      newID,
		roleVersions: make(map[string]int64),
	}
}

// StartRolloutAutoProgress launches a background goroutine that periodically
// polls active rollouts and auto-advances phases whose duration has elapsed.
func (s *Server) StartRolloutAutoProgress(ctx context.Context) {
	if s.cfg.RolloutPollInterval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(s.cfg.RolloutPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, depID := range s.rollout.ActiveDeploymentIDs() {
					_, err := s.rollout.Evaluate(ctx, depID, engine.HealthVerdict{
						SuccessRate: 1.0,
						ErrorRate:   0.0,
					})
					if err != nil {
						log.Printf("rollout auto-progress: %s: %v", depID, err)
					}
				}
			}
		}
	}()
}

// now returns the current time via the (possibly injected) clock.
func (s *Server) now() time.Time { return s.nowFn().UTC() }

// newID returns a fresh opaque identifier via the (possibly injected) generator.
func (s *Server) newID() string { return s.newIDFn() }

// Router builds the Gin engine with all middleware and routes registered under
// the configured API base path, plus the unversioned /healthz and /readyz
// probes.
func (s *Server) Router() *gin.Engine {
	r := gin.New()
	// SECURITY: gin.New() defaults Engine.trustedProxies to {"0.0.0.0/0", "::/0"}
	// -- i.e. it trusts EVERY source as a legitimate reverse proxy. Left at that
	// default, Context.ClientIP() (consumed by auditMiddleware for the audit
	// log's IPAddress field, operational_endpoints.md §4.1) honors an
	// X-Forwarded-For/X-Real-Ip header supplied by ANY direct, untrusted caller,
	// letting an attacker forge the IP address their own mutating action is
	// attributed to in the audit trail. Disabling trusted-proxy parsing entirely
	// (nil) makes ClientIP() fall back to the genuine TCP peer address
	// (r.RemoteAddr) unconditionally. This control plane has no documented
	// reverse-proxy deployment topology that forwards X-Forwarded-For (the
	// existing cfg.TrustTLSProxy flag concerns TLS-only response headers, not IP
	// provenance) so the safe default is to trust no proxy.
	if err := r.SetTrustedProxies(nil); err != nil {
		// SetTrustedProxies(nil) cannot fail (it skips CIDR parsing entirely) --
		// panic here would only mask an unexpected upstream gin behavior change.
		panic("api: SetTrustedProxies(nil): " + err.Error())
	}
	// compressionMiddleware negotiates Brotli -> gzip -> identity and sets
	// `Vary: Accept-Encoding` (superseding the bare varyMiddleware). The optional
	// in-flight cap (HELIX_MAX_INFLIGHT) sheds excess load with 429 before any
	// handler work — DoS protection, default-off.
	//
	// securityHeadersMiddleware (Item O, Tier A) sets the global security response
	// headers on EVERY response. It runs before the in-flight limiter so even a
	// 429-shed and error/panic responses carry them; HSTS is emitted only when
	// TLS is configured.
	// OTA-034 observability: metrics middleware first (measures full request
	// lifecycle including all downstream middleware), then the existing chain
	// with structured logging injected after the request-id is assigned.
	if s.metrics != nil {
		r.Use(s.metrics.Middleware())
	}
	r.Use(recoveryMiddleware(), requestIDMiddleware(),
		StructuredLoggingMiddleware(), securityHeadersMiddleware(s.tlsEnabled()),
		maxInflightMiddleware(s.cfg.MaxInflight),
		compressionMiddleware())

	// Health/readiness and metrics are unversioned, unauthenticated operational probes.
	r.GET("/healthz", s.handleHealthz)
	r.GET("/readyz", s.handleReadyz)
	if s.metrics != nil {
		r.GET("/metrics", s.metrics.Handler())
	}

	v1 := r.Group(s.cfg.APIBasePath)
	// apiSecurityHeadersMiddleware (Item O, Tier B) adds the strict JSON CSP
	// (default-src 'none') + Cache-Control: no-store to every API response,
	// including the public auth endpoints and API error paths. Scoped to the API
	// group so the /manager SPA's hashed assets keep their cacheable headers.
	v1.Use(apiSecurityHeadersMiddleware())

	// Public auth endpoints (endpoints.md §7).
	v1.POST("/auth/login", s.handleLogin)
	v1.POST("/auth/refresh", s.handleRefresh)
	// Accounts M4: POST /auth/select-account — the caller must carry a valid
	// (unscoped, post-login) token; the handler verifies membership in the
	// target account and returns an account-scoped access+refresh pair.
	v1.POST("/auth/select-account", s.authMiddleware(), s.handleSelectAccount)

	// Protected endpoints: authenticate, enforce per-route roles, then audit any
	// successful mutating action (auditMiddleware runs after the handler).
	auth := v1.Group("")
	auth.Use(s.authMiddleware(), s.tenantSessionMiddleware(), piiDetectionMiddleware(), s.auditMiddleware())
	{
		// --- Super-admin routes (design §4.1) ---
		// These bypass tenant isolation entirely — the super_admin global flag
		// is the only gate. Account creation/management is super-admin-only.
		auth.GET("/admin/accounts", requireSuperAdmin(), s.handleAdminListAccounts)
		auth.POST("/admin/accounts", requireSuperAdmin(), s.handleAdminCreateAccount)
		auth.GET("/admin/accounts/:id", requireSuperAdmin(), s.handleAdminGetAccount)
		auth.PATCH("/admin/accounts/:id", requireSuperAdmin(), s.handleAdminUpdateAccount)
		auth.DELETE("/admin/accounts/:id", requireSuperAdmin(), s.handleAdminDeleteAccount)
		auth.POST("/admin/accounts/:id/suspend", requireSuperAdmin(), s.handleAdminSuspendAccount)
		auth.POST("/admin/accounts/:id/unsuspend", requireSuperAdmin(), s.handleAdminUnsuspendAccount)
		auth.POST("/admin/accounts/:id/archive", requireSuperAdmin(), s.handleAdminArchiveAccount)
		auth.POST("/admin/accounts/:id/members", requireSuperAdmin(), s.handleAdminSetAccountMembership)

		// --- Account-scoped management routes (path-based, design §4.1) ---
		// These read the target account from the URL path :accountId param and
		// enforce requireAccountAccess (which re-verifies membership). Used for
		// cross-account admin / account-management operations.
		auth.GET("/accounts/:accountId/projects",
			s.requireAccountAccess(store.AccountRoleViewer), s.handleListAccountProjects)
		auth.GET("/accounts/:accountId/updates",
			s.requireAccountAccess(store.AccountRoleViewer), s.handleListAccountUpdates)
		auth.POST("/accounts/:accountId/devices",
			s.requireAccountAccess(store.AccountRoleOperator), s.handleRegisterDeviceForAccount)

		// --- OTA operational routes (claim-scoped, design §4.2) ---
		// Every device / release / deployment / artifact / delta / rollout /
		// rollback / client / telemetry / group / audit / project route is scoped
		// to the account carried in the verified JWT token claim. The subgroup
		// applies requireClaimAccountAccess(viewer) uniformly — every caller
		// must belong to the token's account at viewer-or-higher. Per-route
		// requireRole() enforces the GLOBAL RBAC role as before (the two
		// middleware stack: claim-account → global-role).
		//
		// Backward compatibility: when the token carries no account_id claim
		// (legacy single-tenant token), requireClaimAccountAccess passes through
		// with an empty-string scope — existing callers keep working during the
		// migration window.
		ota := auth.Group("")
		ota.Use(s.requireClaimAccountAccess(store.AccountRoleViewer))
		{
			// Devices.
			ota.POST("/devices/register", requireRole(RoleOperator, RoleAdmin), s.handleRegisterDevice)
			ota.GET("/devices", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListDevices)
			ota.GET("/devices/by-hardware/:hardwareId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleDeviceByHardware)
			ota.GET("/devices/:deviceId/status", requireRole(RoleViewer, RoleOperator, RoleAdmin, RoleDevice), s.handleDeviceStatus)

			// Artifacts.
			ota.POST("/artifacts/upload", requireRole(RoleOperator, RoleAdmin), s.handleUploadArtifact)
			ota.GET("/artifacts/:artifactId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetArtifact)

			// Delta artifacts (delta_updates_design.md §4): register + lookup a
			// base->target delta. Register is operator/admin; lookup is viewer+.
			ota.POST("/deltas", requireRole(RoleOperator, RoleAdmin), s.handleRegisterDelta)
			ota.GET("/deltas", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleFindDelta)

			// Releases.
			ota.POST("/releases", requireRole(RoleOperator, RoleAdmin), s.handleCreateRelease)
			ota.GET("/releases", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListReleases)
			ota.GET("/releases/:releaseId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetRelease)

			// Deployments.
			ota.POST("/deployments", requireRole(RoleOperator, RoleAdmin), s.handleCreateDeployment)
			ota.GET("/deployments", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListDeployments)
			ota.GET("/deployments/:deploymentId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetDeployment)

			// Staged rollout (1.0.1-staged-rollout/rollout_engine.md §8) — reuses the
			// ota-rollout-engine brick. Create/start + evaluate are operator/admin.
			ota.POST("/deployments/:deploymentId/rollout", requireRole(RoleOperator, RoleAdmin), s.handleCreateRollout)
			ota.GET("/deployments/:deploymentId/rollout", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetRollout)
			ota.POST("/deployments/:deploymentId/rollout/evaluate", requireRole(RoleOperator, RoleAdmin), s.handleEvaluateRollout)

			// Server-driven recall (rollback) + history (rollback_ux.md §7).
			ota.POST("/deployments/:deploymentId/recall", requireRole(RoleOperator, RoleAdmin), s.handleRecall)
			ota.GET("/deployments/:deploymentId/rollbacks", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListRollbacks)

			// Client (device-facing).
			ota.GET("/client/update", requireRole(RoleDevice), s.handleClientUpdate)
			ota.POST("/client/telemetry", requireRole(RoleDevice), s.handleClientTelemetry)

			// Telemetry reads (operational_endpoints.md §5). Device may read its own.
			ota.GET("/devices/:deviceId/telemetry", requireRole(RoleViewer, RoleOperator, RoleAdmin, RoleDevice), s.handleDeviceTelemetry)
			ota.GET("/telemetry/overview", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleTelemetryOverview)

			// Device groups (operational_endpoints.md §6). Writes operator/admin;
			// group delete is admin-only; reads viewer+.
			ota.POST("/groups", requireRole(RoleOperator, RoleAdmin), s.handleCreateGroup)
			ota.GET("/groups", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListGroups)
			ota.GET("/groups/:groupId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetGroup)
			ota.PATCH("/groups/:groupId", requireRole(RoleOperator, RoleAdmin), s.handleUpdateGroup)
			ota.DELETE("/groups/:groupId", requireRole(RoleAdmin), s.handleDeleteGroup)
			ota.GET("/groups/:groupId/members", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListGroupMembers)
			ota.POST("/groups/:groupId/members", requireRole(RoleOperator, RoleAdmin), s.handleAddGroupMembers)
			ota.DELETE("/groups/:groupId/members/:deviceId", requireRole(RoleOperator, RoleAdmin), s.handleRemoveGroupMember)

			// Audit log read (operational_endpoints.md §4.3) — admin only.
			ota.GET("/audit", requireRole(RoleAdmin), s.handleListAudit)

			// Projects (multi-project support, account-scoped via token claim).
			// Writes operator/admin, delete admin-only, reads viewer+.
			ota.POST("/projects", requireRole(RoleOperator, RoleAdmin), s.handleCreateProject)
			ota.GET("/projects", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListProjects)
			ota.GET("/projects/:projectId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetProject)
			ota.PATCH("/projects/:projectId", requireRole(RoleAdmin), s.handleUpdateProject)
			ota.DELETE("/projects/:projectId", requireRole(RoleAdmin), s.handleDeleteProject)

			// Webhooks — project-scoped callback registrations for event-driven
			// delivery (production operations baseline US1).
			ota.POST("/webhooks", requireRole(RoleOperator, RoleAdmin), s.handleCreateWebhook)
			ota.GET("/webhooks", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListWebhooks)
			ota.DELETE("/webhooks/:id", requireRole(RoleOperator, RoleAdmin), s.handleDeleteWebhook)

			// Branches (migration 5) — project-scoped release channels.
			ota.POST("/branches", requireRole(RoleOperator, RoleAdmin), s.handleCreateBranch)
			ota.GET("/branches", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListBranches)
			ota.GET("/branches/:id", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetBranch)
			ota.PATCH("/branches/:id", requireRole(RoleOperator, RoleAdmin), s.handleUpdateBranch)
			ota.DELETE("/branches/:id", requireRole(RoleOperator, RoleAdmin), s.handleDeleteBranch)

			// Project members — role-based access within a project.
			ota.GET("/projects/:projectId/members", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListProjectMembers)
			ota.POST("/projects/:projectId/members", requireRole(RoleAdmin), s.handleAddProjectMember)
			ota.PATCH("/projects/:projectId/members/:userId", requireRole(RoleAdmin), s.handleUpdateProjectMember)
			ota.DELETE("/projects/:projectId/members/:userId", requireRole(RoleOperator, RoleAdmin), s.handleRemoveProjectMember)

			// Delta generation (on-demand compute).
			ota.POST("/deltas/generate", requireRole(RoleOperator, RoleAdmin), s.handleGenerateDelta)

			// Fabric registry (emulation test-fabric) — admin-only at this tier.
			ota.POST("/fabric/nodes", requireRole(RoleAdmin), s.handleRegisterFabricNode)
			ota.GET("/fabric/nodes/:nodeId", requireRole(RoleAdmin), s.handleGetFabricNode)
			ota.POST("/fabric/targets", requireRole(RoleAdmin), s.handleRegisterFabricTarget)
			ota.GET("/fabric/targets", requireRole(RoleAdmin), s.handleListFabricTargets)
		}
	}

	s.MountManagerUI(r)

	return r
}

// defaultAndroidPolicy is the MVP fallback TargetPolicy: every Android board is
// a known, supported Phase-1 target (artifact_validation.md §5.5). Non-Android
// OS types are rejected.
type defaultAndroidPolicy struct{}

// Known reports whether the target is a recognized fleet target.
func (defaultAndroidPolicy) Known(os otaprotocol.OSType, board string) bool {
	return os == otaprotocol.OSAndroid && board != ""
}

// Supported reports whether the target is accepted for Phase-1.
func (defaultAndroidPolicy) Supported(os otaprotocol.OSType, board string) bool {
	return os == otaprotocol.OSAndroid && board != ""
}

// newRandomID returns a random 16-byte hex identifier. Clients treat ids as
// opaque (endpoints.md §2); the OpenAPI models them as UUIDs, but any opaque
// string satisfies the contract.
func newRandomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}

// IncrementRoleVersion bumps the role version for a user so any existing
// token carrying the old version is rejected at the next authenticated
// request (T040 — session invalidation on role change).
func (s *Server) IncrementRoleVersion(userID string) int64 {
	s.roleVersionMu.Lock()
	v := s.roleVersions[userID] + 1
	s.roleVersions[userID] = v
	s.roleVersionMu.Unlock()
	return v
}

// GetRoleVersion returns the current role version for a user. A user with
// no stored version returns 0 (the compatible legacy default — any token
// minted before T040 carries an implicit version of 0).
func (s *Server) GetRoleVersion(userID string) int64 {
	s.roleVersionMu.Lock()
	v := s.roleVersions[userID]
	s.roleVersionMu.Unlock()
	return v
}
