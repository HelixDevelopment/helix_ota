package api

import (
	"context"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

func TestGroupCRUDLifecycle(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Create.
	w := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "fleet-a", Description: "lab"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create group want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var g GroupView
	env.decode(w, &g)
	if g.GroupID == "" || g.Name != "fleet-a" {
		t.Fatalf("created group mismatch: %+v", g)
	}

	// Duplicate name -> 409.
	dup := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "fleet-a"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate group want 409, got %d", dup.Code)
	}

	// Get + list.
	if gw := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID, tok, nil, ""); gw.Code != http.StatusOK {
		t.Fatalf("get group want 200, got %d", gw.Code)
	}
	if nf := env.do(http.MethodGet, "/api/v1/groups/nope", tok, nil, ""); nf.Code != http.StatusNotFound {
		t.Fatalf("get unknown group want 404, got %d", nf.Code)
	}
	lw := env.do(http.MethodGet, "/api/v1/groups", tok, nil, "")
	var list GroupList
	env.decode(lw, &list)
	if len(list.Items) != 1 {
		t.Fatalf("list groups want 1, got %d", len(list.Items))
	}

	// Update.
	uw := env.doJSON(http.MethodPatch, "/api/v1/groups/"+g.GroupID, tok, GroupUpdate{Name: "fleet-a", Description: "field"})
	if uw.Code != http.StatusOK {
		t.Fatalf("update group want 200, got %d (%s)", uw.Code, uw.Body.String())
	}

	// Members: batch add (with not_found + already_member buckets), list, remove.
	// dev-1 is registered; ghost-dev is not (lands in not_found).
	if err := env.repo.CreateDevice(context.Background(), store.Device{DeviceID: "dev-1",
		HardwareID: "HW-D1", Model: "OrangePi5Max", OSType: otaprotocol.OSAndroid, RegisteredAt: env.srv.now()}); err != nil {
		t.Fatalf("register dev-1: %v", err)
	}
	mw := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.GroupID+"/members", tok, MemberAdd{DeviceIDs: []string{"dev-1", "ghost-dev"}})
	if mw.Code != http.StatusOK {
		t.Fatalf("batch add member want 200, got %d (%s)", mw.Code, mw.Body.String())
	}
	var res MemberAddResult
	env.decode(mw, &res)
	if len(res.Added) != 1 || res.Added[0] != "dev-1" || len(res.NotFound) != 1 || res.NotFound[0] != "ghost-dev" {
		t.Fatalf("batch add result mismatch: %+v", res)
	}
	// Re-add dev-1 -> already_member.
	mw2 := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.GroupID+"/members", tok, MemberAdd{DeviceIDs: []string{"dev-1"}})
	var res2 MemberAddResult
	env.decode(mw2, &res2)
	if mw2.Code != http.StatusOK || len(res2.AlreadyMember) != 1 || res2.AlreadyMember[0] != "dev-1" {
		t.Fatalf("re-add result want already_member dev-1, got %d %+v", mw2.Code, res2)
	}
	mlw := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID+"/members", tok, nil, "")
	var members GroupMembers
	env.decode(mlw, &members)
	if len(members.DeviceIDs) != 1 || members.DeviceIDs[0] != "dev-1" {
		t.Fatalf("members mismatch: %+v", members)
	}
	// Batch add to unknown group -> 404.
	if bad := env.doJSON(http.MethodPost, "/api/v1/groups/nope/members", tok, MemberAdd{DeviceIDs: []string{"dev-1"}}); bad.Code != http.StatusNotFound {
		t.Fatalf("add member to unknown group want 404, got %d", bad.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/v1/groups/"+g.GroupID+"/members/dev-1", tok, nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("remove member want 204, got %d", rw.Code)
	}

	// Delete group, then it's gone.
	if dw := env.do(http.MethodDelete, "/api/v1/groups/"+g.GroupID, tok, nil, ""); dw.Code != http.StatusNoContent {
		t.Fatalf("delete group want 204, got %d", dw.Code)
	}
	if gone := env.do(http.MethodGet, "/api/v1/groups/"+g.GroupID, tok, nil, ""); gone.Code != http.StatusNotFound {
		t.Fatalf("deleted group should be 404, got %d", gone.Code)
	}
}

func TestGroupRBAC(t *testing.T) {
	env := newTestEnv(t)
	viewer, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	operator, _ := env.signer.Mint("op@helix.test", []string{RoleOperator}, env.srv.cfg.AccessTokenTTL, env.srv.now())

	// Viewer cannot create.
	if w := env.doJSON(http.MethodPost, "/api/v1/groups", viewer, GroupCreate{Name: "g"}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create group want 403, got %d", w.Code)
	}
	// Operator creates, but cannot DELETE (admin-only).
	cw := env.doJSON(http.MethodPost, "/api/v1/groups", operator, GroupCreate{Name: "g"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("operator create want 201, got %d", cw.Code)
	}
	var g GroupView
	env.decode(cw, &g)
	if dw := env.do(http.MethodDelete, "/api/v1/groups/"+g.GroupID, operator, nil, ""); dw.Code != http.StatusForbidden {
		t.Fatalf("operator delete group want 403 (admin-only), got %d", dw.Code)
	}
}
