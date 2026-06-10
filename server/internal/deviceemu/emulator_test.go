package deviceemu_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/api"
	"github.com/HelixDevelopment/helix_ota/server/internal/config"
	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
	"github.com/HelixDevelopment/helix_ota/server/internal/health"
	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// serverFixture boots the REAL control-plane router in-process over httptest and
// returns its base URL plus the operator credentials + the signing key needed to
// stage a deployment. The emulator drives this server via real HTTP — it logs in
// via /auth/login (NOT by reaching into internals), exactly as a device would.
type serverFixture struct {
	baseURL   string
	adminUser string
	adminPass string
	priKey    ed25519.PrivateKey
	ts        *httptest.Server
}

const (
	fxAdminUser = "admin@helix.test"
	fxAdminPass = "s3cret"
	fxModel     = "OrangePi5Max"
)

func bootServer(t *testing.T) *serverFixture {
	t.Helper()
	pub, pri, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate artifact key: %v", err)
	}
	repo := store.NewMemoryRepository()
	cfg := config.Config{
		APIBasePath:     "/api/v1",
		AccessTokenTTL:  15 * time.Minute,
		DeviceTokenTTL:  24 * time.Hour,
		MaxUploadBytes:  8 << 20,
		ArtifactBaseURL: "https://artifacts.test",
		TokenSecret:     []byte("emu-test-secret"),
	}
	srv := api.NewServer(api.Options{
		Config: cfg,
		Repo:   repo,
		Users: api.NewStaticUserDirectory(api.StaticUser{
			Username: fxAdminUser,
			Password: fxAdminPass,
			Roles:    []string{api.RoleAdmin, api.RoleOperator, api.RoleViewer},
		}),
		Health:      health.New(func(context.Context) bool { return true }),
		ArtifactKey: pub,
	})
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return &serverFixture{
		baseURL:   ts.URL + "/api/v1",
		adminUser: fxAdminUser,
		adminPass: fxAdminPass,
		priKey:    pri,
		ts:        ts,
	}
}

// operatorLogin logs in over real HTTP and returns the operator access token.
func (f *serverFixture) operatorLogin(t *testing.T) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": f.adminUser, "password": f.adminPass})
	res, err := http.Post(f.baseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("operator login: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("operator login want 200, got %d: %s", res.StatusCode, raw)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tr); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return tr.AccessToken
}

// stageDeployment uploads a valid artifact, publishes a release, and creates an
// all-targets deployment over real HTTP using the operator token. It returns the
// release version and the deployment id. This is the OPERATOR side of the world
// (the device side is the emulator under test). The artifact signing mirrors the
// api package's own test fixture: sign the hex-decoded SHA-256 digest of the ZIP.
func (f *serverFixture) stageDeployment(t *testing.T, opToken, version, group string) (relVersion, deploymentID string) {
	t.Helper()

	payload := []byte("emu payload " + version)
	file := zipStored(t, payload)
	digestHex := sha256Hex(file)
	digest, err := hex.DecodeString(digestHex)
	if err != nil {
		t.Fatalf("decode digest: %v", err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(f.priKey, digest))

	meta := map[string]any{
		"sha256":        digestHex,
		"signature":     sig,
		"version":       version,
		"os":            string(otaprotocol.OSAndroid),
		"target_model":  fxModel,
		"file_hash":     base64.StdEncoding.EncodeToString([]byte("file-hash")),
		"file_size":     len(file),
		"metadata_hash": base64.StdEncoding.EncodeToString([]byte("meta-hash")),
		"metadata_size": 64,
	}
	mbody, ct := uploadMultipart(t, file, meta)
	artifactID := f.postArtifact(t, opToken, mbody, ct)

	// Publish a release.
	relBody, _ := json.Marshal(map[string]any{
		"artifact_id": artifactID, "version": version,
		"os": string(otaprotocol.OSAndroid), "target_model": fxModel,
	})
	var rel struct {
		ReleaseID string `json:"release_id"`
		Version   string `json:"version"`
	}
	f.postJSON(t, opToken, "/releases", relBody, http.StatusCreated, &rel)

	// Create an all-targets deployment; capture its id (the operator's only
	// source of the deployment_id — see deviceemu protocol-gap note).
	depBody, _ := json.Marshal(map[string]any{
		"release_id": rel.ReleaseID, "strategy": "all-targets", "group": group,
	})
	var dep struct {
		DeploymentID string `json:"deployment_id"`
	}
	f.postJSON(t, opToken, "/deployments", depBody, http.StatusCreated, &dep)
	if dep.DeploymentID == "" {
		t.Fatalf("deployment id empty")
	}
	return rel.Version, dep.DeploymentID
}

// postArtifact posts the multipart artifact upload and returns the artifact id.
func (f *serverFixture) postArtifact(t *testing.T, opToken string, body []byte, ct string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, f.baseURL+"/artifacts/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload artifact: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload want 201, got %d: %s", res.StatusCode, raw)
	}
	var art struct {
		ArtifactID string `json:"artifact_id"`
	}
	if err := json.Unmarshal(raw, &art); err != nil {
		t.Fatalf("decode artifact %q: %v", raw, err)
	}
	if art.ArtifactID == "" {
		t.Fatalf("artifact id empty: %s", raw)
	}
	return art.ArtifactID
}

// postJSON posts a JSON body with the operator token and decodes the response.
func (f *serverFixture) postJSON(t *testing.T, opToken, path string, body []byte, want int, out any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, f.baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		t.Fatalf("POST %s want %d, got %d: %s", path, want, res.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode %s %q: %v", path, raw, err)
		}
	}
}

// TestEmulatorFullLifecycle is the END-TO-END Tier-1 proof: a real server boots
// in-process, an operator stages a deployment, and the emulator (login ->
// register -> check -> apply+report -> recheck) drives the real protocol. The
// device must move from behind to on-target with its CurrentVersion advanced and
// telemetry accepted.
func TestEmulatorFullLifecycle(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()

	opToken := fx.operatorLogin(t)
	const group = "field-fleet-a"
	relVersion, deploymentID := fx.stageDeployment(t, opToken, "1.1.0", group)

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-EMUTEST01",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.0.0",
		Group:          group,
		DeploymentID:   deploymentID,
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}

	// 1) Register (also logs in via /auth/login under the hood).
	if err := dev.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}
	if dev.DeviceID() == "" {
		t.Fatalf("device id not captured after register")
	}

	// 2) First check: an update to 1.1.0 must be offered (device is on 1.0.0).
	upd, onTarget, err := dev.CheckUpdate(ctx)
	if err != nil {
		t.Fatalf("check update: %v", err)
	}
	if onTarget || upd == nil {
		t.Fatalf("expected an update offer, got onTarget=%v upd=%v", onTarget, upd)
	}
	if upd.Version != relVersion {
		t.Fatalf("offered version %q want %q", upd.Version, relVersion)
	}

	// 3) Apply + report the full success lifecycle.
	ack, err := dev.ApplyAndReport(ctx, upd)
	if err != nil {
		t.Fatalf("apply+report: %v", err)
	}
	if ack.Accepted != 1 || ack.Rejected != 0 {
		t.Fatalf("success telemetry want accepted=1 rejected=0, got %+v", ack)
	}
	if got := dev.CurrentVersion(); got != relVersion {
		t.Fatalf("CurrentVersion after apply = %q, want %q", got, relVersion)
	}
	if !dev.Healthy() {
		t.Fatalf("device should be healthy after success")
	}

	// 4) Re-check: the device is now on target -> 204 No Content.
	upd2, onTarget2, err := dev.CheckUpdate(ctx)
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if !onTarget2 || upd2 != nil {
		t.Fatalf("after apply expected on-target (204), got onTarget=%v upd=%v", onTarget2, upd2)
	}

	// 5) Cross-check via the operator: device status reflects success + version.
	verifyDeviceStatus(t, fx, opToken, dev.DeviceID(), relVersion, true)

	// 6) Telemetry history must contain the success event for this deployment.
	verifySuccessTelemetry(t, fx, opToken, dev.DeviceID(), deploymentID)
}

// TestEmulatorRunOnce proves the orchestrated single-cycle path returns a
// structured Outcome with the version advanced and telemetry accepted.
func TestEmulatorRunOnce(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	opToken := fx.operatorLogin(t)
	const group = "ro-fleet"
	relVersion, deploymentID := fx.stageDeployment(t, opToken, "2.0.0", group)

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-RUNONCE",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.5.0",
		Group:          group,
		DeploymentID:   deploymentID,
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}

	out, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !out.Applied || out.ToVersion != relVersion || out.FromVersion != "1.5.0" {
		t.Fatalf("run-once outcome mismatch: %+v", out)
	}
	if out.TelemetryAccepted != 1 || out.TelemetryRejected != 0 {
		t.Fatalf("run-once telemetry want accepted=1 rejected=0, got %+v", out)
	}
	if !out.Healthy {
		t.Fatalf("run-once device should be healthy: %+v", out)
	}

	// A second RunOnce is now on-target (idempotent register, 204 check).
	out2, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("second run once: %v", err)
	}
	if !out2.OnTarget || out2.Applied {
		t.Fatalf("second run-once should be on-target no-apply: %+v", out2)
	}
}

// TestEmulatorReportFailure proves the rollback path: a device that reports a
// failure event is marked unhealthy on the server, and its current version is
// NOT advanced (the A/B slot rolled back).
func TestEmulatorReportFailure(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	opToken := fx.operatorLogin(t)
	const group = "fail-fleet"
	_, deploymentID := fx.stageDeployment(t, opToken, "3.0.0", group)

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-FAIL01",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.0.0",
		Group:          group,
		DeploymentID:   deploymentID,
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}
	if err := dev.Register(ctx); err != nil {
		t.Fatalf("register: %v", err)
	}

	ack, err := dev.ReportFailure(ctx, "E_AVB_VERIFY")
	if err != nil {
		t.Fatalf("report failure: %v", err)
	}
	if ack.Accepted != 1 || ack.Rejected != 0 {
		t.Fatalf("failure telemetry want accepted=1 rejected=0, got %+v", ack)
	}
	if dev.Healthy() {
		t.Fatalf("device should be unhealthy after a reported failure")
	}
	// The on-disk version must NOT have advanced (rollback).
	if got := dev.CurrentVersion(); got != "1.0.0" {
		t.Fatalf("CurrentVersion after failure = %q, want unchanged 1.0.0", got)
	}
	// Operator-side cross-check: server marks the device unhealthy with the code.
	verifyDeviceStatus(t, fx, opToken, dev.DeviceID(), "", false)
}

// TestEmulatorTelemetryProtocolGap is a FACT-grade probe of the §11.4.6
// protocol gap: with NO operator-supplied deployment_id, the telemetry-schema
// validator rejects the lifecycle events. The emulator surfaces this honestly
// (rejected>0) instead of inventing an id.
func TestEmulatorTelemetryProtocolGap(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	opToken := fx.operatorLogin(t)
	const group = "gap-fleet"
	_, _ = fx.stageDeployment(t, opToken, "4.0.0", group)

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-GAP01",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.0.0",
		Group:          group,
		// DeploymentID intentionally omitted.
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}
	out, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !out.Applied {
		t.Fatalf("update should still be offered+applied locally: %+v", out)
	}
	// The success batch's events are rejected for lack of deployment_id.
	if out.TelemetryRejected == 0 {
		t.Fatalf("expected telemetry rejected without deployment_id (protocol gap), got %+v", out)
	}
}

// verifyDeviceStatus reads the device status over real HTTP (operator token) and
// asserts the health + (optionally) the current version.
func verifyDeviceStatus(t *testing.T, fx *serverFixture, opToken, deviceID, wantVersion string, wantHealthy bool) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, fx.baseURL+"/devices/"+deviceID+"/status", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("device status: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("device status want 200, got %d: %s", res.StatusCode, raw)
	}
	var st struct {
		CurrentVersion string `json:"current_version"`
		Health         struct {
			OK            bool    `json:"ok"`
			LastErrorCode *string `json:"last_error_code"`
		} `json:"health"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("decode status %q: %v", raw, err)
	}
	if st.Health.OK != wantHealthy {
		t.Fatalf("device health = %v, want %v (status: %s)", st.Health.OK, wantHealthy, raw)
	}
	if wantVersion != "" && st.CurrentVersion != wantVersion {
		t.Fatalf("device current_version = %q, want %q", st.CurrentVersion, wantVersion)
	}
}

// verifySuccessTelemetry reads the device telemetry over real HTTP and asserts a
// success event for the given deployment is present.
func verifySuccessTelemetry(t *testing.T, fx *serverFixture, opToken, deviceID, deploymentID string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, fx.baseURL+"/devices/"+deviceID+"/telemetry?event=success", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("telemetry history: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("telemetry history want 200, got %d: %s", res.StatusCode, raw)
	}
	var hist struct {
		Items []struct {
			Event        string `json:"event"`
			DeploymentID string `json:"deployment_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &hist); err != nil {
		t.Fatalf("decode telemetry %q: %v", raw, err)
	}
	found := false
	for _, it := range hist.Items {
		if it.Event == "success" && it.DeploymentID == deploymentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("no success telemetry for deployment %s found: %s", deploymentID, raw)
	}
}

// --- artifact fixture helpers (mirror the api package test fixtures) ---

func zipStored(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Store})
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func uploadMultipart(t *testing.T, file []byte, meta map[string]any) (body []byte, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "ota.zip")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := fw.Write(file); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if err := mw.WriteField("metadata", string(metaJSON)); err != nil {
		t.Fatalf("write meta part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}
