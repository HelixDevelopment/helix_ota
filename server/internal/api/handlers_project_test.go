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

	newDesc := "new desc"
	uw := env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, UpdateProjectRequest{Name: "updatable", Description: &newDesc})
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

// TestProjectUpdatePartialOmitsDescriptionUnchanged proves a PATCH that only
// sets `name` (the `description` field entirely absent from the JSON body,
// not present-and-empty) does NOT wipe the project's existing description.
// handleUpdateProject's current gate is `req.Description != "" ||
// c.Request.ContentLength > 0` -- but ANY non-empty PATCH body (including one
// that legitimately omits `description` to mean "leave it alone") has
// ContentLength > 0, so this condition is true on effectively every real PATCH
// request and clobbers the description to "" whenever the caller does not
// resend it verbatim. A partial update MUST be able to change just the name.
func TestProjectUpdatePartialOmitsDescriptionUnchanged(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	cw := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "partial", Description: "keep me"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", cw.Code)
	}
	var created ProjectResponse
	env.decode(cw, &created)

	// Partial update: only `name` changes, `description` field is entirely
	// absent from the JSON body (UpdateProjectRequest.Description carries
	// `omitempty` and is left at its zero value here).
	uw := env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, UpdateProjectRequest{Name: "partial-renamed"})
	if uw.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d (%s)", uw.Code, uw.Body.String())
	}
	var updated ProjectResponse
	env.decode(uw, &updated)
	if updated.Name != "partial-renamed" {
		t.Fatalf("name want partial-renamed, got %q", updated.Name)
	}
	if updated.Description != "keep me" {
		t.Fatalf("a name-only PATCH must not clear description; want %q, got %q", "keep me", updated.Description)
	}
}

// TestProjectUpdateExplicitEmptyDescriptionClears proves the complementary
// case to TestProjectUpdatePartialOmitsDescriptionUnchanged: when the caller
// DOES send an explicit empty-string `description`, it must actually clear the
// stored value (not be silently ignored just because it is the zero value).
func TestProjectUpdateExplicitEmptyDescriptionClears(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	cw := env.doJSON(http.MethodPost, "/api/v1/projects", tok, CreateProjectRequest{Name: "clearable", Description: "will be cleared"})
	if cw.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", cw.Code)
	}
	var created ProjectResponse
	env.decode(cw, &created)

	empty := ""
	uw := env.doJSON(http.MethodPatch, "/api/v1/projects/"+created.ProjectID, tok, UpdateProjectRequest{Description: &empty})
	if uw.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d (%s)", uw.Code, uw.Body.String())
	}
	var updated ProjectResponse
	env.decode(uw, &updated)
	if updated.Description != "" {
		t.Fatalf("explicit empty description must clear the field; got %q", updated.Description)
	}
	if updated.Name != "clearable" {
		t.Fatalf("name must be unchanged when omitted from the PATCH; got %q", updated.Name)
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
