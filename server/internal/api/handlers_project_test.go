package api

import (
	"net/http"
	"testing"
)

func TestProjectCreate(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "test-project", Description: "integration tests"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create project want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var p ProjectResponse
	env.decode(w, &p)
	if p.ProjectID == "" || p.Name != "test-project" {
		t.Fatalf("created project mismatch: %+v", p)
	}
	if p.Description != "integration tests" {
		t.Fatalf("description want 'integration tests', got %q", p.Description)
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal("created_at / updated_at should be set")
	}
}

func TestProjectCreateDuplicate(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "duplicate"})
	if w.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d", w.Code)
	}
	dup := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "duplicate"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate name want 409, got %d (%s)", dup.Code, dup.Body.String())
	}
}

func TestProjectList(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Create two projects.
	if w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "alpha"}); w.Code != http.StatusCreated {
		t.Fatalf("create alpha: %d", w.Code)
	}
	if w := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "beta"}); w.Code != http.StatusCreated {
		t.Fatalf("create beta: %d", w.Code)
	}

	lw := env.do(http.MethodGet, "/api/v1/projects", tok, nil, "")
	if lw.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", lw.Code)
	}
	var body struct {
		Items []ProjectResponse `json:"items"`
	}
	env.decode(lw, &body)
	if len(body.Items) != 2 {
		t.Fatalf("list want 2 items, got %d", len(body.Items))
	}
	names := make(map[string]bool)
	for _, p := range body.Items {
		names[p.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Fatalf("expected both alpha and beta, got %v", body.Items)
	}
}

func TestProjectGet(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Create then get.
	cw := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "gettest"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", cw.Code)
	}
	var created ProjectResponse
	env.decode(cw, &created)

	gw := env.do(http.MethodGet, "/api/v1/projects/"+created.ProjectID, tok, nil, "")
	if gw.Code != http.StatusOK {
		t.Fatalf("get project want 200, got %d (%s)", gw.Code, gw.Body.String())
	}
	var got ProjectResponse
	env.decode(gw, &got)
	if got.Name != "gettest" || got.ProjectID != created.ProjectID {
		t.Fatalf("get mismatch: %+v", got)
	}
}

func TestProjectNotFound(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	gw := env.do(http.MethodGet, "/api/v1/projects/nonexistent", tok, nil, "")
	if gw.Code != http.StatusNotFound {
		t.Fatalf("get nonexistent project want 404, got %d", gw.Code)
	}
}

func TestProjectUpdate(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	cw := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "updatable", Description: "old desc"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", cw.Code)
	}
	var created ProjectResponse
	env.decode(cw, &created)

	uw := env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, UpdateProjectRequest{Name: "updatable", Description: "new desc"})
	if uw.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d (%s)", uw.Code, uw.Body.String())
	}
	var updated ProjectResponse
	env.decode(uw, &updated)
	if updated.Name != "updatable" || updated.Description != "new desc" {
		t.Fatalf("updated project mismatch: %+v", updated)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && !updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("UpdatedAt should be >= original: %v vs %v", updated.UpdatedAt, created.UpdatedAt)
	}
}

func TestProjectDelete(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	cw := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "deletable"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", cw.Code)
	}
	var created ProjectResponse
	env.decode(cw, &created)

	dw := env.do(http.MethodDelete, "/api/v1/projects/"+created.ProjectID, tok, nil, "")
	if dw.Code != http.StatusNoContent {
		t.Fatalf("delete want 204, got %d", dw.Code)
	}

	// Verify gone.
	gw := env.do(http.MethodGet, "/api/v1/projects/"+created.ProjectID, tok, nil, "")
	if gw.Code != http.StatusNotFound {
		t.Fatalf("deleted project should be 404, got %d", gw.Code)
	}
}

func TestProjectPermissions(t *testing.T) {
	env := newTestEnv(t)
	viewer, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	operator, _ := env.signer.Mint("op@helix.test", []string{RoleOperator}, env.srv.cfg.AccessTokenTTL, env.srv.now())

	// Viewer cannot create project.
	if w := env.doJSON(http.MethodPost, "/api/v1/projects", viewer, CreateProjectRequest{Name: "v-project"}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer create project want 403, got %d", w.Code)
	}

	// Operator can create.
	cw := env.doJSON(http.MethodPost, "/api/v1/projects", operator, CreateProjectRequest{Name: "op-project"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("operator create want 201, got %d (%s)", cw.Code, cw.Body.String())
	}
	var p ProjectResponse
	env.decode(cw, &p)

	// Viewer has NO project access → 403 (IDOR protection).
	if gw := env.do(http.MethodGet, "/api/v1/projects/"+p.ProjectID, viewer, nil, ""); gw.Code != http.StatusForbidden {
		t.Fatalf("viewer without access get want 403, got %d", gw.Code)
	}

	// Admin can access any project (super-admin bypass).
	adminTok := env.adminToken()
	if gw := env.do(http.MethodGet, "/api/v1/projects/"+p.ProjectID, adminTok, nil, ""); gw.Code != http.StatusOK {
		t.Fatalf("admin get want 200, got %d", gw.Code)
	}

	// Operator can view but NOT delete (admin-only at route level).
	if dw := env.do(http.MethodDelete, "/api/v1/projects/"+p.ProjectID, operator, nil, ""); dw.Code != http.StatusForbidden {
		t.Fatalf("operator delete want 403, got %d", dw.Code)
	}
}
