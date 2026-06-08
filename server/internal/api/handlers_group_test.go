package api

import (
	"net/http"
	"testing"
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
	if g.ID == "" || g.Name != "fleet-a" {
		t.Fatalf("created group mismatch: %+v", g)
	}

	// Duplicate name -> 409.
	dup := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "fleet-a"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate group want 409, got %d", dup.Code)
	}

	// Get + list.
	if gw := env.do(http.MethodGet, "/api/v1/groups/"+g.ID, tok, nil, ""); gw.Code != http.StatusOK {
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
	uw := env.doJSON(http.MethodPatch, "/api/v1/groups/"+g.ID, tok, GroupUpdate{Name: "fleet-a", Description: "field"})
	if uw.Code != http.StatusOK {
		t.Fatalf("update group want 200, got %d (%s)", uw.Code, uw.Body.String())
	}

	// Members: add (idempotent), list, remove.
	if mw := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.ID+"/members", tok, MemberAdd{DeviceID: "dev-1"}); mw.Code != http.StatusNoContent {
		t.Fatalf("add member want 204, got %d (%s)", mw.Code, mw.Body.String())
	}
	if mw2 := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.ID+"/members", tok, MemberAdd{DeviceID: "dev-1"}); mw2.Code != http.StatusNoContent {
		t.Fatalf("idempotent add member want 204, got %d", mw2.Code)
	}
	mlw := env.do(http.MethodGet, "/api/v1/groups/"+g.ID+"/members", tok, nil, "")
	var members GroupMembers
	env.decode(mlw, &members)
	if len(members.DeviceIDs) != 1 || members.DeviceIDs[0] != "dev-1" {
		t.Fatalf("members mismatch: %+v", members)
	}
	// Add member to unknown group -> 404.
	if bad := env.doJSON(http.MethodPost, "/api/v1/groups/nope/members", tok, MemberAdd{DeviceID: "dev-1"}); bad.Code != http.StatusNotFound {
		t.Fatalf("add member to unknown group want 404, got %d", bad.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/v1/groups/"+g.ID+"/members/dev-1", tok, nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("remove member want 204, got %d", rw.Code)
	}

	// Delete group, then it's gone.
	if dw := env.do(http.MethodDelete, "/api/v1/groups/"+g.ID, tok, nil, ""); dw.Code != http.StatusNoContent {
		t.Fatalf("delete group want 204, got %d", dw.Code)
	}
	if gone := env.do(http.MethodGet, "/api/v1/groups/"+g.ID, tok, nil, ""); gone.Code != http.StatusNotFound {
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
	if dw := env.do(http.MethodDelete, "/api/v1/groups/"+g.ID, operator, nil, ""); dw.Code != http.StatusForbidden {
		t.Fatalf("operator delete group want 403 (admin-only), got %d", dw.Code)
	}
}
