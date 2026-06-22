// Package api — coverage gap tests for uncovered pure / simple functions.
//
// These are minimal unit tests that cover error-return-path functions and
// simple utility/middleware functions the existing suite does not reach.
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ---------------------------------------------------------------------------
// isRoleAtLeast — standalone project-role rank comparison.
// ---------------------------------------------------------------------------

func TestIsRoleAtLeast(t *testing.T) {
	tests := []struct {
		actual   store.ProjectRole
		required store.ProjectRole
		want     bool
	}{
		{store.ProjectRoleViewer, store.ProjectRoleViewer, true},
		{store.ProjectRoleViewer, store.ProjectRoleOperator, false},
		{store.ProjectRoleViewer, store.ProjectRoleAdmin, false},
		{store.ProjectRoleOperator, store.ProjectRoleViewer, true},
		{store.ProjectRoleOperator, store.ProjectRoleOperator, true},
		{store.ProjectRoleOperator, store.ProjectRoleAdmin, false},
		{store.ProjectRoleAdmin, store.ProjectRoleViewer, true},
		{store.ProjectRoleAdmin, store.ProjectRoleOperator, true},
		{store.ProjectRoleAdmin, store.ProjectRoleAdmin, true},
		// Unknown role ranks below everything.
		{"unknown", store.ProjectRoleViewer, false},
	}
	for _, tc := range tests {
		got := isRoleAtLeast(tc.actual, tc.required)
		if got != tc.want {
			t.Errorf("isRoleAtLeast(%q, %q) = %v, want %v", tc.actual, tc.required, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// varyMiddleware — sets Vary: Accept-Encoding.
// ---------------------------------------------------------------------------

func TestVaryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(varyMiddleware())
	r.GET("/x", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	if v := w.Header().Get("Vary"); v != "Accept-Encoding" {
		t.Fatalf("expected Vary: Accept-Encoding, got %q", v)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// newRandomID — returns a non-empty hex string.
// ---------------------------------------------------------------------------

func TestNewRandomID(t *testing.T) {
	id := newRandomID()
	if id == "" {
		t.Fatal("newRandomID() returned empty")
	}
	if len(id) != 32 { // 16 bytes = 32 hex chars
		t.Fatalf("expected 32 hex chars, got %d (%q)", len(id), id)
	}
}

func TestNewRandomID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newRandomID()
		if ids[id] {
			t.Fatalf("duplicate id %q", id)
		}
		ids[id] = true
	}
}

// ---------------------------------------------------------------------------
// toDeviceListItem — verify pointer fields for TargetVersion and LastSeen.
// ---------------------------------------------------------------------------

func TestToDeviceListItem(t *testing.T) {
	now := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)

	// Device with both optional fields set.
	d := store.Device{
		DeviceID:      "dev-001",
		HardwareID:    "rk3588",
		TargetVersion: "2.0.0",
		LastSeen:      now,
		HealthOK:      true,
		UpdateState:   "idle",
		ActiveSlot:    "A",
		RegisteredAt:  now,
	}
	item := toDeviceListItem(d)
	if item.DeviceID != "dev-001" {
		t.Fatalf("expected dev-001, got %q", item.DeviceID)
	}
	if item.TargetVersion == nil {
		t.Fatal("expected non-nil TargetVersion")
	}
	if *item.TargetVersion != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %q", *item.TargetVersion)
	}
	if item.LastSeen == nil {
		t.Fatal("expected non-nil LastSeen")
	}
	if !item.LastSeen.Equal(now) {
		t.Fatalf("expected %v, got %v", now, item.LastSeen)
	}

	// Device with neither optional field set.
	d2 := store.Device{
		DeviceID:     "dev-002",
		HardwareID:   "rk3588",
		HealthOK:     true,
		UpdateState:  "idle",
		RegisteredAt: now,
	}
	item2 := toDeviceListItem(d2)
	if item2.TargetVersion != nil {
		t.Fatal("expected nil TargetVersion when empty")
	}
	if item2.LastSeen != nil {
		t.Fatal("expected nil LastSeen when zero")
	}
}
