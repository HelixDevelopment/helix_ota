package deviceemu_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
)

// TestRecallRecoveryE2E is the headline FAILURE -> operator-RECALL(forward-fix)
// -> device-RECOVERY end-to-end proof on the Tier-1 emulator. A real control
// plane boots in-process (httptest); an operator stages a release + deployment;
// the emulated device applies it, then hits a post-apply health-check failure;
// the operator recalls the broken deployment FORWARD to a higher-versioned
// fix release; the device re-checks, picks up the recall deployment, recovers
// to the fix version, and reports a healthy success lifecycle. Every step is
// asserted against REAL response data read back over real HTTP (§11.4 anti-bluff;
// the recovery is proven end-to-end per §11.4.107, not by a single green line).
//
// Anti-downgrade note (FACT): the update-check invariant only offers a device a
// version STRICTLY GREATER than its current. The device fails AT v1.1.0, so the
// forward-fix release MUST be HIGHER (v1.2.0) for the device to pick it up on
// re-check — staging the fix below 1.1.0 would (correctly) yield a 204 and no
// recovery.
func TestRecallRecoveryE2E(t *testing.T) {
	fx := bootServer(t)
	ctx := context.Background()
	opToken := fx.operatorLogin(t)
	const group = "recall-recovery-fleet"

	// --- (a) operator stages release v1.1.0 + all-targets deployment D1 ------
	relV1, deploymentD1 := fx.stageDeployment(t, opToken, "1.1.0", group)
	if relV1 != "1.1.0" {
		t.Fatalf("staged release version = %q, want 1.1.0", relV1)
	}
	// We also need the v1.1.0 release_id (the from-release the recall records).
	relID110 := fx.releaseIDForVersion(t, opToken, "1.1.0")

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        fx.baseURL,
		AdminUser:      fx.adminUser,
		AdminPass:      fx.adminPass,
		HardwareID:     "rk3588-RECALL01",
		Model:          fxModel,
		OSType:         string(otaprotocol.OSAndroid),
		CurrentVersion: "1.0.0",
		Group:          group,
		// DeploymentID intentionally omitted on the happy path: the device
		// self-serves the id from its update offer. The failure path below needs
		// an explicit fallback id (rollback is not tied to a fresh offer), so we
		// stamp it onto the device's config after we know D1.
		DeploymentID: deploymentD1,
	})
	if err != nil {
		t.Fatalf("new device: %v", err)
	}

	// Device on 1.0.0 runs a cycle: register -> 200 offer v1.1.0 -> apply.
	out1, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once (apply v1.1.0): %v", err)
	}
	if !out1.Applied || out1.FromVersion != "1.0.0" || out1.ToVersion != "1.1.0" {
		t.Fatalf("step-a apply outcome mismatch: %+v", out1)
	}
	if out1.TelemetryAccepted < 1 || out1.TelemetryRejected != 0 {
		t.Fatalf("step-a telemetry want accepted>=1 rejected=0, got %+v", out1)
	}
	if !dev.Healthy() {
		t.Fatalf("device should be healthy after applying v1.1.0")
	}
	if got := dev.CurrentVersion(); got != "1.1.0" {
		t.Fatalf("device version after apply = %q, want 1.1.0", got)
	}
	deviceID := dev.DeviceID()
	if deviceID == "" {
		t.Fatalf("device id empty after run-once")
	}
	// Operator cross-check: device is on 1.1.0 and healthy.
	verifyDeviceStatus(t, fx, opToken, deviceID, "1.1.0", true)

	// --- (b) device hits a post-apply failure --------------------------------
	// ReportFailure marks the device unhealthy and does NOT advance the version.
	ackFail, err := dev.ReportFailure(ctx, "post_apply_health_check_failed")
	if err != nil {
		t.Fatalf("report failure: %v", err)
	}
	if ackFail.Accepted != 1 || ackFail.Rejected != 0 {
		t.Fatalf("failure telemetry want accepted=1 rejected=0, got %+v", ackFail)
	}
	if dev.Healthy() {
		t.Fatalf("device should be unhealthy after the reported failure")
	}
	if got := dev.CurrentVersion(); got != "1.1.0" {
		t.Fatalf("device version after failure = %q, want unchanged 1.1.0 (no further advance)", got)
	}
	// Operator cross-check: server marks the device unhealthy.
	verifyDeviceStatus(t, fx, opToken, deviceID, "", false)
	// The failure event is persisted server-side, stamped with D1.
	verifyFailureTelemetry(t, fx, opToken, deviceID, deploymentD1, "post_apply_health_check_failed")

	// --- (c) operator stages a FORWARD-FIX release v1.2.0 + recalls D1 -------
	// Publish ONLY the release (no separate deployment): the recall endpoint
	// itself supersedes D1 and creates the new active deployment of the fix
	// release. Creating a second all-targets deployment up front would (correctly)
	// 409 — an active deployment already targets this set.
	relID120 := fx.stageReleaseOnly(t, opToken, "1.2.0")

	// Recall D1 forward to the v1.2.0 release. 201 + a rollback_history row.
	recallBody, _ := json.Marshal(map[string]any{
		"to_release_id": relID120,
		"reason":        "post_apply_health_check_failed on 1.1.0; forward-fix to 1.2.0",
	})
	var recall struct {
		ID                 string            `json:"id"`
		DeploymentID       string            `json:"deployment_id"`
		Kind               string            `json:"kind"`
		FromReleaseID      string            `json:"from_release_id"`
		ToReleaseID        string            `json:"to_release_id"`
		RecallDeploymentID string            `json:"recall_deployment_id"`
		Details            map[string]string `json:"details"`
	}
	fx.postJSON(t, opToken, "/deployments/"+deploymentD1+"/recall", recallBody, http.StatusCreated, &recall)
	if recall.Kind != "rollback" {
		t.Fatalf("recall kind = %q, want rollback", recall.Kind)
	}
	if recall.FromReleaseID != relID110 {
		t.Fatalf("recall from_release_id = %q, want v1.1.0 release %q", recall.FromReleaseID, relID110)
	}
	if recall.ToReleaseID != relID120 {
		t.Fatalf("recall to_release_id = %q, want v1.2.0 release %q", recall.ToReleaseID, relID120)
	}
	if recall.RecallDeploymentID == "" {
		t.Fatalf("recall created no recall_deployment_id")
	}
	if recall.Details["mode"] != "forward-fix" {
		t.Fatalf("recall details.mode = %q, want forward-fix", recall.Details["mode"])
	}
	recallDeploymentID := recall.RecallDeploymentID

	// Cross-check the rollback_history over GET /deployments/{D1}/rollbacks.
	verifyRollbackHistory(t, fx, opToken, deploymentD1, relID110, relID120, recallDeploymentID)

	// --- (d) device recovers: re-check -> v1.2.0 offer -> apply -> healthy ---
	out2, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once (recovery to v1.2.0): %v", err)
	}
	if !out2.Applied {
		t.Fatalf("recovery cycle should apply the fix: %+v", out2)
	}
	if out2.FromVersion != "1.1.0" || out2.ToVersion != "1.2.0" {
		t.Fatalf("recovery outcome version mismatch: %+v", out2)
	}
	if out2.OfferedVersion != "1.2.0" {
		t.Fatalf("recovery offered_version = %q, want 1.2.0", out2.OfferedVersion)
	}
	// The recovery offer must carry the NEW recall deployment id (the device
	// self-serves it from the offer — proving the recall deployment is the active
	// one the control plane now resolves for this device).
	if out2.DeploymentID != recallDeploymentID {
		t.Fatalf("recovery offer deployment_id = %q, want recall deployment %q", out2.DeploymentID, recallDeploymentID)
	}
	if out2.TelemetryAccepted < 1 || out2.TelemetryRejected != 0 {
		t.Fatalf("recovery telemetry want accepted>=1 rejected=0, got %+v", out2)
	}
	if !out2.Healthy || !dev.Healthy() {
		t.Fatalf("device should be healthy again after recovery: %+v", out2)
	}
	if got := dev.CurrentVersion(); got != "1.2.0" {
		t.Fatalf("device version after recovery = %q, want 1.2.0", got)
	}
	// Operator cross-check: device is on 1.2.0 and healthy again.
	verifyDeviceStatus(t, fx, opToken, deviceID, "1.2.0", true)
	// The recovery success lifecycle is persisted server-side, stamped with the
	// recall deployment id.
	verifySuccessTelemetry(t, fx, opToken, deviceID, recallDeploymentID)

	// --- (e) device re-checks a third time -> 204 on-target ------------------
	out3, err := dev.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once (on-target recheck): %v", err)
	}
	if !out3.OnTarget || out3.Applied {
		t.Fatalf("third cycle should be on-target no-apply (204): %+v", out3)
	}
}

// releaseIDForVersion reads the release list over real HTTP and returns the
// release_id matching the given version (the recall records from/to by
// release_id, which stageDeployment does not surface).
func (f *serverFixture) releaseIDForVersion(t *testing.T, opToken, version string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, f.baseURL+"/releases", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list releases: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list releases want 200, got %d: %s", res.StatusCode, raw)
	}
	var list struct {
		Items []struct {
			ReleaseID string `json:"release_id"`
			Version   string `json:"version"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode releases %q: %v", raw, err)
	}
	for _, it := range list.Items {
		if it.Version == version {
			return it.ReleaseID
		}
	}
	t.Fatalf("no release with version %q found: %s", version, raw)
	return ""
}

// stageReleaseOnly uploads + signs a real artifact and publishes a release over
// real HTTP, WITHOUT creating a deployment. The recall endpoint creates the
// recall deployment itself, so staging a deployment here would 409 against the
// still-active prior deployment. It returns the new release_id.
func (f *serverFixture) stageReleaseOnly(t *testing.T, opToken, version string) string {
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

	relBody, _ := json.Marshal(map[string]any{
		"artifact_id": artifactID, "version": version,
		"os": string(otaprotocol.OSAndroid), "target_model": fxModel,
	})
	var rel struct {
		ReleaseID string `json:"release_id"`
	}
	f.postJSON(t, opToken, "/releases", relBody, http.StatusCreated, &rel)
	if rel.ReleaseID == "" {
		t.Fatalf("release-only 201 but no release_id")
	}
	return rel.ReleaseID
}

// verifyFailureTelemetry reads the device telemetry filtered to failure events
// and asserts a failure for the given deployment + error code is present.
func verifyFailureTelemetry(t *testing.T, fx *serverFixture, opToken, deviceID, deploymentID, wantErrorCode string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, fx.baseURL+"/devices/"+deviceID+"/telemetry?event=failure", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failure telemetry history: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("failure telemetry history want 200, got %d: %s", res.StatusCode, raw)
	}
	var hist struct {
		Items []struct {
			Event        string  `json:"event"`
			DeploymentID string  `json:"deployment_id"`
			ErrorCode    *string `json:"error_code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &hist); err != nil {
		t.Fatalf("decode failure telemetry %q: %v", raw, err)
	}
	found := false
	for _, it := range hist.Items {
		if it.Event == "failure" && it.DeploymentID == deploymentID {
			if it.ErrorCode != nil && *it.ErrorCode == wantErrorCode {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no failure telemetry for deployment %s with error_code %q found: %s", deploymentID, wantErrorCode, raw)
	}
}

// verifyRollbackHistory reads GET /deployments/{id}/rollbacks over real HTTP and
// asserts a forward-fix rollback row exists with the expected from/to release
// ids and recall deployment id.
func verifyRollbackHistory(t *testing.T, fx *serverFixture, opToken, deploymentID, wantFrom, wantTo, wantRecallDep string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, fx.baseURL+"/deployments/"+deploymentID+"/rollbacks", nil)
	req.Header.Set("Authorization", "Bearer "+opToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("rollback history: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rollback history want 200, got %d: %s", res.StatusCode, raw)
	}
	var list struct {
		Items []struct {
			Kind               string            `json:"kind"`
			FromReleaseID      string            `json:"from_release_id"`
			ToReleaseID        string            `json:"to_release_id"`
			RecallDeploymentID string            `json:"recall_deployment_id"`
			Details            map[string]string `json:"details"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode rollback history %q: %v", raw, err)
	}
	for _, it := range list.Items {
		if it.Kind == "rollback" && it.FromReleaseID == wantFrom && it.ToReleaseID == wantTo &&
			it.RecallDeploymentID == wantRecallDep && it.Details["mode"] == "forward-fix" {
			return
		}
	}
	t.Fatalf("no forward-fix rollback row (from=%s to=%s recall=%s) in history: %s", wantFrom, wantTo, wantRecallDep, raw)
}
