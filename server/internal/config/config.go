// Package config provides env-based runtime configuration for the Helix OTA
// control-plane server. Per ADR-0003/architecture.md the modular monolith reads
// poll cadence, TTLs, limits, and the API base path from configuration (the
// `config` brick) rather than hard-coding them. No secrets are embedded in code;
// the JWT signing secret and trusted artifact public key are supplied via env.
package config

import (
	"encoding/base64"
	"fmt"
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
	// MaxInflight bounds concurrent in-flight requests (DoS protection); excess
	// requests are shed with 429 RATE_LIMITED. 0 (default) disables the cap.
	MaxInflight int64
	// ArtifactBaseURL is the base of the artifact download reference.
	ArtifactBaseURL string

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

	// TLSCertFile / TLSKeyFile enable the HTTP/3 (QUIC) + HTTP/2 transport. When
	// BOTH are set the control plane is served over HTTP/3 with automatic HTTP/2
	// fallback (ADR-0004) on HTTPSPort; otherwise it serves plain HTTP on Port
	// (development). Supplied via HELIX_TLS_CERT / HELIX_TLS_KEY.
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
	if c.MaxInflight, err = getInt64("HELIX_MAX_INFLIGHT", 0); err != nil {
		return Config{}, err
	}
	if c.MaxInflight < 0 {
		return Config{}, fmt.Errorf("config: HELIX_MAX_INFLIGHT must not be negative, got %d", c.MaxInflight)
	}
	if c.MaxUploadBytes, err = getInt64("HELIX_MAX_UPLOAD_BYTES", DefaultMaxUploadBytes); err != nil {
		return Config{}, err
	}
	if c.MaxUploadBytes < 0 {
		return Config{}, fmt.Errorf("config: HELIX_MAX_UPLOAD_BYTES must not be negative, got %d", c.MaxUploadBytes)
	}

	// Token secret: env-supplied, never a hard-coded secret. A development
	// fallback keeps the binary runnable locally; production MUST set the env.
	if secret := os.Getenv("HELIX_TOKEN_SECRET"); secret != "" {
		c.TokenSecret = []byte(secret)
	} else {
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
