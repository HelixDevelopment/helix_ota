package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RBAC roles (endpoints.md §4.2). RoleSuperAdmin is the global "sees/controls
// everything" identity flag per the Accounts design §3.4 — it bypasses the
// per-account tenant predicate. Set only via config/bootstrap, never a request.
const (
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleViewer     = "viewer"
	RoleDevice     = "device"
	RoleSuperAdmin = "super_admin"
)

// errInvalidToken is returned for any malformed/expired/tampered token.
var errInvalidToken = errors.New("api: invalid token")

// Claims is the subject + roles + expiry carried by a token. This MVP models a
// signed-opaque token (a JWT-shaped, HMAC-signed claims blob) rather than
// re-implementing full OAuth2/JWT — the `auth`/`security` catalogue bricks
// provide the production signer (endpoints.md §4; architecture.md §8). The shape
// (sub, roles, exp, iat) is JWT-forward so the brick drops in without changing
// callers.
//
// AccountID is the per-claim account scope added by Accounts M2 (design §3.2).
// It is SERVER-MINTED (at account-select/device-register time), SERVER-VERIFIED
// (HMAC-SHA256, secret from config only), and the trust boundary is identical to
// `resolvePublicKey`: the claim is NEVER derived from a header/body/request field.
// A token with no account_id (legacy/pre-M2, or a fresh sign-in that has not yet
// selected an account) parses as the empty string — the auth layer treats an empty
// AccountID as unscoped and the account-scoped middleware (requireAccountAccess)
// denies it on every account-scoped route (fail-closed per design §3.3/J).
type Claims struct {
	Subject   string   `json:"sub"`
	Roles     []string `json:"roles"`
	AccountID string   `json:"account_id,omitempty"`
	IssuedAt  int64    `json:"iat"`
	Expiry    int64    `json:"exp"`
}

// HasRole reports whether the claims carry the given role.
func (c Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// TokenSigner mints and verifies the signed-opaque tokens. It is the seam the
// production `security` brick replaces; the MVP uses HMAC-SHA256 over the
// base64url claims, matching a JWT's "<payload>.<sig>" layout in spirit.
type TokenSigner struct {
	secret []byte
}

// NewTokenSigner builds a signer over the configured secret.
func NewTokenSigner(secret []byte) *TokenSigner {
	return &TokenSigner{secret: secret}
}

// Mint issues a signed token for the subject/roles with the given TTL. This is
// the legacy (pre-M2) signature — tokens minted through this path carry an empty
// AccountID (unscoped). Callers that need an account-scoped token should use
// MintAccount instead.
func (s *TokenSigner) Mint(subject string, roles []string, ttl time.Duration, now time.Time) (string, error) {
	return s.MintAccount(subject, roles, "" /* accountID */, ttl, now)
}

// MintAccount issues a signed, account-scoped token. When accountID is non-empty
// the token carries the account claim that the auth middleware extracts and
// requireAccountAccess enforces. An empty accountID produces an unscoped (legacy)
// token — the account-scoped middleware will deny it on every account-scoped route
// (fail-closed per design §3.3/J).
func (s *TokenSigner) MintAccount(subject string, roles []string, accountID string, ttl time.Duration, now time.Time) (string, error) {
	claims := Claims{
		Subject:   subject,
		Roles:     roles,
		AccountID: accountID,
		IssuedAt:  now.Unix(),
		Expiry:    now.Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("api: marshal claims: %w", err)
	}
	encPayload := base64.RawURLEncoding.EncodeToString(payload)
	sig := s.sign(encPayload)
	return encPayload + "." + sig, nil
}

// Verify parses and verifies a token, returning its claims. It rejects a bad
// signature, malformed structure, or an expired token (relative to now).
func (s *TokenSigner) Verify(token string, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errInvalidToken
	}
	encPayload, sig := parts[0], parts[1]
	expected := s.sign(encPayload)
	// Constant-time signature comparison.
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return Claims{}, errInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, errInvalidToken
	}
	if claims.Expiry != 0 && now.Unix() >= claims.Expiry {
		return Claims{}, errInvalidToken
	}
	return claims, nil
}

// sign computes the base64url HMAC-SHA256 of the encoded payload.
func (s *TokenSigner) sign(encPayload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(encPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
