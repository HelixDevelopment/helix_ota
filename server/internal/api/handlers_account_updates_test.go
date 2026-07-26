package api

import (
	"net/http"
	"testing"
	"time"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func TestAccountsM6_RegisterDeviceForAccount_CreatesNew(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6a"
	mustCreateAccount(t, env.repo, acctID, "AcmeFleet", "acme")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	req := DeviceRegistrationRequest{
		HardwareID: "hw-device-001",
		Model:      "OrangePi5Max",
		OSType:     otaprotocol.OSAndroid,
		OSVersion:  "15",
		Group:      "staging",
	}
	w := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register device: want 201, got %d (%s)", w.Code, w.Body.String())
	}

	var resp DeviceRegistrationResponse
	env.decode(w, &resp)
	if resp.AccountID != acctID {
		t.Fatalf("account: want %s, got %s", acctID, resp.AccountID)
	}
	if resp.HardwareID != "hw-device-001" {
		t.Fatalf("hardware_id: want hw-device-001, got %s", resp.HardwareID)
	}
	if resp.DeviceID == "" {
		t.Fatal("expected non-empty device_id")
	}
	if resp.RegisteredAt.IsZero() {
		t.Fatal("expected non-zero registered_at")
	}
}

func TestAccountsM6_RegisterDeviceForAccount_Idempotent(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6b"
	mustCreateAccount(t, env.repo, acctID, "IdempotentFleet", "idemfleet")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	req := DeviceRegistrationRequest{
		HardwareID: "hw-device-002",
		Model:      "OrangePi5Max",
		OSType:     otaprotocol.OSAndroid,
		OSVersion:  "15",
	}
	w1 := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token, req)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d (%s)", w1.Code, w1.Body.String())
	}
	var first DeviceRegistrationResponse
	env.decode(w1, &first)

	w2 := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("second register (idempotent): want 200, got %d (%s)", w2.Code, w2.Body.String())
	}
	var second DeviceRegistrationResponse
	env.decode(w2, &second)

	if second.DeviceID != first.DeviceID {
		t.Fatalf("idempotent: want same device_id %s, got %s", first.DeviceID, second.DeviceID)
	}
}

func TestAccountsM6_RegisterDevice_DifferentAccountIsSeparate(t *testing.T) {
	env := newTestEnv(t)

	acctA := "acct-m6ca"
	acctB := "acct-m6cb"
	mustCreateAccount(t, env.repo, acctA, "FleetA", "fleeta")
	mustCreateAccount(t, env.repo, acctB, "FleetB", "fleetb")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctA, store.AccountRoleAdmin)
	mustCreateMembership(t, env.repo, "admin@helix.test", acctB, store.AccountRoleAdmin)

	tokenA := env.adminScopedToken(acctA, RoleAdmin, RoleOperator, RoleViewer)
	tokenB := env.adminScopedToken(acctB, RoleAdmin, RoleOperator, RoleViewer)

	req := DeviceRegistrationRequest{
		HardwareID: "hw-shared-003",
		Model:      "OrangePi5Max",
		OSType:     otaprotocol.OSAndroid,
		OSVersion:  "15",
	}
	wA := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctA+"/devices", tokenA, req)
	if wA.Code != http.StatusCreated {
		t.Fatalf("register device A: want 201, got %d (%s)", wA.Code, wA.Body.String())
	}
	var devA DeviceRegistrationResponse
	env.decode(wA, &devA)

	wB := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctB+"/devices", tokenB, req)
	if wB.Code != http.StatusCreated {
		t.Fatalf("register device B: want 201, got %d (%s)", wB.Code, wB.Body.String())
	}
	var devB DeviceRegistrationResponse
	env.decode(wB, &devB)

	if devA.DeviceID == devB.DeviceID {
		t.Fatalf("cross-account isolation: different accounts must produce different devices, got same id %s", devA.DeviceID)
	}
	if devA.AccountID != acctA {
		t.Fatalf("device A account: want %s, got %s", acctA, devA.AccountID)
	}
	if devB.AccountID != acctB {
		t.Fatalf("device B account: want %s, got %s", acctB, devB.AccountID)
	}
}

func TestAccountsM6_ListAccountUpdates_ReturnsLatestReleases(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6d"
	mustCreateAccount(t, env.repo, acctID, "UpdateFleet", "updatefleet")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	// Register device and create a release for it.
	regReq := DeviceRegistrationRequest{
		HardwareID: "hw-update-004",
		Model:      "OrangePi5Max",
		OSType:     otaprotocol.OSAndroid,
		OSVersion:  "14",
	}
	w := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token, regReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("register device: want 201, got %d (%s)", w.Code, w.Body.String())
	}

	// Create an artifact and a release for the device's target.
	artID := env.srv.newID()
	mustCreateArtifact(t, env.repo, artID, otaprotocol.OSAndroid, "OrangePi5Max", "16.0.0")
	mustCreateRelease(t, env.repo, artID, otaprotocol.OSAndroid, "OrangePi5Max", "16.0.0")

	w = env.doJSON(http.MethodGet, "/api/v1/accounts/"+acctID+"/updates", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list updates: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var updates []AccountUpdateEntry
	env.decode(w, &updates)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d: %+v", len(updates), updates)
	}
	if updates[0].Version != "16.0.0" {
		t.Fatalf("version: want 16.0.0, got %s", updates[0].Version)
	}
	if updates[0].TargetModel != "OrangePi5Max" {
		t.Fatalf("model: want OrangePi5Max, got %s", updates[0].TargetModel)
	}
}

func TestAccountsM6_ListAccountUpdates_DeduplicatesByTarget(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6e"
	mustCreateAccount(t, env.repo, acctID, "DedupFleet", "dedupfleet")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	// Register two devices with the same model.
	for _, hw := range []string{"hw-a", "hw-b"} {
		regReq := DeviceRegistrationRequest{
			HardwareID: hw,
			Model:      "OrangePi5Max",
			OSType:     otaprotocol.OSAndroid,
			OSVersion:  "14",
		}
		w := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token, regReq)
		if w.Code != http.StatusCreated {
			t.Fatalf("register device %s: want 201, got %d", hw, w.Code)
		}
	}

	artID := env.srv.newID()
	mustCreateArtifact(t, env.repo, artID, otaprotocol.OSAndroid, "OrangePi5Max", "17.0.0")
	mustCreateRelease(t, env.repo, artID, otaprotocol.OSAndroid, "OrangePi5Max", "17.0.0")

	w := env.doJSON(http.MethodGet, "/api/v1/accounts/"+acctID+"/updates", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list updates: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var updates []AccountUpdateEntry
	env.decode(w, &updates)
	if len(updates) != 1 {
		t.Fatalf("expected 1 deduplicated update, got %d: %+v", len(updates), updates)
	}
}

func TestAccountsM6_RegisterDevice_RequiresAuth(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6f"
	mustCreateAccount(t, env.repo, acctID, "AuthFleet", "authfleet")
	req := DeviceRegistrationRequest{
		HardwareID: "hw-any",
		Model:      "OrangePi5Max",
		OSType:     otaprotocol.OSAndroid,
	}
	w := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", "", req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAccountsM6_RegisterDevice_Validation(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6g"
	mustCreateAccount(t, env.repo, acctID, "ValFleet", "valfleet")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	// Missing required fields - use explicit JSON to bypass OSType serialization.
	w := env.doJSON(http.MethodPost, "/api/v1/accounts/"+acctID+"/devices", token,
		map[string]string{"model": "OrangePi5Max"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing hardware_id: want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAccountsM6_ListUpdates_EmptyForNoDevices(t *testing.T) {
	env := newTestEnv(t)

	acctID := "acct-m6h"
	mustCreateAccount(t, env.repo, acctID, "EmptyFleet", "emptyfleet")
	mustCreateMembership(t, env.repo, "admin@helix.test", acctID, store.AccountRoleAdmin)
	token := env.adminScopedToken(acctID, RoleAdmin, RoleOperator, RoleViewer)

	w := env.doJSON(http.MethodGet, "/api/v1/accounts/"+acctID+"/updates", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("empty updates: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var updates []AccountUpdateEntry
	env.decode(w, &updates)
	if len(updates) != 0 {
		t.Fatalf("expected empty updates array, got %+v", updates)
	}
}

// --- helpers ---

func (e *testEnv) adminScopedToken(accountID string, roles ...string) string {
	e.t.Helper()
	tok, err := e.signer.MintAccount("admin@helix.test", roles, accountID, 0, time.Hour, e.srv.now())
	if err != nil {
		e.t.Fatalf("mint account-scoped token: %v", err)
	}
	return tok
}

func mustCreateArtifact(t *testing.T, repo *store.MemoryRepository, id string, os otaprotocol.OSType, model, version string) {
	t.Helper()
	err := repo.CreateArtifact(t.Context(), store.Artifact{
		ArtifactID:  id,
		SHA256:      "abcdef1234567890",
		Size:        1024,
		OSType:      os,
		TargetModel: model,
		Version:     version,
		Verified:    true,
		UploadedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateArtifact(%s): %v", id, err)
	}
}

func mustCreateRelease(t *testing.T, repo *store.MemoryRepository, artifactID string, os otaprotocol.OSType, model, version string) {
	t.Helper()
	err := repo.CreateRelease(t.Context(), store.Release{
		ReleaseID:   "rel-" + version,
		ArtifactID:  artifactID,
		Version:     version,
		OSType:      os,
		TargetModel: model,
		Status:      "published",
		CreatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateRelease(%s): %v", version, err)
	}
}
