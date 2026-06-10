// Package deviceemu is a faithful Go emulator of an RK3588 / Orange Pi 5 Max
// OTA client. It speaks the EXACT HTTP control-plane protocol a real native
// Android A/B device speaks against the Helix OTA server — login (operator),
// device registration, update-check, and the telemetry lifecycle — so the
// control plane can be exercised end-to-end (Tier-1) without physical hardware.
//
// Faithfulness (anti-bluff, Constitution §11.4.27): the emulator drives the
// REAL endpoints over real HTTP and reuses the canonical ota-protocol wire
// types. It is NOT a mock of the server; it is a substitute for the DEVICE. The
// only thing it fakes is the act of flashing — instead of writing a slot it
// advances its in-memory CurrentVersion and reports the success lifecycle the
// same way update_engine would.
//
// Protocol gap (FACT, §11.4.6): the telemetry-schema validator REQUIRES a
// deployment_id (server/internal/api/handlers_client.go:135), but
// otaprotocol.UpdateAvailable (submodules/ota-protocol/types.go:87-101) does
// NOT expose one. A real device therefore cannot derive the deployment_id from
// its update offer. The faithful resolution is operator-side: the principal that
// created the deployment knows its id from the POST /deployments 201 body (or a
// GET /deployments/{id}). The emulator accepts that id via Config.DeploymentID
// (operator-supplied, out-of-band) and attaches it to every telemetry report —
// mirroring how the server's own test seedTelemetry obtains it via
// ListActiveDeployments. When no deployment_id is supplied, the emulator still
// emits the lifecycle, and the server's schema validator rejects the events
// (counted in TelemetryAck.rejected) — the emulator surfaces that honestly
// rather than inventing an id.
package deviceemu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// Config drives a single emulated device. Either AdminUser+AdminPass (the
// emulator logs in to obtain an operator token used to register the device) or a
// pre-supplied OperatorToken must be set.
type Config struct {
	// BaseURL is the server origin including the API base path, e.g.
	// "http://127.0.0.1:8080/api/v1".
	BaseURL string

	// AdminUser / AdminPass are operator credentials. When OperatorToken is empty
	// the emulator logs in with these to register the device.
	AdminUser string
	AdminPass string

	// OperatorToken, when set, is used directly for device registration instead of
	// logging in (the operator already authenticated out-of-band).
	OperatorToken string

	// HardwareID, Model, OSType identify the device on registration. OSType uses
	// the canonical ota-protocol enum value (e.g. "android").
	HardwareID string
	Model      string
	OSType     string

	// CurrentVersion is the device's starting on-disk firmware version. It
	// advances as the emulator applies updates.
	CurrentVersion string

	// Group optionally places the device in a deployment target group.
	Group string

	// DeploymentID is the operator-supplied deployment id attached to telemetry.
	// See the package-level protocol-gap note: it cannot be derived from the
	// update offer, so the operator provides it. May be empty (telemetry then
	// reports the lifecycle but the server's schema validator rejects it).
	DeploymentID string

	// HTTPClient is the transport. nil defaults to a client with a 30s timeout.
	HTTPClient *http.Client

	// Now is the clock used for telemetry timestamps. nil defaults to time.Now.
	Now func() time.Time
}

// Device is a stateful emulated OTA client. It is safe for sequential use; the
// internal state mutations are guarded so a RunLoop and a concurrent reader of
// CurrentVersion do not race.
type Device struct {
	cfg    Config
	client *http.Client
	now    func() time.Time

	mu            sync.Mutex
	operatorToken string
	deviceID      string
	deviceToken   string
	current       string
	healthy       bool
}

// New builds a Device from cfg, applying defaults. It does not perform any I/O.
func New(cfg Config) (*Device, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("deviceemu: BaseURL is required")
	}
	if cfg.HardwareID == "" || cfg.Model == "" || cfg.OSType == "" {
		return nil, fmt.Errorf("deviceemu: HardwareID, Model, and OSType are required")
	}
	if cfg.OperatorToken == "" && (cfg.AdminUser == "" || cfg.AdminPass == "") {
		return nil, fmt.Errorf("deviceemu: either OperatorToken or AdminUser+AdminPass must be set")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Device{
		cfg:           cfg,
		client:        client,
		now:           now,
		operatorToken: cfg.OperatorToken,
		current:       cfg.CurrentVersion,
		healthy:       true,
	}, nil
}

// DeviceID returns the server-assigned device id (empty until Register).
func (d *Device) DeviceID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deviceID
}

// CurrentVersion returns the device's current on-disk firmware version.
func (d *Device) CurrentVersion() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.current
}

// Healthy reports the device's last-known health (false after a reported
// failure, true after a reported success).
func (d *Device) Healthy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.healthy
}

// Login exchanges the configured operator credentials for an access token and
// caches it. It is a no-op when an OperatorToken was supplied.
func (d *Device) Login(ctx context.Context) error {
	d.mu.Lock()
	if d.operatorToken != "" {
		d.mu.Unlock()
		return nil
	}
	user, pass := d.cfg.AdminUser, d.cfg.AdminPass
	d.mu.Unlock()

	body := loginRequest{Username: user, Password: pass}
	var resp tokenResponse
	if err := d.doJSON(ctx, http.MethodPost, "/auth/login", "", body, http.StatusOK, &resp); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if resp.AccessToken == "" {
		return fmt.Errorf("login: empty access_token in response")
	}
	d.mu.Lock()
	d.operatorToken = resp.AccessToken
	d.mu.Unlock()
	return nil
}

// Register provisions the device on the control plane (operator-authed) and
// captures its device_token + device_id. It is idempotent: the HardwareID is
// used as the Idempotency-Key so repeated calls return the original identity
// (the server replays with 200; first call is 201).
func (d *Device) Register(ctx context.Context) error {
	if err := d.Login(ctx); err != nil {
		return err
	}
	d.mu.Lock()
	opTok := d.operatorToken
	reg := deviceRegistration{
		HardwareID:     d.cfg.HardwareID,
		Model:          d.cfg.Model,
		OS:             d.cfg.OSType,
		CurrentVersion: d.current,
		Group:          d.cfg.Group,
	}
	idemKey := d.cfg.HardwareID
	d.mu.Unlock()

	reqBody, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("register: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url("/devices/register"), bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("register: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+opTok)
	req.Header.Set("Idempotency-Key", idemKey)

	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("register: do: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	// The server returns 201 on first registration and 200 on idempotent replay.
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		return fmt.Errorf("register: unexpected status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var resp deviceRegistered
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("register: decode body %q: %w", string(raw), err)
	}
	if resp.DeviceID == "" || resp.DeviceToken == "" {
		return fmt.Errorf("register: missing device_id or device_token in response: %s", string(raw))
	}
	d.mu.Lock()
	d.deviceID = resp.DeviceID
	d.deviceToken = resp.DeviceToken
	d.mu.Unlock()
	return nil
}

// CheckUpdate polls GET /client/update with the device's current version. It
// returns the offered update (nil when on target), an onTarget flag (true when
// the server answered 204 No Content), and any transport/protocol error.
func (d *Device) CheckUpdate(ctx context.Context) (upd *otaprotocol.UpdateAvailable, onTarget bool, err error) {
	d.mu.Lock()
	devTok := d.deviceToken
	current := d.current
	d.mu.Unlock()
	if devTok == "" {
		return nil, false, fmt.Errorf("check update: device not registered")
	}

	path := "/client/update"
	if current != "" {
		path += "?current_version=" + current
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url(path), nil)
	if err != nil {
		return nil, false, fmt.Errorf("check update: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+devTok)
	res, err := d.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("check update: do: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	switch res.StatusCode {
	case http.StatusNoContent:
		return nil, true, nil
	case http.StatusOK:
		var u otaprotocol.UpdateAvailable
		if err := json.Unmarshal(raw, &u); err != nil {
			return nil, false, fmt.Errorf("check update: decode body %q: %w", string(raw), err)
		}
		return &u, false, nil
	default:
		return nil, false, fmt.Errorf("check update: unexpected status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
}

// ApplyAndReport emulates applying an offered update: it reports the real
// telemetry lifecycle (download_started -> installing -> installed -> verifying
// -> success), then advances the device's local CurrentVersion to the update's
// version. The returned ack is the server's response to the FINAL (success)
// telemetry batch. It does not move bytes — flashing is the only faked step
// (see package doc).
func (d *Device) ApplyAndReport(ctx context.Context, upd *otaprotocol.UpdateAvailable) (TelemetryAck, error) {
	if upd == nil {
		return TelemetryAck{}, fmt.Errorf("apply: nil update")
	}
	lifecycle := []otaprotocol.TelemetryEvent{
		otaprotocol.EventDownloadStarted,
		otaprotocol.EventInstalling,
		otaprotocol.EventInstalled,
		otaprotocol.EventVerifying,
		otaprotocol.EventSuccess,
	}
	var ack TelemetryAck
	for _, ev := range lifecycle {
		a, err := d.reportEvent(ctx, ev, upd.Version, nil)
		if err != nil {
			return ack, fmt.Errorf("apply: report %s: %w", ev, err)
		}
		ack = a
	}
	// "Flash" complete: advance the on-disk version and mark healthy.
	d.mu.Lock()
	d.current = upd.Version
	d.healthy = true
	d.mu.Unlock()
	return ack, nil
}

// ReportFailure emulates a failed apply (e.g. AVB/dm-verity rejection or a
// post-reboot health check failure that triggers auto-rollback). It reports a
// failure event with the given error code and marks the device unhealthy. The
// device's CurrentVersion is NOT advanced (the slot rolled back).
func (d *Device) ReportFailure(ctx context.Context, errorCode string) (TelemetryAck, error) {
	d.mu.Lock()
	target := d.current
	d.mu.Unlock()
	ec := errorCode
	ack, err := d.reportEvent(ctx, otaprotocol.EventFailure, target, &ec)
	if err != nil {
		return ack, fmt.Errorf("report failure: %w", err)
	}
	d.mu.Lock()
	d.healthy = false
	d.mu.Unlock()
	return ack, nil
}

// reportEvent posts a single-event telemetry batch and returns the 202 ack.
func (d *Device) reportEvent(ctx context.Context, ev otaprotocol.TelemetryEvent, version string, errorCode *string) (TelemetryAck, error) {
	d.mu.Lock()
	devID := d.deviceID
	devTok := d.deviceToken
	depID := d.cfg.DeploymentID
	d.mu.Unlock()
	if devTok == "" {
		return TelemetryAck{}, fmt.Errorf("device not registered")
	}

	report := telemetryReport{
		DeviceID:     devID,
		DeploymentID: depID,
		Events: []telemetryEventWire{
			{
				Event:     string(ev),
				Version:   version,
				Timestamp: d.now().UTC(),
				ErrorCode: errorCode,
			},
		},
	}
	var ack TelemetryAck
	if err := d.doJSON(ctx, http.MethodPost, "/client/telemetry", devTok, report, http.StatusAccepted, &ack); err != nil {
		return TelemetryAck{}, err
	}
	return ack, nil
}

// Outcome is the structured result of one RunOnce check->apply cycle.
type Outcome struct {
	DeviceID       string `json:"device_id"`
	OnTarget       bool   `json:"on_target"`
	Applied        bool   `json:"applied"`
	FromVersion    string `json:"from_version"`
	ToVersion      string `json:"to_version,omitempty"`
	OfferedVersion string `json:"offered_version,omitempty"`
	DeltaOffered   bool   `json:"delta_offered"`
	// TelemetryAccepted / Rejected are the FINAL success-batch ack counts. A
	// non-zero Rejected with an empty DeploymentID is the §11.4.6 protocol-gap
	// signal (the schema validator dropped the events for lack of a deployment_id).
	TelemetryAccepted int    `json:"telemetry_accepted"`
	TelemetryRejected int    `json:"telemetry_rejected"`
	Healthy           bool   `json:"healthy"`
	Note              string `json:"note,omitempty"`
}

// RunOnce performs one full cycle: ensure registration, check for an update,
// and (if offered) apply+report it. It returns a structured Outcome. RunOnce is
// the unit a CLI -once invocation or a RunLoop iteration uses.
func (d *Device) RunOnce(ctx context.Context) (Outcome, error) {
	if d.DeviceID() == "" {
		if err := d.Register(ctx); err != nil {
			return Outcome{}, err
		}
	}
	from := d.CurrentVersion()
	out := Outcome{DeviceID: d.DeviceID(), FromVersion: from, Healthy: d.Healthy()}

	upd, onTarget, err := d.CheckUpdate(ctx)
	if err != nil {
		return out, err
	}
	if onTarget || upd == nil {
		out.OnTarget = true
		out.Note = "device on target — no update offered (204)"
		return out, nil
	}
	out.OfferedVersion = upd.Version
	out.DeltaOffered = upd.Delta != nil

	ack, err := d.ApplyAndReport(ctx, upd)
	if err != nil {
		return out, err
	}
	out.Applied = true
	out.ToVersion = d.CurrentVersion()
	out.TelemetryAccepted = ack.Accepted
	out.TelemetryRejected = ack.Rejected
	out.Healthy = d.Healthy()
	if ack.Rejected > 0 && d.cfg.DeploymentID == "" {
		out.Note = "telemetry rejected by schema validator: no deployment_id supplied (protocol gap, §11.4.6)"
	}
	return out, nil
}

// RunLoop runs RunOnce on the given interval until ctx is cancelled, invoking
// onOutcome for each cycle's result (or onErr for a cycle error). It registers
// once up front. interval <= 0 defaults to 15 minutes (the spec poll cadence).
func (d *Device) RunLoop(ctx context.Context, interval time.Duration, onOutcome func(Outcome), onErr func(error)) error {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if err := d.Register(ctx); err != nil {
		return err
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	// Run the first cycle immediately rather than after one interval.
	run := func() {
		out, err := d.RunOnce(ctx)
		if err != nil {
			if onErr != nil {
				onErr(err)
			}
			return
		}
		if onOutcome != nil {
			onOutcome(out)
		}
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			run()
		}
	}
}

// doJSON marshals body, POSTs/GETs it with the optional bearer token, asserts
// the expected status (want), and decodes the response into out (when non-nil).
func (d *Device) doJSON(ctx context.Context, method, path, token string, body any, want int, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, d.url(path), reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		return fmt.Errorf("unexpected status %d (want %d): %s", res.StatusCode, want, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode body %q: %w", string(raw), err)
		}
	}
	return nil
}

// url joins the configured base URL with a path.
func (d *Device) url(path string) string {
	return strings.TrimRight(d.cfg.BaseURL, "/") + path
}

// --- wire shapes (mirror server/internal/api/wire.go json tags exactly) ---

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	Roles       []string `json:"roles,omitempty"`
}

type deviceRegistration struct {
	HardwareID     string `json:"hardware_id"`
	Model          string `json:"model"`
	OS             string `json:"os"`
	CurrentVersion string `json:"current_version,omitempty"`
	Group          string `json:"group,omitempty"`
}

type deviceRegistered struct {
	DeviceID    string `json:"device_id"`
	HardwareID  string `json:"hardware_id"`
	DeviceToken string `json:"device_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type telemetryEventWire struct {
	Event     string    `json:"event"`
	Version   string    `json:"version,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	ErrorCode *string   `json:"error_code"`
}

type telemetryReport struct {
	DeviceID     string               `json:"device_id"`
	DeploymentID string               `json:"deployment_id,omitempty"`
	Events       []telemetryEventWire `json:"events"`
}

// TelemetryAck is the 202 body returned by POST /client/telemetry.
type TelemetryAck struct {
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected"`
	RequestID string `json:"request_id,omitempty"`
}
