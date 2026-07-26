// Package config provides env-based runtime configuration for the Helix OTA
// control-plane server. Per ADR-0003/architecture.md the modular monolith reads
// poll cadence, TTLs, limits, and the API base path from configuration (the
// `config` brick) rather than hard-coding them. No secrets are embedded in code;
// the JWT signing secret and trusted artifact public key are supplied via env.
package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// Default values used when the corresponding environment variable is unset.
const (
	// DefaultPort is the TCP port the plain-HTTP server listens on.
	DefaultPort = "8080"
	// DefaultHTTPSPort is the port for the TLS HTTP/2 + HTTP/3 listeners.
	DefaultHTTPSPort = "8443"
	// DefaultAPIBasePath is the REST base path (endpoints.md §2).
	DefaultAPIBasePath = "/api/v1"
	// DefaultPollInterval is the device update-check cadence base (endpoints.md
	// §12.1: 15 min + jitter).
	DefaultPollInterval = 15 * time.Minute
	// DefaultPollJitter is the additional random jitter applied to the poll
	// cadence to de-synchronize the fleet.
	DefaultPollJitter = 2 * time.Minute
	// DefaultAccessTokenTTL is the access-token lifetime (endpoints.md §4.1).
	DefaultAccessTokenTTL = 15 * time.Minute
	// DefaultDeviceTokenTTL is the device-scoped bearer lifetime (endpoints.md
	// §8.1 example: 86400s).
	DefaultDeviceTokenTTL = 24 * time.Hour
	// DefaultMaxUploadBytes caps the artifact upload size (endpoints.md §9.1
	// 413 PAYLOAD_TOO_LARGE). Default 2 GiB.
	DefaultMaxUploadBytes int64 = 2 << 30
	// DefaultArtifactBaseURL is the base URL the update-check uses to build the
	// Range-served, identity-encoded artifact download reference (endpoints.md
	// §12.1). The byte path itself is the Storage brick's concern.
	DefaultArtifactBaseURL = "https://artifacts.helix.example"
	DefaultRateLimitRPS = 100
	DefaultAuthRateLimit = 5
	// DefaultMaxInflight bounds concurrent in-flight requests to protect against
	// connection-flood DoS attacks; excess requests are shed with 429 RATE_LIMITED.
	DefaultMaxInflight int64 = 1000
	// DefaultRolloutPollInterval controls how often the rollout auto-progress
	// scheduler polls for pending window activations.
	DefaultRolloutPollInterval = 60 * time.Second
	// DefaultSigningKeyRotationInterval is the default grace period during which
	// the previous artifact signing key remains valid alongside the new key
	// (T043 — credential rotation). Zero means no rotation window is active.
	DefaultSigningKeyRotationInterval = 0 * time.Hour
	// DefaultArtifactUploadTimeout bounds the total time a single artifact upload
	// handler may spend processing. Exceeding the timeout aborts with 504.
	DefaultArtifactUploadTimeout = 5 * time.Minute
	// DefaultRolloutEvaluationTimeout bounds the rollout evaluation handler's
	// maximum execution time. Exceeding the timeout aborts the request.
	DefaultRolloutEvaluationTimeout = 30 * time.Second
	// DefaultDBConnectionPoolSize is the maximum number of pgx pool connections.
	// 0 means use pgxpool defaults (max(4, runtime.NumCPU())).
	DefaultDBConnectionPoolSize = 0
)

// Config is the resolved server configuration.
type Config struct {
	// Port is the listen port.
	Port string
	// APIBasePath is the REST base path (e.g. /api/v1).
	APIBasePath string
	// PollInterval is the device update-check cadence base.
	PollInterval time.Duration
	// PollJitter is the jitter added on top of PollInterval.
	PollJitter time.Duration
	// AccessTokenTTL is the admin/operator/viewer access-token lifetime.
	AccessTokenTTL time.Duration
	// DeviceTokenTTL is the device-scoped bearer-token lifetime.
	DeviceTokenTTL time.Duration
	// MaxUploadBytes caps the artifact upload body size.
	MaxUploadBytes int64
	// MaxInflight bounds concurrent in-flight requests to protect against
	// connection-flood DoS attacks; excess requests are shed with 429 RATE_LIMITED.
	// Default: DefaultMaxInflight (1000). Set to 0 to disable the cap.
	MaxInflight int64
	// RolloutPollInterval controls how often the rollout auto-progress scheduler
	// polls for pending window activations.
	RolloutPollInterval time.Duration
	// ArtifactBaseURL is the base of the artifact download reference.
	ArtifactBaseURL string

	// RateLimitRPS is the per-IP request allowance (token-bucket refill rate).
	// Default 100 req/s. A non-positive value disables rate limiting.
	RateLimitRPS int
	// AuthRateLimit is the per-IP login attempt cap per minute for POST
	// /auth/login (brute-force guard). Default 5 req/min. Non-positive disables.
	AuthRateLimit int

	// TokenSecret is the symmetric secret used to sign/verify the opaque bearer
	// tokens this MVP mints. Supplied via HELIX_TOKEN_SECRET; a development
	// default is used when unset (and a warning is the caller's concern). Never
	// hard-coded as a secret in code.
	TokenSecret []byte

	// ArtifactPublicKey is the trusted ed25519 build-pipeline public key used by
	// the artifact-validator S3 signature stage (artifact_validation.md §5.3).
	// Supplied base64-encoded via HELIX_ARTIFACT_PUBKEY. May be empty in
	// configurations that inject the key by other means (e.g. tests); the upload
	// handler rejects uploads when no trusted key is configured.
	ArtifactPublicKey []byte

	// PreviousArtifactPublicKey is the PREVIOUS trusted ed25519 public key that
	// remains valid during the SigningKeyRotationInterval grace period (T043).
	// When the operator rotates to a new signing key, they set the new key in
	// HELIX_ARTIFACT_PUBKEY, move the old key here, and set
	// HELIX_ARTIFACT_SIGNING_KEY_ROTATION_INTERVAL to the grace duration. During
	// this window artifacts signed by EITHER the current key or this previous key
	// are accepted. When the interval expires (or is reset to zero), only the
	// current key is trusted. Supplied base64-encoded via
	// HELIX_ARTIFACT_PREVIOUS_PUBKEY.
	PreviousArtifactPublicKey []byte

	// SigningKeyRotationInterval is the grace period during which the PREVIOUS
	// artifact signing key remains valid alongside the NEW key (T043). During
	// this window the server accepts artifacts signed by EITHER key, letting the
	// operator distribute the new public key to build pipelines without downtime.
	// Zero disables rotation (only the current pubkey is trusted). Supplied via
	// HELIX_ARTIFACT_SIGNING_KEY_ROTATION_INTERVAL (Go duration format).
	SigningKeyRotationInterval time.Duration

	// TLSCertFile / TLSKeyFile enable the HTTP/3 (QUIC) + HTTP/2 transport. When
	// BOTH are set the control plane is served over HTTP/3 with automatic HTTP/2
	// fallback (ADR-0004) on HTTPSPort; when NEITHER is set it serves plain HTTP on
	// Port (development). Setting exactly one is a fail-close configuration error in
	// Load() (SRV-NEW-4/OTA-065) — a half-configured pair would otherwise silently
	// serve plaintext. Supplied via HELIX_TLS_CERT / HELIX_TLS_KEY.
	TLSCertFile string
	TLSKeyFile  string
	// HTTPSPort is the port for the TLS HTTP/2 (TCP) + HTTP/3 (UDP) listeners.
	HTTPSPort string

	// TrustTLSProxy declares that this control-plane process is deployed BEHIND
	// a trusted TLS-terminating reverse proxy (nginx / cloud load balancer) that
	// receives HTTPS from the real client and forwards PLAIN HTTP to this
	// process. In that topology TLSCertFile/TLSKeyFile are intentionally unset
	// (the app never terminates TLS itself), so the TLS-only response headers
	// (HSTS + the SPA CSP's upgrade-insecure-requests) would otherwise never be
	// emitted even though the client-facing connection genuinely is HTTPS
	// (docs/research/security_headers_adversarial_review_20260710/ finding I1).
	//
	// Setting this to true is an explicit operator assertion — it does NOT
	// inspect any request header (X-Forwarded-Proto or otherwise); trusting a
	// client-supplied header unconditionally would let a client spoof HSTS
	// emission over an actually-plaintext hop. The operator is the one who
	// knows the real deployment topology, so the trust is expressed as a
	// boolean config flag, never inferred from request data.
	//
	// Default is false (OFF), which preserves the pre-existing behavior
	// byte-for-byte: with TrustTLSProxy unset, HSTS/upgrade-insecure-requests
	// gating depends solely on TLSCertFile/TLSKeyFile exactly as before this
	// field was added. Supplied via HELIX_TRUST_TLS_PROXY (any of "1", "true",
	// "TRUE", "t" is truthy per strconv.ParseBool; unset/empty/anything else is
	// false).
	TrustTLSProxy bool

	// DatabaseURL, when set (HELIX_DATABASE_URL), switches the control plane onto
	// the pgx/PostgreSQL Repository + rollout StoragePort (the production target,
	// architecture.md §4). Unset = the in-memory implementations (dev/MVP default).
	DatabaseURL string

	// ArtifactUploadTimeout bounds the total time a single artifact upload handler
	// may spend processing (multipart parse, signature validation, hash verify,
	// storage write). Default: DefaultArtifactUploadTimeout (5 min).
	ArtifactUploadTimeout time.Duration
	// RolloutEvaluationTimeout bounds the rollout evaluation handler's maximum
	// execution time. Default: DefaultRolloutEvaluationTimeout (30s).
	RolloutEvaluationTimeout time.Duration
	// DBConnectionPoolSize overrides the pgx pool max connection count. 0 means
	// use pgxpool defaults (max(4, runtime.NumCPU())). Supplied via
	// HELIX_DB_CONNECTION_POOL_SIZE. Default: DefaultDBConnectionPoolSize (0).
	DBConnectionPoolSize int
}

// Load builds a Config from the process environment, applying defaults for any
// unset value. It returns an error only for values that are present but
// malformed (so a misconfiguration fails fast rather than silently degrading).
func Load() (Config, error) {
	c := Config{
		Port:            getEnv("HELIX_PORT", DefaultPort),
		APIBasePath:     getEnv("HELIX_API_BASE_PATH", DefaultAPIBasePath),
		PollInterval:    DefaultPollInterval,
		PollJitter:      DefaultPollJitter,
		AccessTokenTTL:  DefaultAccessTokenTTL,
		DeviceTokenTTL:  DefaultDeviceTokenTTL,
		MaxUploadBytes:  DefaultMaxUploadBytes,
		ArtifactBaseURL: getEnv("HELIX_ARTIFACT_BASE_URL", DefaultArtifactBaseURL),
		TLSCertFile:     os.Getenv("HELIX_TLS_CERT"),
		TLSKeyFile:      os.Getenv("HELIX_TLS_KEY"),
		HTTPSPort:       getEnv("HELIX_HTTPS_PORT", DefaultHTTPSPort),
		DatabaseURL:     os.Getenv("HELIX_DATABASE_URL"),
	}

	var err error
	if c.TrustTLSProxy, err = getBool("HELIX_TRUST_TLS_PROXY", false); err != nil {
		return Config{}, err
	}
	if c.PollInterval, err = getDuration("HELIX_POLL_INTERVAL", DefaultPollInterval); err != nil {
		return Config{}, err
	}
	if c.PollInterval < 0 {
		return Config{}, fmt.Errorf("config: HELIX_POLL_INTERVAL must not be negative, got %s", c.PollInterval)
	}
	if c.PollJitter, err = getDuration("HELIX_POLL_JITTER", DefaultPollJitter); err != nil {
		return Config{}, err
	}
	if c.PollJitter < 0 {
		return Config{}, fmt.Errorf("config: HELIX_POLL_JITTER must not be negative, got %s", c.PollJitter)
	}
	if c.AccessTokenTTL, err = getDuration("HELIX_ACCESS_TOKEN_TTL", DefaultAccessTokenTTL); err != nil {
		return Config{}, err
	}
	if c.AccessTokenTTL < 0 {
		return Config{}, fmt.Errorf("config: HELIX_ACCESS_TOKEN_TTL must not be negative, got %s", c.AccessTokenTTL)
	}
	if c.DeviceTokenTTL, err = getDuration("HELIX_DEVICE_TOKEN_TTL", DefaultDeviceTokenTTL); err != nil {
		return Config{}, err
	}
	if c.DeviceTokenTTL < 0 {
		return Config{}, fmt.Errorf("config: HELIX_DEVICE_TOKEN_TTL must not be negative, got %s", c.DeviceTokenTTL)
	}
	if c.MaxInflight, err = getInt64("HELIX_MAX_INFLIGHT", DefaultMaxInflight); err != nil {
		return Config{}, err
	}
	if c.MaxInflight < 0 {
		return Config{}, fmt.Errorf("config: HELIX_MAX_INFLIGHT must not be negative, got %d", c.MaxInflight)
	}
	if c.RolloutPollInterval, err = getDuration("HELIX_ROLLOUT_POLL_INTERVAL", DefaultRolloutPollInterval); err != nil {
		return Config{}, err
	}
	if c.RolloutPollInterval < 0 {
		return Config{}, fmt.Errorf("config: HELIX_ROLLOUT_POLL_INTERVAL must not be negative, got %s", c.RolloutPollInterval)
	}
	if c.SigningKeyRotationInterval, err = getDuration("HELIX_ARTIFACT_SIGNING_KEY_ROTATION_INTERVAL", DefaultSigningKeyRotationInterval); err != nil {
		return Config{}, err
	}
	if c.SigningKeyRotationInterval < 0 {
		return Config{}, fmt.Errorf("config: HELIX_ARTIFACT_SIGNING_KEY_ROTATION_INTERVAL must not be negative, got %s", c.SigningKeyRotationInterval)
	}
	if c.RateLimitRPS, err = getEnvInt("HELIX_RATE_LIMIT_RPS", DefaultRateLimitRPS); err != nil {
		return Config{}, err
	}
	if c.RateLimitRPS < 0 {
		return Config{}, fmt.Errorf("config: HELIX_RATE_LIMIT_RPS must not be negative, got %d", c.RateLimitRPS)
	}
	if c.AuthRateLimit, err = getEnvInt("HELIX_AUTH_RATE_LIMIT", DefaultAuthRateLimit); err != nil {
		return Config{}, err
	}
	if c.AuthRateLimit < 0 {
		return Config{}, fmt.Errorf("config: HELIX_AUTH_RATE_LIMIT must not be negative, got %d", c.AuthRateLimit)
	}
	if c.MaxUploadBytes, err = getInt64("HELIX_MAX_UPLOAD_BYTES", DefaultMaxUploadBytes); err != nil {
		return Config{}, err
	}
	if c.MaxUploadBytes < 0 {
		return Config{}, fmt.Errorf("config: HELIX_MAX_UPLOAD_BYTES must not be negative, got %d", c.MaxUploadBytes)
	}
	if c.ArtifactUploadTimeout, err = getDuration("HELIX_ARTIFACT_UPLOAD_TIMEOUT", DefaultArtifactUploadTimeout); err != nil {
		return Config{}, err
	}
	if c.ArtifactUploadTimeout < 0 {
		return Config{}, fmt.Errorf("config: HELIX_ARTIFACT_UPLOAD_TIMEOUT must not be negative, got %s", c.ArtifactUploadTimeout)
	}
	if c.RolloutEvaluationTimeout, err = getDuration("HELIX_ROLLOUT_EVALUATION_TIMEOUT", DefaultRolloutEvaluationTimeout); err != nil {
		return Config{}, err
	}
	if c.RolloutEvaluationTimeout < 0 {
		return Config{}, fmt.Errorf("config: HELIX_ROLLOUT_EVALUATION_TIMEOUT must not be negative, got %s", c.RolloutEvaluationTimeout)
	}
	if c.DBConnectionPoolSize, err = getEnvInt("HELIX_DB_CONNECTION_POOL_SIZE", DefaultDBConnectionPoolSize); err != nil {
		return Config{}, err
	}
	if c.DBConnectionPoolSize < 0 {
		return Config{}, fmt.Errorf("config: HELIX_DB_CONNECTION_POOL_SIZE must not be negative, got %d", c.DBConnectionPoolSize)
	}

	// SRV-NEW-4 / OTA-065: TLS cert+key form a PAIR — either BOTH are configured
	// (the control plane terminates TLS: HTTP/3 + HTTP/2 on HTTPSPort per ADR-0004)
	// or NEITHER is (plain HTTP on Port, the documented development default). A
	// HALF-configured pair (exactly one of HELIX_TLS_CERT / HELIX_TLS_KEY set) is a
	// security-relevant misconfiguration: main.go's TLS path is gated on BOTH being
	// non-empty (an && guard), so with only one set the server SILENTLY serves plain
	// HTTP on Port while the operator believes TLS is terminated — a downgrade that a
	// deployment can carry into production undetected. We FAIL-CLOSED (matching the
	// SEC-1 token-secret pattern below) rather than silently downgrade: the operator
	// MUST set both env vars, or neither.
	certSet, keySet := c.TLSCertFile != "", c.TLSKeyFile != ""
	if certSet != keySet {
		present, missing := "HELIX_TLS_CERT", "HELIX_TLS_KEY"
		if keySet {
			present, missing = "HELIX_TLS_KEY", "HELIX_TLS_CERT"
		}
		return Config{}, fmt.Errorf("config: %s is set but %s is not — the TLS cert and key must be "+
			"configured together (set BOTH to terminate TLS, or NEITHER to serve plain HTTP); refusing "+
			"to start with a half-configured TLS pair that would silently serve plaintext", present, missing)
	}

	// SEC-1: Token secret is env-supplied and MUST be set in any real
	// deployment — it is the symmetric key that signs AND verifies every bearer
	// token this server mints (internal/api NewTokenSigner). A hard-coded
	// default would let anyone who can read the source forge valid auth tokens,
	// so we FAIL-CLOSED: when HELIX_TOKEN_SECRET is unset/empty we REFUSE to
	// start, UNLESS the operator explicitly opts into the insecure development
	// secret via HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET (truthy), in which case
	// we use the dev fallback AND log a loud warning. The previous behaviour
	// silently substituted the publicly-known dev secret, so a production deploy
	// that merely forgot the env signed tokens with a forgeable key. The secret
	// value itself is NEVER logged.
	if secret := os.Getenv("HELIX_TOKEN_SECRET"); secret != "" {
		c.TokenSecret = []byte(secret)
	} else {
		allowInsecure, aerr := getBool("HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET", false)
		if aerr != nil {
			return Config{}, aerr
		}
		if !allowInsecure {
			return Config{}, fmt.Errorf("config: HELIX_TOKEN_SECRET is required (it signs and verifies auth " +
				"tokens); refusing to start with the insecure hard-coded development secret — set " +
				"HELIX_TOKEN_SECRET to a strong random value, or set HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET=1 " +
				"for LOCAL DEVELOPMENT ONLY")
		}
		log.Println("config: WARNING — HELIX_TOKEN_SECRET is unset; using the INSECURE hard-coded development " +
			"token secret because HELIX_ALLOW_INSECURE_DEV_TOKEN_SECRET is enabled. Auth tokens are FORGEABLE " +
			"by anyone with source access — NEVER use this in production; set HELIX_TOKEN_SECRET.")
		c.TokenSecret = []byte("helix-ota-dev-token-secret-change-me")
	}

	// Trusted artifact public key (base64). Optional at config-load time.
	if raw := os.Getenv("HELIX_ARTIFACT_PUBKEY"); raw != "" {
		key, decErr := base64.StdEncoding.DecodeString(raw)
		if decErr != nil {
			return Config{}, fmt.Errorf("config: HELIX_ARTIFACT_PUBKEY is not valid base64: %w", decErr)
		}
		c.ArtifactPublicKey = key
	}

	// Previous trusted artifact public key for rotation grace period (T043).
	if raw := os.Getenv("HELIX_ARTIFACT_PREVIOUS_PUBKEY"); raw != "" {
		key, decErr := base64.StdEncoding.DecodeString(raw)
		if decErr != nil {
			return Config{}, fmt.Errorf("config: HELIX_ARTIFACT_PREVIOUS_PUBKEY is not valid base64: %w", decErr)
		}
		c.PreviousArtifactPublicKey = key
	}

	return c, nil
}

// getEnv returns the env var value or fallback when unset/empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getDuration parses a Go duration env var, returning fallback when unset.
func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a valid duration: %w", key, err)
	}
	return d, nil
}

// getInt64 parses an int64 env var, returning fallback when unset.
func getInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a valid integer: %w", key, err)
	}
	return n, nil
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a valid integer: %w", key, err)
	}
	return n, nil
}

// getBool parses a boolean env var (strconv.ParseBool: "1"/"t"/"T"/"TRUE"/
// "true"/"True" and their "0"/"f"/... false counterparts), returning fallback
// when unset/empty.
func getBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s is not a valid boolean: %w", key, err)
	}
	return b, nil
}
