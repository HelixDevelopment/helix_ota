// Package device -- client.go: HTTP client for the Helix OTA control-plane API.
//
// This client is the in-guest OTA agent's protocol seam. It speaks the
// canonical ota-protocol wire types over HTTP against the server endpoints:
//
//   - POST  /auth/login           -> operator bearer token (registration
//     prerequisite)
//   - POST  /devices/register     -> device-scoped bearer token + device ID
//   - GET   /client/update        -> UpdateAvailable (or 204 no-content)
//   - POST  /client/telemetry     -> TelemetryAck (202)
//
// The client reuses the existing ota-protocol types (SS11.4.74 reuse decision)
// and mirrors the deviceemu/emulator.go contract (emulator.go:9-12).
//
// Credentials (SS11.4.10):
//   - The operator login secret is supplied via config, NEVER from untrusted
//     input.
//   - Tokens are held in memory only, never logged.
//   - The device-scoped token is re-issued on registration and cached.
package device

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ApplyPortClient communicates with the Helix OTA control-plane server.
type ApplyPortClient struct {
	baseURL    string
	httpClient *http.Client

	// Authentication state.
	operatorToken string // obtained from POST /auth/login
	deviceID      string // obtained from POST /devices/register
	deviceToken   string // device-scoped bearer token

	// Config.
	insecureSkipVerify bool // allow self-signed TLS in dev
	timeout            time.Duration
}

// NewApplyPortClient creates a new client pointing at the given server base URL.
func NewApplyPortClient(baseURL string, opts ...ClientOption) *ApplyPortClient {
	c := &ApplyPortClient{
		baseURL:            baseURL,
		insecureSkipVerify: false,
		timeout:            30 * time.Second,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ClientOption configures the ApplyPortClient.
type ClientOption func(*ApplyPortClient)

// WithInsecureSkipVerify disables TLS certificate verification for development.
func WithInsecureSkipVerify() ClientOption {
	return func(c *ApplyPortClient) {
		c.insecureSkipVerify = true
		// Secure-by-default: the zero-value client (NewApplyPortClient) sets no
		// custom transport, so certificates ARE verified in production. This
		// insecure transport is reached ONLY when a caller explicitly opts in
		// via WithInsecureSkipVerify() — the documented dev-only path for
		// self-signed servers. MinVersion pins the floor to TLS 1.2.
		c.httpClient.Transport = &http.Transport{
			// nosemgrep: problem-based-packs.insecure-transport.go-stdlib.bypass-tls-verification.bypass-tls-verification -- InsecureSkipVerify is dev-only, gated behind the explicit WithInsecureSkipVerify() opt-in option; default NewApplyPortClient verifies certs. MinVersion pins TLS 1.2.
			TLSClientConfig: &tls.Config{ //nolint:gosec
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		}
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *ApplyPortClient) {
		c.timeout = d
		c.httpClient.Timeout = d
	}
}

// --------------------------------------------------------------------------
// Auth
// --------------------------------------------------------------------------

// LoginRequest is the wire format for operator login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the wire format for the login response.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// Login authenticates as an operator and caches the bearer token.
func (c *ApplyPortClient) Login(ctx context.Context, username, password string) error {
	body := LoginRequest{Username: username, Password: password}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("client: marshal login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/auth/login", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("client: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("client: login failed (HTTP %d): %s",
			resp.StatusCode, string(bodyBytes))
	}

	var lr LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("client: decode login response: %w", err)
	}

	c.operatorToken = lr.AccessToken
	return nil
}

// --------------------------------------------------------------------------
// Registration
// --------------------------------------------------------------------------

// DeviceRegistrationRequest is the wire format for device registration.
// It mirrors the server's DeviceRegistration wire struct
// (internal/api/wire.go) exactly. The OS field's wire key is "os" -- NOT
// "os_type" -- per the OpenAPI schema (endpoints.md) and the server's own
// strict decode discipline (internal/api/bind.go:bindJSON calls
// json.Decoder.DisallowUnknownFields(), so any drift here makes the server
// reject the request outright with "json: unknown field").
type DeviceRegistrationRequest struct {
	HardwareID     string `json:"hardware_id"`
	Model          string `json:"model"`
	OSType         string `json:"os"`
	OSVersion      string `json:"os_version,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
	Group          string `json:"group,omitempty"`
}

// DeviceRegistrationResponse is the wire format for the registration response.
// It mirrors otaprotocol.DeviceRegistrationResponse.
type DeviceRegistrationResponse struct {
	DeviceID    string `json:"device_id"`
	HardwareID  string `json:"hardware_id"`
	DeviceToken string `json:"device_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Register provisions the device and obtains a device-scoped bearer token.
// Requires Login() to have been called first.
func (c *ApplyPortClient) Register(ctx context.Context, req DeviceRegistrationRequest) (*DeviceRegistrationResponse, error) {
	if c.operatorToken == "" {
		return nil, fmt.Errorf("client: must call Login() before Register()")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal register: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/devices/register", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("client: create register request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.operatorToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: register failed (HTTP %d): %s",
			resp.StatusCode, string(bodyBytes))
	}

	var dr DeviceRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, fmt.Errorf("client: decode register response: %w", err)
	}

	c.deviceID = dr.DeviceID
	c.deviceToken = dr.DeviceToken
	return &dr, nil
}

// --------------------------------------------------------------------------
// Update check
// --------------------------------------------------------------------------

// UpdateCheckResult holds the parsed update check response.
type UpdateCheckResult struct {
	Available bool
	// Set when Available == true.
	ReleaseID    string
	DeploymentID string
	Version      string
	URL          string
	Offset       int64
	Size         int64
	SHA256       string
	Signature    string
}

// CheckForUpdate polls the server for an available update.
// Returns (false, nil) when no update is available (HTTP 204).
func (c *ApplyPortClient) CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateCheckResult, error) {
	u, err := url.Parse(c.baseURL + "/client/update")
	if err != nil {
		return nil, fmt.Errorf("client: parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("current_version", currentVersion)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("client: create check request: %w", err)
	}
	if c.deviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.deviceToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: check-for-update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return &UpdateCheckResult{Available: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: check-for-update failed (HTTP %d): %s",
			resp.StatusCode, string(bodyBytes))
	}

	// Parse the response into the ota-protocol-like shape.
	var raw struct {
		ReleaseID    string `json:"release_id"`
		DeploymentID string `json:"deployment_id,omitempty"`
		Version      string `json:"version"`
		URL          string `json:"url"`
		Offset       int64  `json:"offset"`
		Size         int64  `json:"size"`
		SHA256       string `json:"sha256"`
		Signature    string `json:"signature"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("client: decode update response: %w", err)
	}

	return &UpdateCheckResult{
		Available:    true,
		ReleaseID:    raw.ReleaseID,
		DeploymentID: raw.DeploymentID,
		Version:      raw.Version,
		URL:          raw.URL,
		Offset:       raw.Offset,
		Size:         raw.Size,
		SHA256:       raw.SHA256,
		Signature:    raw.Signature,
	}, nil
}

// DownloadBundle downloads the update payload from the given URL.
// Returns the raw bytes. Includes Range-based offset/size support.
func (c *ApplyPortClient) DownloadBundle(ctx context.Context, url string, offset, size int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client: create download request: %w", err)
	}

	// Range header for resumable partial downloads.
	if offset > 0 || size > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size-1))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: download bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: download bundle failed (HTTP %d): %s",
			resp.StatusCode, string(bodyBytes))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("client: read bundle body: %w", err)
	}

	return data, nil
}

// --------------------------------------------------------------------------
// Telemetry
// --------------------------------------------------------------------------

// TelemetryEvent is a single lifecycle event for telemetry reporting.
// Timestamp is Unix seconds for caller convenience (e.g.
// cmd/applyport/main.go: Timestamp: time.Now().Unix()); see MarshalJSON for
// the actual wire encoding.
type TelemetryEvent struct {
	Event     string `json:"event"`
	Version   string `json:"version,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// MarshalJSON emits Timestamp as an RFC 3339 string, matching the server's
// wire contract (internal/api/wire.go: TelemetryEventWire.Timestamp
// time.Time). encoding/json's default time.Time codec REQUIRES a quoted
// RFC 3339 string; a bare Unix-seconds number (what the zero-value int64
// field would otherwise marshal to) fails server-side decode with
// "Time.UnmarshalJSON: input is not a JSON string" (internal/api's
// bindJSON -- see wireformat_telemetry_test.go for the captured proof), so
// every real telemetry report from cmd/applyport/main.go would be rejected
// without this. The public Timestamp field stays an int64 (Unix seconds) so
// existing callers are unaffected -- only the wire encoding changes.
func (e TelemetryEvent) MarshalJSON() ([]byte, error) {
	type wire struct {
		Event     string    `json:"event"`
		Version   string    `json:"version,omitempty"`
		ErrorCode string    `json:"error_code,omitempty"`
		Detail    string    `json:"detail,omitempty"`
		Timestamp time.Time `json:"timestamp"`
	}
	return json.Marshal(wire{
		Event:     e.Event,
		Version:   e.Version,
		ErrorCode: e.ErrorCode,
		Detail:    e.Detail,
		Timestamp: time.Unix(e.Timestamp, 0).UTC(),
	})
}

// UnmarshalJSON is the symmetric counterpart to MarshalJSON: it accepts the
// wire format (RFC 3339 string) and converts it back to Unix seconds so the
// public Timestamp field's Go type (int64) round-trips exactly.
func (e *TelemetryEvent) UnmarshalJSON(data []byte) error {
	type wire struct {
		Event     string    `json:"event"`
		Version   string    `json:"version,omitempty"`
		ErrorCode string    `json:"error_code,omitempty"`
		Detail    string    `json:"detail,omitempty"`
		Timestamp time.Time `json:"timestamp"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	e.Event = w.Event
	e.Version = w.Version
	e.ErrorCode = w.ErrorCode
	e.Detail = w.Detail
	e.Timestamp = w.Timestamp.Unix()
	return nil
}

// TelemetryReport is the wire format for the telemetry POST body.
type TelemetryReport struct {
	DeviceID     string           `json:"device_id"`
	DeploymentID string           `json:"deployment_id"`
	Events       []TelemetryEvent `json:"events"`
	Health       *TelemetryHealth `json:"health,omitempty"`
}

// TelemetryHealth carries the device's optional health block.
type TelemetryHealth struct {
	ActiveSlot    string `json:"active_slot,omitempty"`
	BatteryPct    int    `json:"battery_pct,omitempty"`
	StorageFreeMB int64  `json:"storage_free_mb,omitempty"`
}

// TelemetryAck is the server's 202 acceptance response.
type TelemetryAck struct {
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected"`
	RequestID string `json:"request_id"`
}

// ReportTelemetry sends a telemetry report (POST /client/telemetry).
// Returns the acknowledgment or an error.
func (c *ApplyPortClient) ReportTelemetry(ctx context.Context, report TelemetryReport) (*TelemetryAck, error) {
	data, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("client: marshal telemetry: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/client/telemetry", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("client: create telemetry request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.deviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.deviceToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: telemetry request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: telemetry rejected (HTTP %d): %s",
			resp.StatusCode, string(bodyBytes))
	}

	var ack TelemetryAck
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return nil, fmt.Errorf("client: decode telemetry ack: %w", err)
	}

	return &ack, nil
}

// DeviceID returns the cached device ID from the most recent registration.
func (c *ApplyPortClient) DeviceID() string {
	return c.deviceID
}

// DeviceToken returns the cached device token.
func (c *ApplyPortClient) DeviceToken() string {
	return c.deviceToken
}
