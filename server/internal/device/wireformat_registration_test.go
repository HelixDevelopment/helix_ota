// Package device -- wireformat_registration_test.go: proves the device
// registration request the real ApplyPortClient sends is byte-for-byte
// decodable by the REAL server-side handler (internal/api), not merely by a
// permissive test double.
//
// Forensic root cause (§11.4.108 SOURCE<->RUNTIME contract): the server's
// DeviceRegistration wire struct (internal/api/wire.go) declares the OS
// field with JSON tag `os` (`OS otaprotocol.OSType `json:"os"“), and the
// server decodes every request body with `encoding/json`'s
// DisallowUnknownFields() (internal/api/bind.go:bindJSON) -- an unrecognised
// field is a hard decode error, not a silently-ignored extra. Before the fix
// accompanying this test, internal/device/client.go's
// DeviceRegistrationRequest declared the OS field with JSON tag `os_type`
// instead of `os`. That means the wire body the real applyport CLI
// (cmd/applyport/main.go) sends over the wire never carries an `os` key at
// all -- so the server's bindJSON call fails with "json: unknown field
// \"os_type\"" and handleRegisterDevice responds 400 "malformed device
// registration body" on EVERY SINGLE real registration attempt. The
// existing TestApplyPortClient_Register (applyport_test.go) never caught
// this because its httptest fixture server does not inspect the request
// body at all -- it just asserts method/path/auth-header and returns a
// canned response, so it validates the client can parse a response, never
// that the server can parse the client's request. This test decodes the
// exact bytes ApplyPortClient.Register() puts on the wire using the REAL
// server-side wire struct (internal/api.DeviceRegistration) and the same
// strict decode discipline internal/api/bind.go uses, closing that gap.
package device

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
)

// strictDecode mirrors internal/api/bind.go:bindJSON's decode discipline
// exactly (DisallowUnknownFields + reject trailing data) so this test proves
// the wire body against the REAL server contract, not a relaxed stand-in.
func strictDecode(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// TestApplyPortClient_Register_WireBodyDecodesOnRealServerStruct captures the
// exact JSON bytes ApplyPortClient.Register() sends and decodes them with
// the REAL server-side wire type (internal/api.DeviceRegistration) under the
// server's own strict-decode discipline. If the client's JSON tags drift
// from the server's, this fails exactly the way a live registration attempt
// against the real Helix OTA server would fail.
func TestApplyPortClient_Register_WireBodyDecodesOnRealServerStruct(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"device_id":"dev-001","hardware_id":"rk3588","device_token":"dev-token-xyz","token_type":"bearer","expires_in":86400}`))
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.operatorToken = "operator-token"

	req := DeviceRegistrationRequest{
		HardwareID: "rk3588",
		Model:      "opi5max",
		OSType:     "linux",
	}
	if _, err := client.Register(context.Background(), req); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(capturedBody) == 0 {
		t.Fatal("test bug: server did not capture a request body")
	}

	// Decode with the REAL server wire struct under the REAL server's strict
	// decode discipline (DisallowUnknownFields) -- exactly what
	// internal/api/bind.go:bindJSON does before handleRegisterDevice's
	// req.OS == "" required-field check.
	var wire api.DeviceRegistration
	if err := strictDecode(capturedBody, &wire); err != nil {
		t.Fatalf("the real server's bindJSON would reject this request body: %v "+
			"(body: %s). This proves a live applyport CLI registration attempt "+
			"against the real Helix OTA server fails on every call.", err, string(capturedBody))
	}
	if string(wire.OS) != "linux" {
		t.Fatalf("server-side wire struct OS field = %q, want %q -- the os value "+
			"the client sent never reached the field the server's required-field "+
			"check (\"hardware_id, model, and os are required\") inspects",
			wire.OS, "linux")
	}
	if wire.HardwareID != "rk3588" || wire.Model != "opi5max" {
		t.Fatalf("server-side wire struct = %+v, want HardwareID=rk3588 Model=opi5max", wire)
	}
}
