package api

import (
	"net/http"
	"testing"
)

func TestLogin(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name     string
		body     LoginRequest
		wantCode int
		wantErr  string
	}{
		{"valid admin", LoginRequest{Username: "admin@helix.test", Password: "s3cret"}, http.StatusOK, ""},
		{"bad password", LoginRequest{Username: "admin@helix.test", Password: "wrong"}, http.StatusUnauthorized, CodeUnauthenticated},
		{"unknown user", LoginRequest{Username: "nobody@helix.test", Password: "s3cret"}, http.StatusUnauthorized, CodeUnauthenticated},
		{"missing fields", LoginRequest{}, http.StatusBadRequest, CodeValidationFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", tc.body)
			if w.Code != tc.wantCode {
				t.Fatalf("want %d, got %d (%s)", tc.wantCode, w.Code, w.Body.String())
			}
			if tc.wantErr != "" {
				if got := env.errCode(w); got != tc.wantErr {
					t.Fatalf("want error code %s, got %s", tc.wantErr, got)
				}
				return
			}
			var resp TokenResponse
			env.decode(w, &resp)
			if resp.AccessToken == "" || resp.RefreshToken == "" {
				t.Fatalf("expected non-empty token pair, got %+v", resp)
			}
			if resp.TokenType != "Bearer" {
				t.Fatalf("token_type want Bearer, got %q", resp.TokenType)
			}
		})
	}
}

func TestRefreshRotation(t *testing.T) {
	env := newTestEnv(t)

	// Login to obtain a refresh token.
	w := env.doJSON(http.MethodPost, "/api/v1/auth/login", "", LoginRequest{Username: "admin@helix.test", Password: "s3cret"})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var login TokenResponse
	env.decode(w, &login)

	// First refresh succeeds.
	w = env.doJSON(http.MethodPost, "/api/v1/auth/refresh", "", RefreshRequest{RefreshToken: login.RefreshToken})
	if w.Code != http.StatusOK {
		t.Fatalf("first refresh want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var refreshed TokenResponse
	env.decode(w, &refreshed)
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("refresh token should rotate")
	}

	// Reusing the old (rotated) refresh token must fail (single-use).
	w = env.doJSON(http.MethodPost, "/api/v1/auth/refresh", "", RefreshRequest{RefreshToken: login.RefreshToken})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh token want 401, got %d", w.Code)
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name   string
		token  string
		expect int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"garbage token", "not-a-real-token", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(http.MethodPost, "/api/v1/releases", tc.token, []byte(`{}`), "application/json")
			if w.Code != tc.expect {
				t.Fatalf("want %d, got %d", tc.expect, w.Code)
			}
		})
	}
}

func TestRBACForbidsWrongRole(t *testing.T) {
	env := newTestEnv(t)
	// A device token must not be able to create a release (operator-only).
	deviceTok := env.deviceToken("id-device")
	w := env.doJSON(http.MethodPost, "/api/v1/releases", deviceTok, ReleaseCreate{
		ArtifactID: "x", Version: "1.0.0", OS: "android", TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("device creating release: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeForbidden {
		t.Fatalf("want FORBIDDEN, got %s", got)
	}
}
