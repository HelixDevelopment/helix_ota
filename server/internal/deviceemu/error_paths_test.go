package deviceemu_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
)

// stubServer is a minimal httptest server whose handler is supplied per-test.
// It lets the error/branch tests exercise the emulator's transport + protocol
// error handling (unexpected status, malformed JSON, missing fields) against
// controlled responses. This validates the DEVICE client logic (the unit under
// test), not the control plane — the full-server end-to-end happy paths live in
// emulator_test.go and drive the REAL router.
func stubServer(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts.URL
}

// newStubDevice builds a device pointed at a stub base URL with a pre-supplied
// OperatorToken (so no real login is needed) and the given current version.
func newStubDevice(t *testing.T, baseURL, current string) *deviceemu.Device {
	t.Helper()
	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        baseURL,
		OperatorToken:  "op-token",
		HardwareID:     "rk3588-STUB",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: current,
	})
	if err != nil {
		t.Fatalf("new stub device: %v", err)
	}
	return dev
}

// TestNewValidation exercises every constructor guard in New: each invalid
// Config must yield a descriptive error and a nil device, and a valid one must
// succeed. These are real misconfiguration defenses — a regression that drops a
// guard would let a device with no BaseURL/identity/credentials be built and
// fail opaquely later.
func TestNewValidation(t *testing.T) {
	valid := deviceemu.Config{
		BaseURL:       "http://h/api/v1",
		OperatorToken: "tok",
		HardwareID:    "hw",
		Model:         "m",
		OSType:        "android",
	}
	cases := []struct {
		name    string
		mutate  func(c *deviceemu.Config)
		wantErr string
	}{
		{"missing base url", func(c *deviceemu.Config) { c.BaseURL = "" }, "BaseURL is required"},
		{"missing hardware id", func(c *deviceemu.Config) { c.HardwareID = "" }, "HardwareID, Model, and OSType are required"},
		{"missing model", func(c *deviceemu.Config) { c.Model = "" }, "HardwareID, Model, and OSType are required"},
		{"missing os", func(c *deviceemu.Config) { c.OSType = "" }, "HardwareID, Model, and OSType are required"},
		{"no credentials", func(c *deviceemu.Config) { c.OperatorToken = ""; c.AdminUser = ""; c.AdminPass = "" }, "either OperatorToken or AdminUser+AdminPass"},
		{"partial credentials (user only)", func(c *deviceemu.Config) { c.OperatorToken = ""; c.AdminUser = "u"; c.AdminPass = "" }, "either OperatorToken or AdminUser+AdminPass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			dev, err := deviceemu.New(c)
			if err == nil {
				t.Fatalf("want error, got nil device=%v", dev)
			}
			if dev != nil {
				t.Fatalf("want nil device on error, got %v", dev)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}

	// The admin-creds path (no OperatorToken) must also build successfully.
	c := valid
	c.OperatorToken = ""
	c.AdminUser = "admin"
	c.AdminPass = "pw"
	if _, err := deviceemu.New(c); err != nil {
		t.Fatalf("admin-creds config should be valid: %v", err)
	}
}

// TestNewDefaults proves New applies its defaults without I/O: a nil HTTPClient
// and nil Now must not panic and the device must report its configured starting
// version. The default 30s client timeout is exercised implicitly by every other
// stub test (they all rely on the default client).
func TestNewDefaults(t *testing.T) {
	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        "http://h/api/v1",
		OperatorToken:  "tok",
		HardwareID:     "hw",
		Model:          "m",
		OSType:         "android",
		CurrentVersion: "9.9.9",
		// HTTPClient nil, Now nil -> defaults applied.
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if got := dev.CurrentVersion(); got != "9.9.9" {
		t.Fatalf("CurrentVersion = %q, want 9.9.9", got)
	}
	if dev.DeviceID() != "" {
		t.Fatalf("DeviceID should be empty before register, got %q", dev.DeviceID())
	}
	if !dev.Healthy() {
		t.Fatalf("device should start healthy")
	}
}

// TestLoginNoOpWithToken proves Login short-circuits when an OperatorToken was
// supplied: it makes NO HTTP call (the stub fails the test if hit) and returns
// nil. A regression that always logs in would break the pre-authenticated
// operator flow.
func TestLoginNoOpWithToken(t *testing.T) {
	var hits int32
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		t.Errorf("unexpected HTTP call to %s — Login should be a no-op with a token", r.URL.Path)
	})
	dev := newStubDevice(t, base, "1.0.0")
	if err := dev.Login(context.Background()); err != nil {
		t.Fatalf("login no-op: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("Login made %d HTTP calls, want 0", n)
	}
}

// TestLoginEmptyAccessToken proves Login rejects a 200 response whose body has
// an empty access_token — a malformed-auth-response guard. Without it the device
// would proceed with an empty bearer and every later call would 401 opaquely.
func TestLoginEmptyAccessToken(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"","token_type":"Bearer"}`))
	})
	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:    base,
		AdminUser:  "admin",
		AdminPass:  "pw",
		HardwareID: "hw",
		Model:      "m",
		OSType:     "android",
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = dev.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty access_token") {
		t.Fatalf("want empty access_token error, got %v", err)
	}
}

// TestLoginBadStatus proves Login surfaces a non-200 (e.g. 401 wrong creds) as a
// wrapped error rather than silently caching a token.
func TestLoginBadStatus(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
	})
	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:    base,
		AdminUser:  "admin",
		AdminPass:  "wrong",
		HardwareID: "hw",
		Model:      "m",
		OSType:     "android",
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = dev.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "login:") {
		t.Fatalf("want wrapped login error, got %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should mention status 401: %v", err)
	}
}

// TestRegisterUnexpectedStatus proves Register rejects any status other than
// 200/201 (here a 500). The accepted-status set is the idempotency contract
// (201 first, 200 replay); a regression widening it would mask server errors.
func TestRegisterUnexpectedStatus(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	})
	dev := newStubDevice(t, base, "1.0.0")
	err := dev.Register(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("want unexpected status 500 error, got %v", err)
	}
}

// TestRegisterMissingFields proves Register rejects a 201 whose body omits the
// device_id/device_token — the device cannot operate without them, so an empty
// identity must be a hard error, not a silent success.
func TestRegisterMissingFields(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"device_id":"","device_token":""}`))
	})
	dev := newStubDevice(t, base, "1.0.0")
	err := dev.Register(context.Background())
	if err == nil || !strings.Contains(err.Error(), "missing device_id or device_token") {
		t.Fatalf("want missing-fields error, got %v", err)
	}
	if dev.DeviceID() != "" {
		t.Fatalf("DeviceID must stay empty on failed register, got %q", dev.DeviceID())
	}
}

// TestRegisterBadJSON proves Register surfaces a decode error when the server
// returns a 200/201 with a non-JSON body (rather than panicking or silently
// continuing with a zero identity).
func TestRegisterBadJSON(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`not-json`))
	})
	dev := newStubDevice(t, base, "1.0.0")
	err := dev.Register(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode body") {
		t.Fatalf("want decode error, got %v", err)
	}
}

// TestRegisterSendsIdempotencyAndAuth asserts Register transmits the operator
// bearer AND the HardwareID as the Idempotency-Key (the server's replay key) and
// accepts a 200 idempotent replay. This guards the idempotency wiring that makes
// repeated registration safe.
func TestRegisterSendsIdempotencyAndAuth(t *testing.T) {
	var gotAuth, gotIdem string
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotIdem = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK) // idempotent replay
		_, _ = w.Write([]byte(`{"device_id":"dev-1","device_token":"dtok-1"}`))
	})
	dev := newStubDevice(t, base, "1.0.0")
	if err := dev.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	if gotAuth != "Bearer op-token" {
		t.Fatalf("Authorization = %q, want Bearer op-token", gotAuth)
	}
	if gotIdem != "rk3588-STUB" {
		t.Fatalf("Idempotency-Key = %q, want hardware id rk3588-STUB", gotIdem)
	}
	if dev.DeviceID() != "dev-1" {
		t.Fatalf("DeviceID = %q, want dev-1", dev.DeviceID())
	}
}

// TestCheckUpdateNotRegistered proves CheckUpdate refuses to poll before
// registration (no device token) — it must error rather than send an
// unauthenticated request.
func TestCheckUpdateNotRegistered(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected HTTP call %s — should not poll unregistered", r.URL.Path)
	})
	dev := newStubDevice(t, base, "1.0.0")
	_, _, err := dev.CheckUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "device not registered") {
		t.Fatalf("want not-registered error, got %v", err)
	}
}

// TestCheckUpdateUnexpectedStatus proves CheckUpdate maps a non-200/204 (here
// 503) into an error rather than treating it as on-target or a valid offer.
func TestCheckUpdateUnexpectedStatus(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`down`))
		}
	})
	dev := newStubDevice(t, base, "1.0.0")
	if err := dev.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, _, err := dev.CheckUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected status 503") {
		t.Fatalf("want unexpected status 503 error, got %v", err)
	}
}

// TestCheckUpdateBadJSON proves a 200 offer with a malformed body surfaces a
// decode error rather than returning a half-populated UpdateAvailable.
func TestCheckUpdateBadJSON(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{bad json`))
		}
	})
	dev := newStubDevice(t, base, "1.0.0")
	if err := dev.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, _, err := dev.CheckUpdate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode body") {
		t.Fatalf("want decode error, got %v", err)
	}
}

// TestCheckUpdateNoCurrentVersionOmitsQuery proves the current=="" branch: when
// the device has no on-disk version, CheckUpdate must NOT append a
// ?current_version= query param (an empty value would be a malformed poll). A
// device with a version DOES include it. Both forms must yield the 204 on-target
// result the stub returns.
func TestCheckUpdateNoCurrentVersionOmitsQuery(t *testing.T) {
	var rawQuery string
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		default:
			rawQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Empty current version -> no query string.
	devEmpty := newStubDevice(t, base, "")
	if err := devEmpty.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, onTarget, err := devEmpty.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if !onTarget {
		t.Fatalf("want on-target (204)")
	}
	if rawQuery != "" {
		t.Fatalf("empty current version must omit query, got %q", rawQuery)
	}

	// Non-empty current version -> query carries it.
	devVer := newStubDevice(t, base, "2.3.4")
	if err := devVer.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, _, err := devVer.CheckUpdate(context.Background()); err != nil {
		t.Fatalf("check update: %v", err)
	}
	if rawQuery != "current_version=2.3.4" {
		t.Fatalf("query = %q, want current_version=2.3.4", rawQuery)
	}
}

// TestApplyAndReportNilUpdate proves the nil-guard: applying a nil offer is a
// programmer error that must surface immediately, not stamp telemetry for a
// missing version.
func TestApplyAndReportNilUpdate(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected HTTP call %s — nil update must short-circuit", r.URL.Path)
	})
	dev := newStubDevice(t, base, "1.0.0")
	_, err := dev.ApplyAndReport(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "nil update") {
		t.Fatalf("want nil update error, got %v", err)
	}
}

// TestReportEventNotRegistered proves both telemetry entry points
// (ApplyAndReport and ReportFailure) refuse to report before registration: with
// no device token the reportEvent guard must fire and no HTTP request is made.
func TestReportEventNotRegistered(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected HTTP call %s — should not report unregistered", r.URL.Path)
	})
	dev := newStubDevice(t, base, "1.0.0")

	upd := &otaprotocol.UpdateAvailable{Version: "1.1.0"}
	_, err := dev.ApplyAndReport(context.Background(), upd)
	if err == nil || !strings.Contains(err.Error(), "device not registered") {
		t.Fatalf("ApplyAndReport want not-registered, got %v", err)
	}

	_, err = dev.ReportFailure(context.Background(), "E_X")
	if err == nil || !strings.Contains(err.Error(), "device not registered") {
		t.Fatalf("ReportFailure want not-registered, got %v", err)
	}
}

// TestApplyAndReportTelemetryRejectedStatus proves a non-202 telemetry response
// (here 400 from the schema validator) is surfaced as a wrapped "apply: report"
// error that names the failing lifecycle event — and the device version is NOT
// advanced because the apply did not complete.
func TestApplyAndReportTelemetryRejectedStatus(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		case strings.HasSuffix(r.URL.Path, "/client/telemetry"):
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"schema"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	dev := newStubDevice(t, base, "1.0.0")
	if err := dev.Register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	upd := &otaprotocol.UpdateAvailable{Version: "1.1.0", Size: 1024}
	_, err := dev.ApplyAndReport(context.Background(), upd)
	if err == nil || !strings.Contains(err.Error(), "apply: report") {
		t.Fatalf("want apply: report error, got %v", err)
	}
	if got := dev.CurrentVersion(); got != "1.0.0" {
		t.Fatalf("version must NOT advance on telemetry failure, got %q", got)
	}
	if !dev.Healthy() {
		t.Fatalf("device health untouched on a mid-apply telemetry failure")
	}
}

// TestDoJSONTransportError proves a transport-level failure (server closed,
// connection refused) is wrapped as a "do:" error rather than panicking — the
// device must degrade gracefully when the control plane is unreachable.
func TestDoJSONTransportError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := ts.URL
	ts.Close() // close immediately so the connection is refused

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:    base,
		AdminUser:  "admin",
		AdminPass:  "pw",
		HardwareID: "hw",
		Model:      "m",
		OSType:     "android",
		HTTPClient: &http.Client{Timeout: 2 * time.Second},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = dev.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "login:") {
		t.Fatalf("want wrapped login transport error, got %v", err)
	}
}

// TestRunLoopRunsThenCancels proves RunLoop registers once, runs the first cycle
// immediately (not after one interval), and exits with ctx.Err() on
// cancellation. The first immediate cycle reports an on-target Outcome via
// onOutcome; cancellation then unwinds the loop. This is the previously-0%
// orchestration path.
func TestRunLoopRunsThenCancels(t *testing.T) {
	var checks int32
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"loop-dev","device_token":"loop-tok"}`))
		default: // /client/update -> always on-target
			atomic.AddInt32(&checks, 1)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	dev := newStubDevice(t, base, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	outc := make(chan deviceemu.Outcome, 4)
	done := make(chan error, 1)
	go func() {
		// Long interval so only the immediate first cycle runs before we cancel.
		done <- dev.RunLoop(ctx, time.Hour, func(o deviceemu.Outcome) { outc <- o }, nil)
	}()

	select {
	case o := <-outc:
		if !o.OnTarget {
			t.Fatalf("first immediate cycle should be on-target: %+v", o)
		}
		if o.DeviceID != "loop-dev" {
			t.Fatalf("outcome device id = %q, want loop-dev", o.DeviceID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunLoop did not run the first cycle immediately")
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("RunLoop returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunLoop did not exit on cancel")
	}
	if atomic.LoadInt32(&checks) < 1 {
		t.Fatalf("expected at least one update check, got %d", checks)
	}
}

// TestRunLoopRegisterErrorReturns proves RunLoop fails fast when up-front
// registration fails: it returns the register error and never enters the ticker
// loop (no onOutcome/onErr callback fires for a cycle).
func TestRunLoopRegisterErrorReturns(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`nope`))
	})
	dev := newStubDevice(t, base, "1.0.0")

	called := false
	err := dev.RunLoop(context.Background(), time.Hour,
		func(deviceemu.Outcome) { called = true },
		func(error) { called = true },
	)
	if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("want register failure from RunLoop, got %v", err)
	}
	if called {
		t.Fatalf("no cycle callback should fire when up-front register fails")
	}
}

// TestRunLoopOnErrInvokedThenCancel proves the per-cycle error path: when a
// cycle's CheckUpdate fails (here a 503 after a successful register), RunLoop
// routes the error to onErr (not onOutcome) and keeps looping until cancelled.
func TestRunLoopOnErrInvokedThenCancel(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	dev := newStubDevice(t, base, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 4)
	done := make(chan error, 1)
	go func() {
		done <- dev.RunLoop(ctx, time.Hour,
			func(deviceemu.Outcome) { t.Errorf("onOutcome must NOT fire when the cycle errors") },
			func(e error) { errc <- e },
		)
	}()

	select {
	case e := <-errc:
		if !strings.Contains(e.Error(), "unexpected status 503") {
			t.Fatalf("onErr got %v, want 503 cycle error", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("onErr not invoked for the failing first cycle")
	}

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("RunLoop returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunLoop did not exit on cancel")
	}
}

// TestRunLoopDefaultInterval proves the interval<=0 default branch is taken
// (interval becomes 15m) without changing the immediate-first-cycle behaviour:
// with a 0 interval the first cycle still runs immediately and cancellation
// still unwinds cleanly (the 15m ticker never fires in the test window).
func TestRunLoopDefaultInterval(t *testing.T) {
	base := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/devices/register"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"device_id":"d","device_token":"dt"}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	dev := newStubDevice(t, base, "1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	outc := make(chan deviceemu.Outcome, 1)
	done := make(chan error, 1)
	go func() {
		done <- dev.RunLoop(ctx, 0, func(o deviceemu.Outcome) { outc <- o }, nil)
	}()

	select {
	case <-outc: // immediate first cycle fired despite interval<=0
	case <-time.After(5 * time.Second):
		t.Fatal("default-interval RunLoop did not run the immediate cycle")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("RunLoop returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunLoop did not exit on cancel")
	}
}
