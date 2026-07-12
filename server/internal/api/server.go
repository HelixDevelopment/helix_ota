package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
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
	pubKey  ed25519.PublicKey
	target  otavalidator.TargetPolicy
	refresh *refreshStore
	rollout *rollout.Service
	nowFn   func() time.Time
	newIDFn func() string

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
	rolloutSvc := opts.Rollout
	if rolloutSvc == nil {
		rolloutSvc = rollout.NewService(now)
	}
	return &Server{
		cfg:     opts.Config,
		repo:    opts.Repo,
		signer:  NewTokenSigner(opts.Config.TokenSecret),
		users:   opts.Users,
		health:  checker,
		pubKey:  pubKey,
		target:  policy,
		refresh: newRefreshStore(),
		rollout: rolloutSvc,
		nowFn:   now,
		newIDFn: newID,
	}
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
	r.Use(recoveryMiddleware(), requestIDMiddleware(), securityHeadersMiddleware(s.tlsEnabled()),
		maxInflightMiddleware(s.cfg.MaxInflight), rateLimitMiddleware(s.cfg.RateLimitRPS),
		compressionMiddleware())

	// Health/readiness are unversioned, unauthenticated operational probes.
	r.GET("/healthz", s.handleHealthz)
	r.GET("/readyz", s.handleReadyz)

	v1 := r.Group(s.cfg.APIBasePath)
	// apiSecurityHeadersMiddleware (Item O, Tier B) adds the strict JSON CSP
	// (default-src 'none') + Cache-Control: no-store to every API response,
	// including the public auth endpoints and API error paths. Scoped to the API
	// group so the /manager SPA's hashed assets keep their cacheable headers.
	v1.Use(apiSecurityHeadersMiddleware())

	// Public auth endpoints (endpoints.md §7).
	v1.POST("/auth/login", authRateLimitMiddleware(s.cfg.AuthRateLimit), s.handleLogin)
	v1.POST("/auth/refresh", s.handleRefresh)

	// Protected endpoints: authenticate, enforce per-route roles, then audit any
	// successful mutating action (auditMiddleware runs after the handler).
	auth := v1.Group("")
	auth.Use(s.authMiddleware(), s.auditMiddleware())
	{
		auth.POST("/devices/register", requireRole(RoleOperator, RoleAdmin), s.handleRegisterDevice)
		auth.GET("/devices", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListDevices)
		auth.GET("/devices/by-hardware/:hardwareId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleDeviceByHardware)
		auth.GET("/devices/:deviceId/status", requireRole(RoleViewer, RoleOperator, RoleAdmin, RoleDevice), s.handleDeviceStatus)

		auth.POST("/artifacts/upload", requireRole(RoleOperator, RoleAdmin), s.handleUploadArtifact)
		auth.GET("/artifacts/:artifactId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetArtifact)

		// Delta artifacts (delta_updates_design.md §4): register + lookup a
		// base->target delta. Register is operator/admin; lookup is viewer+.
		auth.POST("/deltas", requireRole(RoleOperator, RoleAdmin), s.handleRegisterDelta)
		auth.GET("/deltas", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleFindDelta)

		auth.POST("/releases", requireRole(RoleOperator, RoleAdmin), s.handleCreateRelease)
		auth.GET("/deployments/:deploymentId/rollbacks", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListRollbacks)

		auth.GET("/client/update", requireRole(RoleDevice), s.handleClientUpdate)
		auth.POST("/client/telemetry", requireRole(RoleDevice), s.handleClientTelemetry)

		// Telemetry reads (operational_endpoints.md §5). Device may read its own.
		auth.GET("/devices/:deviceId/telemetry", requireRole(RoleViewer, RoleOperator, RoleAdmin, RoleDevice), s.handleDeviceTelemetry)
		auth.GET("/telemetry/overview", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleTelemetryOverview)

		// Device groups (operational_endpoints.md §6). Writes operator/admin;
		// group delete is admin-only; reads viewer+.
		auth.POST("/groups", requireRole(RoleOperator, RoleAdmin), s.handleCreateGroup)
		auth.GET("/groups", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListGroups)
		auth.GET("/groups/:groupId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetGroup)
		auth.PATCH("/groups/:groupId", requireRole(RoleOperator, RoleAdmin), s.handleUpdateGroup)
		auth.DELETE("/groups/:groupId", requireRole(RoleAdmin), s.handleDeleteGroup)
		auth.GET("/groups/:groupId/members", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListGroupMembers)
		auth.POST("/groups/:groupId/members", requireRole(RoleOperator, RoleAdmin), s.handleAddGroupMembers)
		auth.DELETE("/groups/:groupId/members/:deviceId", requireRole(RoleOperator, RoleAdmin), s.handleRemoveGroupMember)

		// Audit log read (operational_endpoints.md §4.3) — admin only.
		auth.GET("/audit", requireRole(RoleAdmin), s.handleListAudit)

		// Accounts M2 — super-admin (design §4.1): list/create/manage accounts.
		auth.GET("/admin/accounts", requireSuperAdmin(), s.handleAdminListAccounts)

		// Accounts M2 — account-scoped (design §4.1): list projects for this account.
		auth.GET("/accounts/:accountId/projects",
			s.requireAccountAccess(store.AccountRoleViewer), s.handleListAccountProjects)

		// Projects (multi-project support). Writes operator/admin, delete admin-only,
		// reads viewer+.
		auth.POST("/projects", requireRole(RoleOperator, RoleAdmin), s.handleCreateProject)
		auth.GET("/projects", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListProjects)
		auth.GET("/projects/:projectId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetProject)
		auth.PATCH("/projects/:projectId", requireRole(RoleAdmin), s.handleUpdateProject)
		auth.DELETE("/projects/:projectId", requireRole(RoleAdmin), s.handleDeleteProject)
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
