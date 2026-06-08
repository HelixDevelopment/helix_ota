package api

import (
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// listAudit fetches the audit log with the given token.
func listAudit(t *testing.T, env *testEnv, token string) (*AuditLogList, int) {
	t.Helper()
	w := env.do(http.MethodGet, "/api/v1/audit", token, nil, "")
	if w.Code != http.StatusOK {
		return nil, w.Code
	}
	var list AuditLogList
	env.decode(w, &list)
	return &list, w.Code
}

// TestAuditRecordsSuccessfulMutation proves a successful mutating action
// (device register) is recorded with the derived action + resource, and shows
// up in GET /audit.
func TestAuditRecordsSuccessfulMutation(t *testing.T) {
	env := newTestEnv(t)
	registerDevice(t, env, DeviceRegistration{HardwareID: "audit-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})

	list, code := listAudit(t, env, env.adminToken())
	if code != http.StatusOK {
		t.Fatalf("GET /audit want 200, got %d", code)
	}
	if len(list.Items) != 1 {
		t.Fatalf("want 1 audit entry, got %d (%+v)", len(list.Items), list.Items)
	}
	e := list.Items[0]
	if e.Action != "DEVICE_REGISTER" || e.ResourceType != "device" {
		t.Fatalf("audit entry action/resource mismatch: %+v", e)
	}
	if e.Actor != "admin@helix.test" {
		t.Fatalf("audit actor mismatch: %q", e.Actor)
	}
}

// TestAuditSkipsReadsAndFailures proves GET requests and failed mutations are
// NOT audited.
func TestAuditSkipsReadsAndFailures(t *testing.T) {
	env := newTestEnv(t)

	// A read (GET /releases) — not audited.
	env.do(http.MethodGet, "/api/v1/releases", env.adminToken(), nil, "")

	// A failed mutation: a viewer uploading an artifact -> 403, must not audit.
	viewerTok, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	file := zipStored(t, []byte("p"))
	body, ct := uploadMultipart(t, file, env.validMeta(file, "1.1.0"))
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", viewerTok, body, ct)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer upload want 403, got %d", w.Code)
	}

	list, code := listAudit(t, env, env.adminToken())
	if code != http.StatusOK {
		t.Fatalf("GET /audit want 200, got %d", code)
	}
	if len(list.Items) != 0 {
		t.Fatalf("reads + failed mutations must not be audited; got %d entries (%+v)", len(list.Items), list.Items)
	}
}

// TestAuditReadIsAdminOnly proves GET /audit is forbidden for non-admins.
func TestAuditReadIsAdminOnly(t *testing.T) {
	env := newTestEnv(t)
	viewerTok, _ := env.signer.Mint("v@helix.test", []string{RoleViewer, RoleOperator}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	w := env.do(http.MethodGet, "/api/v1/audit", viewerTok, nil, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin GET /audit want 403, got %d", w.Code)
	}
}

// TestDeriveAuditAction unit-checks the action/resource derivation from the
// route template (no ids leak into the action string).
func TestDeriveAuditAction(t *testing.T) {
	cases := []struct {
		method, path        string
		wantAction, wantRes string
	}{
		{http.MethodPost, "/api/v1/devices/register", "DEVICE_REGISTER", "device"},
		{http.MethodPost, "/api/v1/artifacts/upload", "ARTIFACT_UPLOAD", "artifact"},
		{http.MethodPost, "/api/v1/releases", "RELEASE_CREATE", "release"},
		{http.MethodPost, "/api/v1/deployments", "DEPLOYMENT_CREATE", "deployment"},
		{http.MethodGet, "/api/v1/releases", "RELEASE_ACTION", "release"}, // verb for GET (unused by middleware)
	}
	for _, tc := range cases {
		gotA, gotR := deriveAuditAction(tc.method, tc.path)
		if gotA != tc.wantAction || gotR != tc.wantRes {
			t.Fatalf("%s %s: got (%s,%s) want (%s,%s)", tc.method, tc.path, gotA, gotR, tc.wantAction, tc.wantRes)
		}
	}
}
