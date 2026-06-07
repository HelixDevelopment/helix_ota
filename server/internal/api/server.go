package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
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
	nowFn   func() time.Time
	newIDFn func() string
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
	return &Server{
		cfg:     opts.Config,
		repo:    opts.Repo,
		signer:  NewTokenSigner(opts.Config.TokenSecret),
		users:   opts.Users,
		health:  checker,
		pubKey:  pubKey,
		target:  policy,
		refresh: newRefreshStore(),
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
	r.Use(recoveryMiddleware(), requestIDMiddleware(), varyMiddleware())

	// Health/readiness are unversioned, unauthenticated operational probes.
	r.GET("/healthz", s.handleHealthz)
	r.GET("/readyz", s.handleReadyz)

	v1 := r.Group(s.cfg.APIBasePath)

	// Public auth endpoints (endpoints.md §7).
	v1.POST("/auth/login", s.handleLogin)
	v1.POST("/auth/refresh", s.handleRefresh)

	// Protected endpoints: authenticate, then enforce per-route roles.
	auth := v1.Group("")
	auth.Use(s.authMiddleware())
	{
		auth.POST("/devices/register", requireRole(RoleOperator, RoleAdmin), s.handleRegisterDevice)
		auth.GET("/devices/:deviceId/status", requireRole(RoleViewer, RoleOperator, RoleAdmin, RoleDevice), s.handleDeviceStatus)

		auth.POST("/artifacts/upload", requireRole(RoleOperator, RoleAdmin), s.handleUploadArtifact)
		auth.GET("/artifacts/:artifactId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetArtifact)

		auth.POST("/releases", requireRole(RoleOperator, RoleAdmin), s.handleCreateRelease)
		auth.GET("/releases", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleListReleases)
		auth.GET("/releases/:releaseId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetRelease)

		auth.POST("/deployments", requireRole(RoleOperator, RoleAdmin), s.handleCreateDeployment)
		auth.GET("/deployments/:deploymentId", requireRole(RoleViewer, RoleOperator, RoleAdmin), s.handleGetDeployment)

		auth.GET("/client/update", requireRole(RoleDevice), s.handleClientUpdate)
		auth.POST("/client/telemetry", requireRole(RoleDevice), s.handleClientTelemetry)
	}

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
