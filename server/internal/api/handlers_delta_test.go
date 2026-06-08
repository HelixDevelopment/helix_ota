package api

import (
	"net/http"
	"testing"
)

func TestDeltaRegisterAndFind(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	base := env.newArtifactDirect("1.0.0")
	target := env.newArtifactDirect("1.1.0")

	// Register.
	w := env.doJSON(http.MethodPost, "/api/v1/deltas", tok, DeltaRegister{
		BaseArtifactID: base, TargetArtifactID: target, SHA256: "deltahash", Size: 512, StorageRef: "s3://d/1",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register delta want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var d DeltaView
	env.decode(w, &d)
	if d.ID == "" || d.BaseArtifactID != base || d.TargetArtifactID != target || d.Size != 512 {
		t.Fatalf("delta view mismatch: %+v", d)
	}

	// Find.
	fw := env.do(http.MethodGet, "/api/v1/deltas?base="+base+"&target="+target, tok, nil, "")
	if fw.Code != http.StatusOK {
		t.Fatalf("find delta want 200, got %d", fw.Code)
	}
	var found DeltaView
	env.decode(fw, &found)
	if found.ID != d.ID {
		t.Fatalf("found delta mismatch: %+v", found)
	}

	// Duplicate pair -> 409.
	if dup := env.doJSON(http.MethodPost, "/api/v1/deltas", tok, DeltaRegister{BaseArtifactID: base, TargetArtifactID: target}); dup.Code != http.StatusConflict {
		t.Fatalf("duplicate delta want 409, got %d", dup.Code)
	}
	// Missing pair -> 404.
	if nf := env.do(http.MethodGet, "/api/v1/deltas?base="+base+"&target=nope", tok, nil, ""); nf.Code != http.StatusNotFound {
		t.Fatalf("find unknown delta want 404, got %d", nf.Code)
	}
}

func TestDeltaRegisterValidation(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	base := env.newArtifactDirect("1.0.0")

	// base == target -> 400.
	if w := env.doJSON(http.MethodPost, "/api/v1/deltas", tok, DeltaRegister{BaseArtifactID: base, TargetArtifactID: base}); w.Code != http.StatusBadRequest {
		t.Fatalf("base==target want 400, got %d", w.Code)
	}
	// Unknown target artifact -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deltas", tok, DeltaRegister{BaseArtifactID: base, TargetArtifactID: "ghost"}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown target artifact want 404, got %d", w.Code)
	}
	// Missing fields -> 400.
	if w := env.doJSON(http.MethodPost, "/api/v1/deltas", tok, DeltaRegister{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing fields want 400, got %d", w.Code)
	}
}

func TestDeltaRegisterForbiddenForViewer(t *testing.T) {
	env := newTestEnv(t)
	base := env.newArtifactDirect("1.0.0")
	target := env.newArtifactDirect("1.1.0")
	viewer, _ := env.signer.Mint("v@helix.test", []string{RoleViewer}, env.srv.cfg.AccessTokenTTL, env.srv.now())
	if w := env.doJSON(http.MethodPost, "/api/v1/deltas", viewer, DeltaRegister{BaseArtifactID: base, TargetArtifactID: target}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer register delta want 403, got %d", w.Code)
	}
}
