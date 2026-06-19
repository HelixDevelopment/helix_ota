package api

import (
	"fmt"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

func registerDevice(t *testing.T, env *testEnv, reg DeviceRegistration) DeviceRegistered {
	t.Helper()
	w := env.doJSON(http.MethodPost, "/api/v1/devices/register", env.adminToken(), reg)
	if w.Code != http.StatusCreated {
		t.Fatalf("register want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var resp DeviceRegistered
	env.decode(w, &resp)
	return resp
}

func TestDeviceRegisterAndStatus(t *testing.T) {
	env := newTestEnv(t)

	reg := DeviceRegistration{
		HardwareID:     "rk3588-AABBCCDD",
		Model:          "OrangePi5Max",
		OS:             otaprotocol.OSAndroid,
		OSVersion:      "15",
		CurrentVersion: "1.0.0",
		Group:          "field-fleet-a",
	}
	resp := registerDevice(t, env, reg)
	if resp.DeviceID == "" || resp.DeviceToken == "" {
		t.Fatalf("expected device id + token, got %+v", resp)
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("token_type want Bearer, got %q", resp.TokenType)
	}

	// Admin can read status.
	w := env.do(http.MethodGet, "/api/v1/devices/"+resp.DeviceID+"/status", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var st DeviceStatus
	env.decode(w, &st)
	if st.DeviceID != resp.DeviceID || st.HardwareID != reg.HardwareID {
		t.Fatalf("status mismatch: %+v", st)
	}
	if st.UpdateState != "idle" {
		t.Fatalf("fresh device update_state want idle, got %q", st.UpdateState)
	}
}

func TestDeviceRegisterValidation(t *testing.T) {
	env := newTestEnv(t)
	// Raw JSON bodies so an absent/empty os field exercises handler validation
	// (the typed OSType marshaler refuses an empty enum, which is itself a
	// separate guarantee).
	tests := []struct {
		name string
		body string
	}{
		{"missing hardware_id", `{"model":"M","os":"android"}`},
		{"missing model", `{"hardware_id":"hw","os":"android"}`},
		{"missing os", `{"hardware_id":"hw","model":"M"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(http.MethodPost, "/api/v1/devices/register", env.adminToken(), []byte(tc.body), "application/json")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestDeviceRegisterConflict(t *testing.T) {
	env := newTestEnv(t)
	reg := DeviceRegistration{HardwareID: "dup-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid}
	_ = registerDevice(t, env, reg)

	// Same hardware id, second registration mints a new identity -> conflict.
	w := env.doJSON(http.MethodPost, "/api/v1/devices/register", env.adminToken(), reg)
	if w.Code != http.StatusConflict {
		t.Fatalf("dup hardware_id want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeConflict {
		t.Fatalf("want CONFLICT, got %s", got)
	}
}

func TestDeviceRegisterIdempotent(t *testing.T) {
	env := newTestEnv(t)
	reg := DeviceRegistration{HardwareID: "idem-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid}
	body := mustJSON(t, reg)

	r1 := newAuthedReq(t, http.MethodPost, "/api/v1/devices/register", env.adminToken(), body)
	r1.Header.Set("Idempotency-Key", "key-123")
	w1 := serveReq(env, r1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first idempotent register want 201, got %d", w1.Code)
	}
	var first DeviceRegistered
	env.decode(w1, &first)

	r2 := newAuthedReq(t, http.MethodPost, "/api/v1/devices/register", env.adminToken(), body)
	r2.Header.Set("Idempotency-Key", "key-123")
	w2 := serveReq(env, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("replayed idempotent register want 200, got %d (%s)", w2.Code, w2.Body.String())
	}
	var second DeviceRegistered
	env.decode(w2, &second)
	if first.DeviceID != second.DeviceID {
		t.Fatalf("idempotent replay returned different id: %s vs %s", first.DeviceID, second.DeviceID)
	}
}

func TestDeviceStatusOwnership(t *testing.T) {
	env := newTestEnv(t)
	dev := registerDevice(t, env, DeviceRegistration{HardwareID: "own-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	other := registerDevice(t, env, DeviceRegistration{HardwareID: "other-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})

	// A device token may read its own status.
	ownTok := env.deviceToken(dev.DeviceID)
	w := env.do(http.MethodGet, "/api/v1/devices/"+dev.DeviceID+"/status", ownTok, nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("device reading own status want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// A device token may NOT read another device's status.
	w = env.do(http.MethodGet, "/api/v1/devices/"+other.DeviceID+"/status", ownTok, nil, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("device reading other status want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDeviceStatusNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/devices/does-not-exist/status", env.adminToken(), nil, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestDeviceList(t *testing.T) {
	env := newTestEnv(t)

	// Register 3 devices.
	reg1 := DeviceRegistration{HardwareID: "hw-001", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid,
		OSVersion: "15", CurrentVersion: "1.0.0"}
	reg2 := DeviceRegistration{HardwareID: "hw-002", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid,
		OSVersion: "15", CurrentVersion: "1.0.0"}
	reg3 := DeviceRegistration{HardwareID: "hw-003", Model: "RK3588", OS: otaprotocol.OSAndroid,
		OSVersion: "14", CurrentVersion: "1.0.0"}
	registerDevice(t, env, reg1)
	registerDevice(t, env, reg2)
	registerDevice(t, env, reg3)

	w := env.do(http.MethodGet, "/api/v1/devices", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var list DeviceList
	env.decode(w, &list)
	if len(list.Items) != 3 {
		t.Fatalf("want 3 devices, got %d", len(list.Items))
	}
	if list.NextCursor != nil {
		t.Fatalf("expected no next_cursor for 3 devices with limit 50, got %s", *list.NextCursor)
	}
}

func TestDeviceListFilterByOS(t *testing.T) {
	env := newTestEnv(t)

	// Register devices with different OS types.
	reg1 := DeviceRegistration{HardwareID: "hw-010", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid}
	reg2 := DeviceRegistration{HardwareID: "hw-011", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid}
	registerDevice(t, env, reg1)
	registerDevice(t, env, reg2)

	// No Linux devices exist; filter should return empty.
	w := env.do(http.MethodGet, "/api/v1/devices?os=linux", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var list DeviceList
	env.decode(w, &list)
	if len(list.Items) != 0 {
		t.Fatalf("want 0 linux devices, got %d", len(list.Items))
	}
}

func TestDeviceListPagination(t *testing.T) {
	env := newTestEnv(t)

	// Register 5 devices.
	for i := 0; i < 5; i++ {
		hw := fmt.Sprintf("hw-pag-%02d", i+1)
		registerDevice(t, env, DeviceRegistration{HardwareID: hw, Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	}

	// First page: limit=2.
	w := env.do(http.MethodGet, "/api/v1/devices?limit=2", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var page1 DeviceList
	env.decode(w, &page1)
	if len(page1.Items) != 2 {
		t.Fatalf("page 1 want 2 items, got %d", len(page1.Items))
	}
	if page1.NextCursor == nil {
		t.Fatal("page 1 expected next_cursor, got nil")
	}

	// Second page.
	w = env.do(http.MethodGet, "/api/v1/devices?limit=2&cursor="+*page1.NextCursor, env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("page 2 want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var page2 DeviceList
	env.decode(w, &page2)
	if len(page2.Items) != 2 {
		t.Fatalf("page 2 want 2 items, got %d", len(page2.Items))
	}

	// Third page — should have 1 item and no cursor.
	w = env.do(http.MethodGet, "/api/v1/devices?limit=2&cursor="+*page2.NextCursor, env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("page 3 want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var page3 DeviceList
	env.decode(w, &page3)
	if len(page3.Items) != 1 {
		t.Fatalf("page 3 want 1 item, got %d", len(page3.Items))
	}
	if page3.NextCursor != nil {
		t.Fatalf("page 3 expected no next_cursor, got %s", *page3.NextCursor)
	}
}

func TestDeviceByHardware(t *testing.T) {
	env := newTestEnv(t)

	reg := DeviceRegistration{HardwareID: "hw-lookup", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid}
	resp := registerDevice(t, env, reg)

	w := env.do(http.MethodGet, "/api/v1/devices/by-hardware/hw-lookup", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("by-hardware want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var item DeviceListItem
	env.decode(w, &item)
	if item.DeviceID != resp.DeviceID {
		t.Fatalf("device_id want %s, got %s", resp.DeviceID, item.DeviceID)
	}
	if item.HardwareID != "hw-lookup" {
		t.Fatalf("hardware_id want hw-lookup, got %s", item.HardwareID)
	}
}

func TestDeviceByHardwareNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodGet, "/api/v1/devices/by-hardware/unknown-hw", env.adminToken(), nil, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
