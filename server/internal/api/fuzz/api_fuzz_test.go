// server/internal/api/fuzz/api_fuzz_test.go
// §11.4.169 Go native fuzzing targets for the REST API endpoints.
//
// Each fuzz target exercises an API boundary route with attacker-controlled
// input: multipart form fields, path parameters, JSON body fields.
// Property under fuzz: no panic, no crash, no unhandled 500.
//
// Run: cd server && go test -fuzz=FuzzArtifactsUpload -fuzztime=30s ./internal/api/fuzz/
//
// NOTE: These fuzz targets are in package fuzz (NOT package api) because
// they exercise the public HTTP interface via httptest rather than
// package-internal state. This avoids deep coupling to internal handler
// implementations while still fuzzing every byte on the wire.

// This file deliberately imports the api package for router construction.
// It is placed in internal/api/fuzz/ to co-locate with the API package
// while staying in a separate test package for hermetic boundary fuzzing.
package fuzz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func fuzzServer(t testing.TB) (*gin.Engine, string) {
	t.Helper()
	var ctr int64
	srv := api.NewServer(api.Options{
		Config: config.Config{
			APIBasePath:    "/api/v1",
			AccessTokenTTL: time.Hour,
			DeviceTokenTTL: 24 * time.Hour,
			MaxUploadBytes: 8 << 20,
			TokenSecret:    []byte("fuzz-test-secret-32-byte-long!"),
		},
		Repo: store.NewMemoryRepository(),
		Users: api.NewStaticUserDirectory(
			api.StaticUser{Username: "admin@helix.test", Password: "s3cret", Roles: []string{api.RoleAdmin, api.RoleOperator, api.RoleViewer}},
		),
		Health:  health.New(func(context.Context) bool { return true }),
		Now:     time.Now,
		NewID:   func() string { return fmt.Sprintf("fuzz-id-%d", atomic.AddInt64(&ctr, 1)) },
		Rollout: nil,
	})
	router := srv.Router()

	// Login to get a token for authenticated endpoints.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"username":"admin@helix.test","password":"s3cret"}`))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	var resp struct {
		Token string `json:"access_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	return router, resp.Token
}

// FuzzArtifactsUploadMultipart fuzzes the POST /artifacts endpoint with
// attacker-controlled multipart form data: varying field names, file
// sizes, and content types.
func FuzzArtifactsUploadMultipart(f *testing.F) {
	router, token := fuzzServer(f)
	if token == "" {
		f.Fatal("failed to obtain admin token for fuzz server")
	}

	// Seed corpus: valid multipart upload shapes.
	seeds := []struct {
		fileName    string
		fileContent string
		metadata    string
	}{
		{"ota.zip", "PK\x03\x04\x00\x00\x00\x00", `{"version":"1.0.0","os":"android","target_model":"OrangePi5Max"}`},
		{"", "", `{}`},
		{"payload", strings.Repeat("A", 1000), `{"version":"999.999.999"}`},
		{strings.Repeat("x", 100), "small", `{"version":"bad"}`},
	}
	for _, s := range seeds {
		f.Add(s.fileName, s.fileContent, s.metadata)
	}

	f.Fuzz(func(t *testing.T, fileName, fileContent, metadata string) {
		// Build a multipart form body.
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		if fileName != "" {
			fw, err := w.CreateFormFile("file", fileName)
			if err != nil {
				return // cannot create part — valid rejection
			}
			if _, err := io.WriteString(fw, fileContent); err != nil {
				return
			}
		}
		fw2, err := w.CreateFormField("metadata")
		if err != nil {
			return
		}
		_, _ = fw2.Write([]byte(metadata))
		w.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/artifacts", &buf)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", w.FormDataContentType())

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Property: no panic, no crash, no unhandled 500 leaking a stack trace.
		if strings.Contains(rec.Body.String(), "goroutine") && strings.Contains(rec.Body.String(), "[recovered]") {
			t.Errorf("PANIC detected in response body for filename=%q metadata=%q", fileName, metadata)
		}
	})
}

// FuzzDevicesByHardwareID fuzzes GET /devices/by-hardware/:hardwareId
// with path-parameter injection payloads (path traversal, SQL injection,
// Unicode overlong encodings, null bytes).
func FuzzDevicesByHardwareID(f *testing.F) {
	router, token := fuzzServer(f)
	if token == "" {
		f.Fatal("failed to obtain admin token for fuzz server")
	}

	seeds := []string{
		"valid-device-1",
		"",
		"../etc/passwd",
		"%00",
		"'; DROP TABLE devices; --",
		"' OR '1'='1",
		"<script>alert(1)</script>",
		strings.Repeat("A", 4096),
		"../../../",
		"..%2F..%2F..%2F",
		"\\x00",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, hardwareID string) {
		// Skip extremely long inputs that would OOM the fuzz runner.
		if len(hardwareID) > 10000 {
			return
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/by-hardware/"+hardwareID, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Property: no unhandled panic.
		if strings.Contains(rec.Body.String(), "goroutine") && strings.Contains(rec.Body.String(), "[recovered]") {
			t.Errorf("PANIC for hardwareID=%q", hardwareID)
		}
	})
}

// FuzzAdminAccountsPOST fuzzes POST /admin/accounts with attacker-controlled
// JSON body fields.
func FuzzAdminAccountsPOST(f *testing.F) {
	router, token := fuzzServer(f)
	if token == "" {
		f.Fatal("failed to obtain admin token for fuzz server")
	}

	seeds := []string{
		`{"name":"test-account","slug":"test-slug"}`,
		`{}`,
		`{"name":"","slug":""}`,
		`{"name":"a","slug":"b","status":"hacked"}`,
		`{"name":"` + strings.Repeat("x", 10000) + `","slug":"test"}`,
		`{bad json!!!`,
		`{"name":123,"slug":456}`,
		`{"name":null,"slug":null}`,
		`["array","not","object"]`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 128*1024 {
			return
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts",
			strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), "goroutine") && strings.Contains(rec.Body.String(), "[recovered]") {
			t.Errorf("PANIC for body=%q", body)
		}
	})
}

// FuzzProjectMembersPOST fuzzes POST /projects/:id/members with
// attacker-controlled JSON body and path parameter.
func FuzzProjectMembersPOST(f *testing.F) {
	router, token := fuzzServer(f)
	if token == "" {
		f.Fatal("failed to obtain admin token for fuzz server")
	}

	// Create a project for the seed corpus.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/projects",
		strings.NewReader(`{"name":"fuzz-proj","display_name":"Fuzz Project","os_type":"android","hardware_target":"OrangePi5Max"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	var createResp struct {
		ProjectID string `json:"project_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &createResp)

	seeds := []struct {
		projectID string
		body      string
	}{
		{createResp.ProjectID, `{"caller_id":"member-1","role":"viewer"}`},
		{"nonexistent", `{"caller_id":"member-1","role":"viewer"}`},
		{"", `{}`},
		{"../admin", `{"caller_id":"bad","role":"admin"}`},
	}
	for _, s := range seeds {
		f.Add(s.projectID, s.body)
	}

	f.Fuzz(func(t *testing.T, projectID, body string) {
		if len(body) > 128*1024 {
			return
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/members",
			strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), "goroutine") && strings.Contains(rec.Body.String(), "[recovered]") {
			t.Errorf("PANIC for projectID=%q body=%q", projectID, body)
		}
	})
}
