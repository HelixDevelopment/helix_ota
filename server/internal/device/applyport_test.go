// Package device — applyport_test.go: RED→GREEN tests for ApplyPort, slot
// manager, signature verification, health marker, and HTTP client.
//
// Each test carries captured physical evidence (§11.4.5/§11.4.69) and the
// suite spans unit, integration (httptest), and edge-case paths.
package device

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// Slot detection tests
// --------------------------------------------------------------------------

func TestSlotDevice_ActiveSlot_DefaultsToA(t *testing.T) {
	t.Parallel()

	// Create a slotDevice with non-existent paths — should fall back to "A".
	s := NewSlotDevice("/nonexistent/cmdline", "/nonexistent/slot_id", "/dev")
	slot, err := s.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "A" {
		t.Fatalf("expected default slot A, got %q", slot)
	}
}

func TestSlotDevice_ActiveSlot_FromProcCmdline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte("root=/dev/vda2 console=ttyS0 helix_slot=B quiet\n"), 0644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	s := NewSlotDevice(cmdline, "/nonexistent", "/dev")
	slot, err := s.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("expected slot B from helix_slot=B, got %q", slot)
	}
}

func TestSlotDevice_ActiveSlot_FromEtcSlotID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	slotID := filepath.Join(dir, "slot_id")
	if err := os.WriteFile(slotID, []byte("B\n"), 0644); err != nil {
		t.Fatalf("write slot_id: %v", err)
	}

	// Provide a cmdline without helix_slot to test the fallback.
	cmdline := filepath.Join(dir, "cmdline2")
	if err := os.WriteFile(cmdline, []byte("root=/dev/vda2 console=ttyS0 quiet\n"), 0644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	s := NewSlotDevice(cmdline, slotID, "/dev")
	slot, err := s.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("expected slot B from /etc/slot_id fallback, got %q", slot)
	}
}

func TestSlotDevice_ActiveSlot_Cached(t *testing.T) {
	t.Parallel()

	// Once cached, changing the underlying file should not affect the slot.
	dir := t.TempDir()
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte("helix_slot=A\n"), 0644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}

	s := NewSlotDevice(cmdline, "/nonexistent", "/dev")

	slot1, _ := s.ActiveSlot()
	if slot1 != "A" {
		t.Fatalf("expected A, got %q", slot1)
	}

	// Change the file — the cache should prevent re-read.
	if err := os.WriteFile(cmdline, []byte("helix_slot=B\n"), 0644); err != nil {
		t.Fatalf("rewrite cmdline: %v", err)
	}

	slot2, _ := s.ActiveSlot()
	if slot2 != "A" {
		t.Fatalf("expected cached A, got %q", slot2)
	}
}

func TestSlotDevice_InactiveSlot(t *testing.T) {
	tests := []struct {
		active   string
		inactive string
	}{
		{"A", "B"},
		{"B", "A"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("active=%s", tc.active), func(t *testing.T) {
			w := NewDDWriter(tc.active)
			inactive, err := w.InactiveSlot()
			if err != nil {
				t.Fatalf("InactiveSlot: %v", err)
			}
			if inactive != tc.inactive {
				t.Fatalf("expected %q, got %q", tc.inactive, inactive)
			}
		})
	}
}

func TestSlotDevice_WriteInactiveSlot_WithDummyWriter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(imagePath, []byte("fake rootfs content"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	w := NewDDWriter("A")
	slot, err := w.WriteInactiveSlot(context.Background(), imagePath)
	if err != nil {
		t.Fatalf("WriteInactiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("expected inactive slot B when active is A, got %q", slot)
	}

	// Verify the writer recorded the path.
	dw := w.(*ddWriter)
	if dw.writtenPath != imagePath {
		t.Fatalf("expected writtenPath=%q, got %q", imagePath, dw.writtenPath)
	}
}

func TestParseHelixSlot(t *testing.T) {
	tests := []struct {
		cmdline string
		want    string
	}{
		{"root=/dev/vda2 helix_slot=A quiet", "A"},
		{"helix_slot=B", "B"},
		{"root=/dev/vda3 console=ttyS0 helix_slot=B hugepages=1024", "B"},
		{"root=/dev/vda2 quiet", ""},
		{"", ""},
		{"helix_slot=", ""},
		{"helix_slot=C", ""},
	}

	for _, tc := range tests {
		got := parseHelixSlot(tc.cmdline)
		if got != tc.want {
			t.Errorf("parseHelixSlot(%q) = %q, want %q", tc.cmdline, got, tc.want)
		}
	}
}

// --------------------------------------------------------------------------
// Signature verification tests (real crypto — no mock)
// --------------------------------------------------------------------------

func TestSignatureVerifier_ValidSignature(t *testing.T) {
	t.Parallel()

	pub, priv, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)
	payload := []byte("test bundle payload for signature verification")

	sigHex := signAndEncode(priv, payload)
	if err := verifier.Verify(payload, sigHex); err != nil {
		t.Fatalf("Verify: expected PASS, got: %v", err)
	}
}

func TestSignatureVerifier_InvalidSignature(t *testing.T) {
	t.Parallel()

	pub, _, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)
	payload := []byte("test payload")

	// Sign with a DIFFERENT key.
	_, otherPriv, _ := generateTestKeypair()
	wrongSig := signAndEncode(otherPriv, payload)

	if err := verifier.Verify(payload, wrongSig); err == nil {
		t.Fatal("Verify: expected FAIL for wrong key, got PASS")
	} else {
		t.Logf("got expected error: %v", err)
	}
}

func TestSignatureVerifier_TamperedPayload(t *testing.T) {
	t.Parallel()

	pub, priv, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)
	payload := []byte("original payload")

	sigHex := signAndEncode(priv, payload)

	// Tamper the payload by appending a byte.
	tampered := append(payload, 0)
	if err := verifier.Verify(tampered, sigHex); err == nil {
		t.Fatal("Verify: expected FAIL for tampered payload, got PASS")
	}
}

func TestSignatureVerifier_NoKeyConfigured(t *testing.T) {
	t.Parallel()

	verifier := NewSignatureVerifier(nil)
	if err := verifier.Verify([]byte("anything"), "hex"); err == nil {
		t.Fatal("Verify: expected error when no key configured")
	}
}

func TestSignatureVerifier_KeyConfigured(t *testing.T) {
	t.Parallel()

	if NewSignatureVerifier(nil).KeyConfigured() {
		t.Fatal("nil key should not be configured")
	}
	if !NewSignatureVerifier(make([]byte, 32)).KeyConfigured() {
		t.Fatal("non-nil key should be configured")
	}
}

func TestSignatureVerifier_BadSignatureEncoding(t *testing.T) {
	t.Parallel()

	pub, _, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	// "-" is not in the standard base64 alphabet (only the URL-safe variant
	// uses it), so this must be rejected as an invalid base64 signature.
	if err := verifier.Verify([]byte("x"), "not-base64"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestSignatureVerifier_BadSignatureLength(t *testing.T) {
	t.Parallel()

	pub, _, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	// Valid base64, but decodes to far fewer than ed25519.SignatureSize bytes.
	shortB64 := "aabb"
	if err := verifier.Verify([]byte("x"), shortB64); err == nil {
		t.Fatal("expected error for short signature")
	}
}

// --------------------------------------------------------------------------
// Signature test helper: sign-and-verify round trip
// --------------------------------------------------------------------------

func TestSignAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	for i := 0; i < 5; i++ {
		pub, priv, _ := generateTestKeypair()
		verifier := NewSignatureVerifier(pub)
		payload := []byte(fmt.Sprintf("round-trip test payload %d", i))

		sig := signAndEncode(priv, payload)
		if err := verifier.Verify(payload, sig); err != nil {
			t.Fatalf("round-trip %d: expected PASS, got: %v", i, err)
		}
	}
}

// --------------------------------------------------------------------------
// HealthMarker tests
// --------------------------------------------------------------------------

// mockEnvManager implements UBootEnvManager for testing.
type mockEnvManager struct {
	store map[string]string
}

func newMockEnvManager() *mockEnvManager {
	return &mockEnvManager{store: make(map[string]string)}
}

func (m *mockEnvManager) SetEnv(key, value string) error {
	m.store[key] = value
	return nil
}

func (m *mockEnvManager) GetEnv(key string) (string, error) {
	return m.store[key], nil
}

func (m *mockEnvManager) SaveEnv() error {
	return nil
}

func TestHealthMarker_ConfirmHealthy(t *testing.T) {
	t.Parallel()

	mock := newMockEnvManager()
	mock.SetEnv("upgrade_available", "1")
	mock.SetEnv("bootcount", "3")

	marker := NewHealthMarker(mock)
	if err := marker.ConfirmHealthy(); err != nil {
		t.Fatalf("ConfirmHealthy: %v", err)
	}

	if v, _ := mock.GetEnv("upgrade_available"); v != "0" {
		t.Fatalf("expected upgrade_available=0, got %q", v)
	}
	if v, _ := mock.GetEnv("bootcount"); v != "0" {
		t.Fatalf("expected bootcount=0, got %q", v)
	}
}

func TestHealthMarker_ConfirmHealthy_Idempotent(t *testing.T) {
	t.Parallel()

	mock := newMockEnvManager()
	marker := NewHealthMarker(mock)

	// First call: nothing armed, should be no-op.
	if err := marker.ConfirmHealthy(); err != nil {
		t.Fatalf("first ConfirmHealthy: %v", err)
	}
	if v, _ := mock.GetEnv("upgrade_available"); v != "" {
		t.Fatalf("expected empty upgrade_available, got %q", v)
	}

	// Second call: still no-op.
	if err := marker.ConfirmHealthy(); err != nil {
		t.Fatalf("second ConfirmHealthy: %v", err)
	}
}

func TestHealthMarker_IsArmed(t *testing.T) {
	t.Parallel()

	mock := newMockEnvManager()

	marker := NewHealthMarker(mock)
	if armed, _ := marker.IsArmed(); armed {
		t.Fatal("expected not armed initially")
	}

	mock.SetEnv("upgrade_available", "1")
	armed, err := marker.IsArmed()
	if err != nil {
		t.Fatalf("IsArmed: %v", err)
	}
	if !armed {
		t.Fatal("expected armed when upgrade_available=1")
	}
}

// --------------------------------------------------------------------------
// ApplyPort WriteAndArm tests
// --------------------------------------------------------------------------

func TestApplyPort_WriteAndArm(t *testing.T) {
	t.Parallel()

	env := newMockEnvManager()
	writer := NewDDWriter("A")
	applier := NewApplyPort(writer, env, "/dev/vda2")

	// Write a plausible rootfs.
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(imagePath, []byte("fake rootfs for arm test\n"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := applier.WriteAndArm(context.Background(), imagePath); err != nil {
		t.Fatalf("WriteAndArm: %v", err)
	}

	// Verify the U-Boot env was set correctly.
	if v, _ := env.GetEnv("BOOT_ORDER"); v != "B A" {
		t.Fatalf("expected BOOT_ORDER='B A', got %q", v)
	}
	if v, _ := env.GetEnv("upgrade_available"); v != "1" {
		t.Fatalf("expected upgrade_available=1, got %q", v)
	}
	if v, _ := env.GetEnv("bootcount"); v != "1" {
		t.Fatalf("expected bootcount=1, got %q", v)
	}
}

func TestApplyPort_WriteAndArm_FromSlotB(t *testing.T) {
	t.Parallel()

	env := newMockEnvManager()
	writer := NewDDWriter("B")
	applier := NewApplyPort(writer, env, "/dev/vda3")

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(imagePath, []byte("fake rootfs\n"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if err := applier.WriteAndArm(context.Background(), imagePath); err != nil {
		t.Fatalf("WriteAndArm: %v", err)
	}

	// When active is B, inactive is A → BOOT_ORDER should be "A B".
	if v, _ := env.GetEnv("BOOT_ORDER"); v != "A B" {
		t.Fatalf("expected BOOT_ORDER='A B', got %q", v)
	}
}

func TestApplyPort_ActiveSlot(t *testing.T) {
	t.Parallel()

	applier := NewApplyPort(NewDDWriter("A"), newMockEnvManager(), "/dev/vda2")
	slot, err := applier.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "A" {
		t.Fatalf("expected A, got %q", slot)
	}
}

// --------------------------------------------------------------------------
// HTTP client tests (httptest server)
// --------------------------------------------------------------------------

func TestApplyPortClient_Login(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/auth/login" {
			t.Errorf("expected /auth/login, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token":"test-token-abc","token_type":"bearer"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	if err := client.Login(context.Background(), "admin", "password"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if client.operatorToken != "test-token-abc" {
		t.Fatalf("expected token, got %q", client.operatorToken)
	}
}

func TestApplyPortClient_Login_Failure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid credentials"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	if err := client.Login(context.Background(), "admin", "wrong"); err == nil {
		t.Fatal("expected login error, got nil")
	}
}

func TestApplyPortClient_Register(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/devices/register" {
			t.Errorf("expected /devices/register, got %s", r.URL.Path)
		}

		// Verify auth header.
		if r.Header.Get("Authorization") != "Bearer operator-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"device_id":"dev-001","hardware_id":"rk3588","device_token":"dev-token-xyz","token_type":"bearer","expires_in":86400}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.operatorToken = "operator-token"

	resp, err := client.Register(context.Background(), DeviceRegistrationRequest{
		HardwareID: "rk3588",
		Model:      "opi5max",
		OSType:     "linux",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.DeviceID != "dev-001" {
		t.Fatalf("expected device_id 'dev-001', got %q", resp.DeviceID)
	}
	if resp.DeviceToken != "dev-token-xyz" {
		t.Fatalf("expected device_token, got %q", resp.DeviceToken)
	}
	if client.deviceID != "dev-001" {
		t.Fatalf("expected cached deviceID 'dev-001', got %q", client.deviceID)
	}
}

func TestApplyPortClient_CheckForUpdate_Available(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/client/update" {
			t.Errorf("expected /client/update, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "current_version=1.0.0" {
			t.Errorf("expected current_version=1.0.0, got %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"release_id":"rel-001","deployment_id":"dep-001","version":"2.0.0","url":"http://example.com/bundle.zip","offset":0,"size":1024,"sha256":"abc123","signature":"deadbeef"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	result, err := client.CheckForUpdate(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if !result.Available {
		t.Fatal("expected update available")
	}
	if result.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %q", result.Version)
	}
}

func TestApplyPortClient_CheckForUpdate_NoContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	result, err := client.CheckForUpdate(context.Background(), "2.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if result.Available {
		t.Fatal("expected no update available")
	}
}

func TestApplyPortClient_DownloadBundle(t *testing.T) {
	t.Parallel()

	payload := []byte("test bundle content for download test")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bundle.zip" {
			t.Errorf("expected /bundle.zip, got %s", r.URL.Path)
		}
		if r.Header.Get("Range") != "bytes=0-31" {
			t.Errorf("expected Range header, got %q", r.Header.Get("Range"))
		}
		w.WriteHeader(http.StatusPartialContent)
		w.Write(payload) //nolint:errcheck
	}))
	defer srv.Close()

	client := NewApplyPortClient("http://example.com")
	data, err := client.DownloadBundle(context.Background(), srv.URL+"/bundle.zip", 0, 32)
	if err != nil {
		t.Fatalf("DownloadBundle: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded content mismatch")
	}
}

func TestApplyPortClient_ReportTelemetry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/client/telemetry" {
			t.Errorf("expected /client/telemetry, got %s", r.URL.Path)
		}

		var body TelemetryReport
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if len(body.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(body.Events))
		}
		if body.Events[0].Event != "download_started" {
			t.Errorf("expected download_started, got %q", body.Events[0].Event)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"accepted":1,"rejected":0,"request_id":"req-001"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	ack, err := client.ReportTelemetry(context.Background(), TelemetryReport{
		DeviceID:     "dev-001",
		DeploymentID: "dep-001",
		Events: []TelemetryEvent{
			{Event: "download_started", Version: "2.0.0", Timestamp: time.Now().Unix()},
		},
	})
	if err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if ack.Accepted != 1 {
		t.Fatalf("expected 1 accepted, got %d", ack.Accepted)
	}
}

func TestApplyPortClient_ReportTelemetry_WithHealth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body TelemetryReport
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		if body.Health == nil {
			t.Errorf("expected health block")
		} else if body.Health.ActiveSlot != "A" {
			t.Errorf("expected active_slot=A, got %q", body.Health.ActiveSlot)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"accepted":1,"rejected":0,"request_id":"req-002"}`)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	ack, err := client.ReportTelemetry(context.Background(), TelemetryReport{
		DeviceID:     "dev-001",
		DeploymentID: "dep-001",
		Events: []TelemetryEvent{
			{Event: "success", Version: "2.0.0", Timestamp: time.Now().Unix()},
		},
		Health: &TelemetryHealth{ActiveSlot: "A", StorageFreeMB: 1024},
	})
	if err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if ack.Accepted != 1 {
		t.Fatalf("expected 1 accepted, got %d", ack.Accepted)
	}
}

func TestApplyPortClient_CheckForUpdate_Error(t *testing.T) {
	t.Parallel()

	// Server returning 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewApplyPortClient(srv.URL)
	client.deviceToken = "dev-token"

	_, err := client.CheckForUpdate(context.Background(), "1.0.0")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestApplyPortClient_DownloadBundle_Error(t *testing.T) {
	t.Parallel()

	client := NewApplyPortClient("http://example.com")
	_, err := client.DownloadBundle(context.Background(), "http://nonexistent.example/bundle.zip", 0, 100)
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

// --------------------------------------------------------------------------
// Integration: full lifecycle with httptest server
// --------------------------------------------------------------------------

func TestFullLifecycle_LoginRegisterCheck(t *testing.T) {
	t.Parallel()

	var registered bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"access_token":"op-token","token_type":"bearer"}`)
		case "/devices/register":
			if !registered {
				registered = true
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"device_id":"dev-42","hardware_id":"rk3588","device_token":"dev-token","token_type":"bearer","expires_in":86400}`)
			} else {
				w.WriteHeader(http.StatusConflict)
			}
		case "/client/update":
			if currentVersion := r.URL.Query().Get("current_version"); currentVersion == "2.0.0" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"release_id":"rel-001","deployment_id":"dep-001","version":"2.0.0","url":"http://artifacts/bundle.zip","offset":0,"size":100,"sha256":"abc","signature":"def"}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client := NewApplyPortClient(srv.URL)

	if err := client.Login(ctx, "admin", "pass"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	regReq := DeviceRegistrationRequest{HardwareID: "rk3588", Model: "opi5max", OSType: "linux"}
	if _, err := client.Register(ctx, regReq); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// First check (1.0.0) should return available.
	result, err := client.CheckForUpdate(ctx, "1.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate (1.0.0): %v", err)
	}
	if !result.Available {
		t.Fatal("expected update available for 1.0.0")
	}

	// Second check (2.0.0) should return no content.
	result, err = client.CheckForUpdate(ctx, "2.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate (2.0.0): %v", err)
	}
	if result.Available {
		t.Fatal("expected no update for 2.0.0 (on target)")
	}
}

// --------------------------------------------------------------------------
// Edge case: empty/boundary inputs
// --------------------------------------------------------------------------

func TestSignatureVerifier_EmptyPayload(t *testing.T) {
	t.Parallel()

	pub, priv, _ := generateTestKeypair()
	verifier := NewSignatureVerifier(pub)

	// Empty payload is valid and should verify if the signature matches.
	sig := signAndEncode(priv, []byte{})
	if err := verifier.Verify([]byte{}, sig); err != nil {
		t.Fatalf("empty payload verify: %v", err)
	}
}

func TestSlotDevice_EmptyCmdline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte(""), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s := NewSlotDevice(cmdline, "/nonexistent", "/dev")
	slot, err := s.ActiveSlot()
	if err != nil {
		t.Fatalf("ActiveSlot: %v", err)
	}
	if slot != "A" {
		t.Fatalf("expected default A for empty cmdline, got %q", slot)
	}
}

// --------------------------------------------------------------------------
// Concurrent slot access (§11.4.85 stress)
// --------------------------------------------------------------------------

func TestSlotDevice_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	s := NewSlotDevice("/nonexistent", "/nonexistent", "/dev")
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, _ = s.ActiveSlot()
				_, _ = s.InactiveSlot()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// --------------------------------------------------------------------------
// Real ed25519 keypair test (anti-bluff — real crypto, no mock)
// --------------------------------------------------------------------------

func TestRealEd25519Signature(t *testing.T) {
	t.Parallel()

	// Generate a real ed25519 keypair.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	verifier := NewSignatureVerifier(pub)

	// Sign a real payload.
	payload := []byte("real firmware bundle abc123")
	digest := sha256.Sum256(payload)
	sig := ed25519.Sign(priv, digest[:])
	// The real wire format is base64 (endpoints.md §12.1), not hex — see
	// signature.go's package doc comment and signature_wireformat_test.go.
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Verify.
	if err := verifier.Verify(payload, sigB64); err != nil {
		t.Fatalf("real ed25519 verify: %v", err)
	}
}

// --------------------------------------------------------------------------
// Ensure exports exist
// --------------------------------------------------------------------------

func TestExports(t *testing.T) {
	t.Parallel()

	// Verify key exported types and functions exist.
	var _ SlotWriter = (*ddWriter)(nil)
	var _ SlotWriter = (*slotDevice)(nil)
	var _ UBootEnvManager = (*mockEnvManager)(nil)
	var _ UBootEnvManager = (*fwEnvManager)(nil)

	_ = NewApplyPort(nil, nil, "")
	_ = NewSlotDevice("", "", "")
	_ = NewDDWriter("A")
	_ = NewFwEnvManager("", "")
	_ = NewSignatureVerifier(nil)
	_ = NewHealthMarker(nil)
	_ = NewApplyPortClient("http://example.com")
	_, _, _ = generateTestKeypair()
}

func TestFwEnvManager_ConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	printenvPath := filepath.Join(dir, "fw_printenv")
	setenvPath := filepath.Join(dir, "fw_setenv")
	envFile := filepath.Join(dir, "env.data")

	// fw_printenv script: reads key from envFile and prints value
	if err := os.WriteFile(printenvPath, []byte(fmt.Sprintf(`#!/bin/sh
val=$(grep "^$1=" %s 2>/dev/null | sed "s/^$1=//")
if [ -n "$val" ]; then
  echo -n "$val"
else
  echo "not set" >&2
  exit 1
fi
`, envFile)), 0755); err != nil {
		t.Fatalf("write printenv: %v", err)
	}
	// fw_setenv script: writes key=value to envFile
	if err := os.WriteFile(setenvPath, []byte(fmt.Sprintf(`#!/bin/sh
if [ $# -ge 2 ]; then
  echo "$1=$2" >> %s
fi
`, envFile)), 0755); err != nil {
		t.Fatalf("write setenv: %v", err)
	}

	mgr := NewFwEnvManager(setenvPath, printenvPath)

	// Set a value
	if err := mgr.SetEnv("BOOT_ORDER", "B A"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	// Read back
	v, err := mgr.GetEnv("BOOT_ORDER")
	if err != nil {
		t.Fatalf("GetEnv: %v", err)
	}
	if v != "B A" {
		t.Fatalf("round-trip failed: expected 'B A', got %q", v)
	}
}
