// Package device — coverage_extra_test.go: real unit tests for the previously
// uncovered fwEnvManager (fw_setenv/fw_printenv shell-out), slotDevice
// WriteInactiveSlot/detect paths (dd shell-out + /proc/cmdline + /etc/slot_id),
// and the HealthMarker error branches.
//
// These exercise the real shell-out code paths by writing stub fw_setenv /
// fw_printenv / dd scripts into a temp dir and pointing the manager at them
// (fwEnvManager) or prepending the temp dir to PATH (dd). Every assertion
// checks real observable behaviour (file contents the stub wrote, returned
// values, error propagation), not coverage padding.
package device

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeStub writes an executable shell script at dir/name and returns its
// absolute path. Skips the whole test on non-POSIX hosts where /bin/sh stubs
// are not meaningful (§11.4.3 topology SKIP, never a fake PASS).
func writeStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-out stub tests require a POSIX shell; not applicable on windows")
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
	return p
}

// ---------------------------------------------------------------------------
// fwEnvManager.resolveSetenv / resolvePrintenv — default vs explicit path.
// ---------------------------------------------------------------------------

func TestFwEnvManager_Resolve(t *testing.T) {
	t.Parallel()

	def := NewFwEnvManager("", "").(*fwEnvManager)
	if got := def.resolveSetenv(); got != "fw_setenv" {
		t.Errorf("resolveSetenv default = %q, want fw_setenv", got)
	}
	if got := def.resolvePrintenv(); got != "fw_printenv" {
		t.Errorf("resolvePrintenv default = %q, want fw_printenv", got)
	}

	exp := NewFwEnvManager("/opt/fw_setenv", "/opt/fw_printenv").(*fwEnvManager)
	if got := exp.resolveSetenv(); got != "/opt/fw_setenv" {
		t.Errorf("resolveSetenv explicit = %q, want /opt/fw_setenv", got)
	}
	if got := exp.resolvePrintenv(); got != "/opt/fw_printenv" {
		t.Errorf("resolvePrintenv explicit = %q, want /opt/fw_printenv", got)
	}
}

// ---------------------------------------------------------------------------
// fwEnvManager.SetEnv — success writes the args, failure propagates stderr.
// ---------------------------------------------------------------------------

func TestFwEnvManager_SetEnv_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	// Stub records its args so we can prove SetEnv passed key+value.
	setenv := writeStub(t, dir, "fw_setenv_stub", `echo "$@" > "`+out+`"; exit 0`)

	m := NewFwEnvManager(setenv, "")
	if err := m.SetEnv("BOOT_ORDER", "B A"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read stub output: %v", err)
	}
	if strings.TrimSpace(string(got)) != "BOOT_ORDER B A" {
		t.Fatalf("stub received %q, want %q", strings.TrimSpace(string(got)), "BOOT_ORDER B A")
	}
}

func TestFwEnvManager_SetEnv_Error(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	setenv := writeStub(t, dir, "fw_setenv_fail", `echo "device not found" >&2; exit 1`)

	m := NewFwEnvManager(setenv, "")
	err := m.SetEnv("BOOT_ORDER", "B A")
	if err == nil {
		t.Fatal("expected error from failing fw_setenv, got nil")
	}
	// The wrapped error must surface both the key=value and the stderr.
	if !strings.Contains(err.Error(), "BOOT_ORDER=B A") {
		t.Errorf("error %q missing key=value", err.Error())
	}
	if !strings.Contains(err.Error(), "device not found") {
		t.Errorf("error %q missing stderr text", err.Error())
	}
}

// ---------------------------------------------------------------------------
// fwEnvManager.GetEnv — value returned (trimmed), unset→empty, hard error.
// ---------------------------------------------------------------------------

func TestFwEnvManager_GetEnv_Value(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Stub echoes a value with trailing whitespace to prove trimming.
	printenv := writeStub(t, dir, "fw_printenv_val", `echo "B A   "; exit 0`)

	m := NewFwEnvManager("", printenv)
	got, err := m.GetEnv("BOOT_ORDER")
	if err != nil {
		t.Fatalf("GetEnv: %v", err)
	}
	if got != "B A" {
		t.Fatalf("GetEnv = %q, want %q (trimmed)", got, "B A")
	}
}

func TestFwEnvManager_GetEnv_UnsetIsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
	}{
		{"not-set", `echo "## Error: \"BOOT_ORDER\" not set" >&2; exit 1`},
		{"not-defined", `echo "variable not defined" >&2; exit 1`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			printenv := writeStub(t, dir, "fw_printenv_unset", tc.stderr)

			m := NewFwEnvManager("", printenv)
			got, err := m.GetEnv("BOOT_ORDER")
			if err != nil {
				t.Fatalf("GetEnv unset must swallow error, got %v", err)
			}
			if got != "" {
				t.Fatalf("GetEnv unset = %q, want empty", got)
			}
		})
	}
}

func TestFwEnvManager_GetEnv_HardError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Non-zero exit whose stderr is NOT an "unset" message → hard error.
	printenv := writeStub(t, dir, "fw_printenv_err", `echo "cannot access env device" >&2; exit 2`)

	m := NewFwEnvManager("", printenv)
	_, err := m.GetEnv("BOOT_ORDER")
	if err == nil {
		t.Fatal("expected hard error from fw_printenv, got nil")
	}
	if !strings.Contains(err.Error(), "cannot access env device") {
		t.Errorf("error %q missing stderr text", err.Error())
	}
}

// ---------------------------------------------------------------------------
// fwEnvManager.SaveEnv — best-effort, returns nil even when the binary fails.
// ---------------------------------------------------------------------------

func TestFwEnvManager_SaveEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Even a failing save returns nil (best-effort flush).
	setenv := writeStub(t, dir, "fw_setenv_save", `exit 1`)

	m := NewFwEnvManager(setenv, "")
	if err := m.SaveEnv(); err != nil {
		t.Fatalf("SaveEnv must be best-effort nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// slotDevice.WriteInactiveSlot — dd shell-out success + error branches.
// ---------------------------------------------------------------------------

// withStubDD prepends a temp dir containing a `dd` stub to PATH for the
// duration of the test and returns the dir (so other stubs can be added too).
func withStubDD(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("dd shell-out test requires a POSIX shell")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dd"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write dd stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestSlotDevice_WriteInactiveSlot_Success(t *testing.T) {
	// NOT parallel: mutates PATH via t.Setenv.
	dir := withStubDD(t, `echo "dd-ran $@" >&2; exit 0`)

	// cmdline says active=A, so inactive=B → partition /dev/vda3.
	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte("helix_slot=A\n"), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}
	img := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(img, []byte("not-empty-image-bytes"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	s := NewSlotDevice(cmdline, "/nonexistent/slot_id", "/dev")
	slot, err := s.WriteInactiveSlot(context.Background(), img)
	if err != nil {
		t.Fatalf("WriteInactiveSlot: %v", err)
	}
	if slot != "B" {
		t.Fatalf("WriteInactiveSlot returned slot %q, want B", slot)
	}
}

func TestSlotDevice_WriteInactiveSlot_MissingImage(t *testing.T) {
	t.Parallel()

	s := NewSlotDevice("/nonexistent/cmdline", "/nonexistent/slot_id", "/dev")
	_, err := s.WriteInactiveSlot(context.Background(), "/no/such/image.img")
	if err == nil {
		t.Fatal("expected error for missing image, got nil")
	}
	if !strings.Contains(err.Error(), "stat image") {
		t.Errorf("error %q should mention stat image", err.Error())
	}
}

func TestSlotDevice_WriteInactiveSlot_EmptyImage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	img := filepath.Join(dir, "empty.img")
	if err := os.WriteFile(img, nil, 0o644); err != nil {
		t.Fatalf("write empty image: %v", err)
	}

	s := NewSlotDevice("/nonexistent/cmdline", "/nonexistent/slot_id", "/dev")
	_, err := s.WriteInactiveSlot(context.Background(), img)
	if err == nil {
		t.Fatal("expected error for empty image, got nil")
	}
	if !strings.Contains(err.Error(), "is empty") {
		t.Errorf("error %q should mention empty image", err.Error())
	}
}

func TestSlotDevice_WriteInactiveSlot_DDFails(t *testing.T) {
	// NOT parallel: mutates PATH.
	dir := withStubDD(t, `echo "no space left on device" >&2; exit 1`)

	cmdline := filepath.Join(dir, "cmdline")
	if err := os.WriteFile(cmdline, []byte("helix_slot=A\n"), 0o644); err != nil {
		t.Fatalf("write cmdline: %v", err)
	}
	img := filepath.Join(dir, "rootfs.img")
	if err := os.WriteFile(img, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	s := NewSlotDevice(cmdline, "/nonexistent/slot_id", "/dev")
	_, err := s.WriteInactiveSlot(context.Background(), img)
	if err == nil {
		t.Fatal("expected error when dd fails, got nil")
	}
	if !strings.Contains(err.Error(), "no space left on device") {
		t.Errorf("error %q should surface dd stderr", err.Error())
	}
}

// ---------------------------------------------------------------------------
// HealthMarker error branches — GetEnv error, SetEnv error, SaveEnv error.
// ---------------------------------------------------------------------------

// errEnvManager is a UBootEnvManager that can be configured to fail on a
// specific operation, exercising HealthMarker's error-wrapping branches.
type errEnvManager struct {
	store      map[string]string
	getErr     error
	setErrOn   string // key that triggers a SetEnv error ("" = never)
	saveErr    error
	setErrText error
}

func newErrEnv() *errEnvManager {
	return &errEnvManager{store: make(map[string]string)}
}

func (m *errEnvManager) GetEnv(key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.store[key], nil
}

func (m *errEnvManager) SetEnv(key, value string) error {
	if m.setErrOn != "" && key == m.setErrOn {
		return m.setErrText
	}
	m.store[key] = value
	return nil
}

func (m *errEnvManager) SaveEnv() error {
	return m.saveErr
}

func TestHealthMarker_ConfirmHealthy_GetError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.getErr = errors.New("env device read failure")
	marker := NewHealthMarker(env)

	err := marker.ConfirmHealthy()
	if err == nil {
		t.Fatal("expected error from GetEnv failure, got nil")
	}
	if !strings.Contains(err.Error(), "read upgrade_available") {
		t.Errorf("error %q should mention read upgrade_available", err.Error())
	}
}

func TestHealthMarker_ConfirmHealthy_SetUpgradeError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.store["upgrade_available"] = "1" // armed → triggers the SetEnv path
	env.setErrOn = "upgrade_available"
	env.setErrText = errors.New("write blocked")
	marker := NewHealthMarker(env)

	err := marker.ConfirmHealthy()
	if err == nil {
		t.Fatal("expected error from SetEnv(upgrade_available) failure, got nil")
	}
	if !strings.Contains(err.Error(), "clear upgrade_available") {
		t.Errorf("error %q should mention clear upgrade_available", err.Error())
	}
}

func TestHealthMarker_ConfirmHealthy_SetBootcountError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.store["upgrade_available"] = "1"
	env.setErrOn = "bootcount"
	env.setErrText = errors.New("write blocked")
	marker := NewHealthMarker(env)

	err := marker.ConfirmHealthy()
	if err == nil {
		t.Fatal("expected error from SetEnv(bootcount) failure, got nil")
	}
	if !strings.Contains(err.Error(), "reset bootcount") {
		t.Errorf("error %q should mention reset bootcount", err.Error())
	}
}

func TestHealthMarker_ConfirmHealthy_SaveError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.store["upgrade_available"] = "1"
	env.saveErr = errors.New("flush failed")
	marker := NewHealthMarker(env)

	err := marker.ConfirmHealthy()
	if err == nil {
		t.Fatal("expected error from SaveEnv failure, got nil")
	}
	if !strings.Contains(err.Error(), "save env after confirm") {
		t.Errorf("error %q should mention save env after confirm", err.Error())
	}
}

func TestHealthMarker_IsArmed_Error(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.getErr = errors.New("env read failure")
	marker := NewHealthMarker(env)

	_, err := marker.IsArmed()
	if err == nil {
		t.Fatal("expected error from IsArmed GetEnv failure, got nil")
	}
	if !strings.Contains(err.Error(), "read upgrade_available") {
		t.Errorf("error %q should mention read upgrade_available", err.Error())
	}
}

func TestHealthMarker_IsArmed_True(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.store["upgrade_available"] = "1"
	marker := NewHealthMarker(env)

	armed, err := marker.IsArmed()
	if err != nil {
		t.Fatalf("IsArmed: %v", err)
	}
	if !armed {
		t.Fatal("expected IsArmed=true when upgrade_available=1")
	}
}

// ---------------------------------------------------------------------------
// ApplyPortClient error branches not yet covered.
// ---------------------------------------------------------------------------

func TestApplyPortClient_Register_RequiresLogin(t *testing.T) {
	t.Parallel()

	c := NewApplyPortClient("http://example.invalid")
	_, err := c.Register(context.Background(), DeviceRegistrationRequest{HardwareID: "hw-1"})
	if err == nil {
		t.Fatal("expected error registering before Login(), got nil")
	}
	if !strings.Contains(err.Error(), "must call Login()") {
		t.Errorf("error %q should require Login()", err.Error())
	}
}

func TestApplyPortClient_Register_HTTPError(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/devices/register": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	c.operatorToken = "op-token" // skip Login() so we reach the HTTP error branch
	_, err := c.Register(context.Background(), DeviceRegistrationRequest{HardwareID: "hw-1"})
	if err == nil {
		t.Fatal("expected error on HTTP 403 register, got nil")
	}
	if !strings.Contains(err.Error(), "register failed (HTTP 403)") {
		t.Errorf("error %q should mention HTTP 403", err.Error())
	}
}

func TestApplyPortClient_CheckForUpdate_BadStatus(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/client/update": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	_, err := c.CheckForUpdate(context.Background(), "1.0.0")
	if err == nil {
		t.Fatal("expected error on HTTP 500 update check, got nil")
	}
	if !strings.Contains(err.Error(), "check-for-update failed (HTTP 500)") {
		t.Errorf("error %q should mention HTTP 500", err.Error())
	}
}

func TestApplyPortClient_CheckForUpdate_DecodeError(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/client/update": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{not json"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	_, err := c.CheckForUpdate(context.Background(), "1.0.0")
	if err == nil {
		t.Fatal("expected decode error on malformed update body, got nil")
	}
	if !strings.Contains(err.Error(), "decode update response") {
		t.Errorf("error %q should mention decode update response", err.Error())
	}
}

func TestApplyPortClient_DownloadBundle_WithRange(t *testing.T) {
	t.Parallel()

	var gotRange string
	srv := newDeviceTestServer(t, map[string]testHandler{
		"/blob": func(w http.ResponseWriter, r *http.Request) {
			gotRange = r.Header.Get("Range")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("partial-bytes"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	data, err := c.DownloadBundle(context.Background(), srv.URL+"/blob", 10, 5)
	if err != nil {
		t.Fatalf("DownloadBundle: %v", err)
	}
	if string(data) != "partial-bytes" {
		t.Fatalf("DownloadBundle data = %q", string(data))
	}
	// offset=10,size=5 → bytes=10-14
	if gotRange != "bytes=10-14" {
		t.Fatalf("Range header = %q, want bytes=10-14", gotRange)
	}
}

func TestApplyPortClient_ReportTelemetry_Rejected(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/client/telemetry": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("malformed"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	_, err := c.ReportTelemetry(context.Background(), TelemetryReport{DeviceID: "d-1"})
	if err == nil {
		t.Fatal("expected error on HTTP 400 telemetry, got nil")
	}
	if !strings.Contains(err.Error(), "telemetry rejected (HTTP 400)") {
		t.Errorf("error %q should mention telemetry rejected", err.Error())
	}
}

func TestApplyPortClient_ReportTelemetry_DecodeError(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/client/telemetry": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("not-json"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	_, err := c.ReportTelemetry(context.Background(), TelemetryReport{DeviceID: "d-1"})
	if err == nil {
		t.Fatal("expected decode error on malformed telemetry ack, got nil")
	}
	if !strings.Contains(err.Error(), "decode telemetry ack") {
		t.Errorf("error %q should mention decode telemetry ack", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ApplyPort.WriteAndArm error branches — writer + env failures.
// ---------------------------------------------------------------------------

// errWriter is a SlotWriter whose individual operations can be made to fail.
type errWriter struct {
	active       string
	writeErr     error
	activeErr    error
	writtenSlot  string
	writtenImage string
}

func (w *errWriter) WriteInactiveSlot(_ context.Context, imagePath string) (string, error) {
	if w.writeErr != nil {
		return "", w.writeErr
	}
	w.writtenImage = imagePath
	if w.active == "A" {
		w.writtenSlot = "B"
	} else {
		w.writtenSlot = "A"
	}
	return w.writtenSlot, nil
}

func (w *errWriter) ActiveSlot() (string, error) {
	if w.activeErr != nil {
		return "", w.activeErr
	}
	if w.active == "" {
		return "A", nil
	}
	return w.active, nil
}

func (w *errWriter) InactiveSlot() (string, error) {
	a, err := w.ActiveSlot()
	if err != nil {
		return "", err
	}
	if a == "A" {
		return "B", nil
	}
	return "A", nil
}

func TestApplyPort_WriteAndArm_WriteError(t *testing.T) {
	t.Parallel()

	w := &errWriter{active: "A", writeErr: errors.New("dd exploded")}
	p := NewApplyPort(w, newMockEnvManager(), "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when WriteInactiveSlot fails, got nil")
	}
	if !strings.Contains(err.Error(), "write inactive slot") {
		t.Errorf("error %q should mention write inactive slot", err.Error())
	}
}

func TestApplyPort_WriteAndArm_ActiveSlotError(t *testing.T) {
	t.Parallel()

	// Write succeeds, but reading the active slot afterwards fails.
	w := &errWriter{active: "A", activeErr: errors.New("cmdline unreadable")}
	// activeErr makes BOTH WriteInactiveSlot's internal InactiveSlot AND the
	// later ActiveSlot fail; WriteInactiveSlot here does not call ActiveSlot,
	// so the first failure surfaces at the ActiveSlot() call.
	p := NewApplyPort(w, newMockEnvManager(), "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when ActiveSlot fails, got nil")
	}
	if !strings.Contains(err.Error(), "read active slot") {
		t.Errorf("error %q should mention read active slot", err.Error())
	}
}

func TestApplyPort_WriteAndArm_SetEnvError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.setErrOn = "BOOT_ORDER"
	env.setErrText = errors.New("env locked")
	p := NewApplyPort(&errWriter{active: "A"}, env, "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when SetEnv(BOOT_ORDER) fails, got nil")
	}
	if !strings.Contains(err.Error(), "set BOOT_ORDER") {
		t.Errorf("error %q should mention set BOOT_ORDER", err.Error())
	}
}

func TestApplyPort_WriteAndArm_SaveEnvError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.saveErr = errors.New("flush failed")
	p := NewApplyPort(&errWriter{active: "A"}, env, "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when SaveEnv fails, got nil")
	}
	if !strings.Contains(err.Error(), "save env") {
		t.Errorf("error %q should mention save env", err.Error())
	}
}

func TestApplyPort_WriteAndArm_SetUpgradeError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.setErrOn = "upgrade_available"
	env.setErrText = errors.New("env locked")
	p := NewApplyPort(&errWriter{active: "A"}, env, "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when SetEnv(upgrade_available) fails, got nil")
	}
	if !strings.Contains(err.Error(), "set upgrade_available") {
		t.Errorf("error %q should mention set upgrade_available", err.Error())
	}
}

func TestApplyPort_WriteAndArm_SetBootcountError(t *testing.T) {
	t.Parallel()

	env := newErrEnv()
	env.setErrOn = "bootcount"
	env.setErrText = errors.New("env locked")
	p := NewApplyPort(&errWriter{active: "A"}, env, "/dev/vda2")
	err := p.WriteAndArm(context.Background(), "/tmp/img")
	if err == nil {
		t.Fatal("expected error when SetEnv(bootcount) fails, got nil")
	}
	if !strings.Contains(err.Error(), "set bootcount") {
		t.Errorf("error %q should mention set bootcount", err.Error())
	}
}

func TestApplyPortClient_Login_DecodeError(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/auth/login": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{bad"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	err := c.Login(context.Background(), "op", "pw")
	if err == nil {
		t.Fatal("expected decode error on malformed login body, got nil")
	}
	if !strings.Contains(err.Error(), "decode login response") {
		t.Errorf("error %q should mention decode login response", err.Error())
	}
}

func TestApplyPortClient_DownloadBundle_BadStatus(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/blob": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("missing"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	_, err := c.DownloadBundle(context.Background(), srv.URL+"/blob", 0, 0)
	if err == nil {
		t.Fatal("expected error on HTTP 404 download, got nil")
	}
	if !strings.Contains(err.Error(), "download bundle failed (HTTP 404)") {
		t.Errorf("error %q should mention HTTP 404", err.Error())
	}
}

func TestApplyPortClient_Register_DecodeError(t *testing.T) {
	t.Parallel()

	srv := newDeviceTestServer(t, map[string]testHandler{
		"/devices/register": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("not-json"))
		},
	})
	defer srv.Close()

	c := NewApplyPortClient(srv.URL)
	c.operatorToken = "op-token"
	_, err := c.Register(context.Background(), DeviceRegistrationRequest{HardwareID: "hw-1"})
	if err == nil {
		t.Fatal("expected decode error on malformed register body, got nil")
	}
	if !strings.Contains(err.Error(), "decode register response") {
		t.Errorf("error %q should mention decode register response", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ddWriter.InactiveSlot — unexpected active slot is NOT an error for ddWriter
// (it has a fixed A/B fold), but slotDevice.InactiveSlot propagates ActiveSlot
// errors. Cover slotDevice.InactiveSlot's error fold via an explicit bad slot.
// ---------------------------------------------------------------------------

func TestSlotDevice_InactiveSlot_UnexpectedActive(t *testing.T) {
	t.Parallel()

	s := NewSlotDevice("/nonexistent/cmdline", "/nonexistent/slot_id", "/dev").(*slotDevice)
	// Force a corrupt cached slot to hit the default branch of InactiveSlot.
	s.cachedSlot = "Z"
	s.cacheSlotOnce.Do(func() {}) // mark cache as populated so detectSlot won't override
	_, err := s.InactiveSlot()
	if err == nil {
		t.Fatal("expected error for unexpected active slot, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected active slot") {
		t.Errorf("error %q should mention unexpected active slot", err.Error())
	}
}

// ---------------------------------------------------------------------------
// shared device test-server helper.
// ---------------------------------------------------------------------------

type testHandler func(http.ResponseWriter, *http.Request)

func newDeviceTestServer(t *testing.T, routes map[string]testHandler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range routes {
		mux.HandleFunc(path, h)
	}
	return httptest.NewServer(mux)
}
