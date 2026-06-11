package api

import (
	"context"
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// This file raises coverage of the error / edge branches that the happy-path
// suites leave uncovered: malformed-body 400s, missing-field 422/400s, 404s on
// unknown resources, conflict branches, and pagination-param validation. Every
// assertion checks a status code (and where meaningful, an error code) that the
// handler would NOT produce if the branch under test regressed — i.e. each test
// fails if its specific guard is removed.

// ---------------------------------------------------------------------------
// bindJSON strictness (unknown fields + trailing data) — exercised through a
// real handler so the strict-decode branch is hit on the request path.
// ---------------------------------------------------------------------------

// TestBindJSONRejectsUnknownField proves bindJSON's DisallowUnknownFields guard:
// a body with a field the wire type does not declare is a 400, not silently
// accepted. Regression (dropping DisallowUnknownFields) would make this 201.
func TestBindJSONRejectsUnknownField(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodPost, "/api/v1/groups", env.adminToken(),
		[]byte(`{"name":"g","unexpected_field":true}`), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeValidationFailed {
		t.Fatalf("want VALIDATION_FAILED, got %s", got)
	}
}

// TestBindJSONRejectsTrailingData proves bindJSON rejects a second JSON value
// after the first (dec.More()). Regression (dropping the trailing check) makes
// this 201.
func TestBindJSONRejectsTrailingData(t *testing.T) {
	env := newTestEnv(t)
	w := env.do(http.MethodPost, "/api/v1/groups", env.adminToken(),
		[]byte(`{"name":"g"}{"name":"h"}`), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trailing data want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Group handler error branches.
// ---------------------------------------------------------------------------

func TestGroupMalformedAndMissingBodies(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Create with malformed JSON -> 400.
	if w := env.do(http.MethodPost, "/api/v1/groups", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed create body want 400, got %d", w.Code)
	}
	// Create with empty name -> 400 VALIDATION_FAILED.
	if w := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: ""}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty name want 400, got %d", w.Code)
	}
}

// TestGroupUpdateUnknownAndConflict exercises handleUpdateGroup's 404 (unknown
// group), malformed-body, and conflict branches plus the description-clearing
// behaviour.
func TestGroupUpdateUnknownAndConflict(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()

	// Update a group that does not exist -> 404.
	if w := env.doJSON(http.MethodPatch, "/api/v1/groups/ghost", tok, GroupUpdate{Name: "x"}); w.Code != http.StatusNotFound {
		t.Fatalf("update unknown group want 404, got %d", w.Code)
	}
	// Malformed update body -> 400.
	if w := env.do(http.MethodPatch, "/api/v1/groups/ghost", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed update body want 400, got %d", w.Code)
	}

	// Create two groups, then rename one onto the other's name -> 409 conflict.
	a := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "alpha"})
	if a.Code != http.StatusCreated {
		t.Fatalf("create alpha want 201, got %d", a.Code)
	}
	b := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "beta"})
	if b.Code != http.StatusCreated {
		t.Fatalf("create beta want 201, got %d", b.Code)
	}
	var bv GroupView
	env.decode(b, &bv)
	conflict := env.doJSON(http.MethodPatch, "/api/v1/groups/"+bv.GroupID, tok, GroupUpdate{Name: "alpha"})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("rename onto existing name want 409, got %d (%s)", conflict.Code, conflict.Body.String())
	}
	if got := env.errCode(conflict); got != CodeConflict {
		t.Fatalf("want CONFLICT, got %s", got)
	}
}

// TestGroupDeleteAndRemoveMemberUnknown covers the 404 branches of delete-group
// and remove-member against a non-existent group.
func TestGroupDeleteAndRemoveMemberUnknown(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	if w := env.do(http.MethodDelete, "/api/v1/groups/ghost", tok, nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("delete unknown group want 404, got %d", w.Code)
	}
	if w := env.do(http.MethodDelete, "/api/v1/groups/ghost/members/dev-x", tok, nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("remove member from unknown group want 404, got %d", w.Code)
	}
}

// TestGroupAddMembersBadBodies covers handleAddGroupMembers' malformed-body and
// empty-device_ids 400 branches.
func TestGroupAddMembersBadBodies(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	cw := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: "g"})
	var g GroupView
	env.decode(cw, &g)

	// Malformed members body -> 400.
	if w := env.do(http.MethodPost, "/api/v1/groups/"+g.GroupID+"/members", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed members body want 400, got %d", w.Code)
	}
	// Empty device_ids -> 400 VALIDATION_FAILED.
	if w := env.doJSON(http.MethodPost, "/api/v1/groups/"+g.GroupID+"/members", tok, MemberAdd{DeviceIDs: []string{}}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty device_ids want 400, got %d", w.Code)
	}
}

// TestGroupListPaginationBadParams proves parsePage's validation: a bad limit
// and a bad cursor each yield 400 VALIDATION_FAILED on the groups list.
func TestGroupListPaginationBadParams(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	cases := []struct{ name, query string }{
		{"non-numeric limit", "?limit=abc"},
		{"limit too large", "?limit=9999"},
		{"limit zero", "?limit=0"},
		{"negative cursor", "?cursor=-1"},
		{"non-numeric cursor", "?cursor=xyz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(http.MethodGet, "/api/v1/groups"+tc.query, tok, nil, "")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s want 400, got %d (%s)", tc.name, w.Code, w.Body.String())
			}
			if got := env.errCode(w); got != CodeValidationFailed {
				t.Fatalf("%s want VALIDATION_FAILED, got %s", tc.name, got)
			}
		})
	}
}

// TestGroupListPaginationCursor proves the NextCursor / offset paging math: with
// 3 groups and limit=2 the first page returns 2 items + a cursor; the cursor
// page returns the remaining 1 and no cursor.
func TestGroupListPaginationCursor(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	for _, n := range []string{"g1", "g2", "g3"} {
		if w := env.doJSON(http.MethodPost, "/api/v1/groups", tok, GroupCreate{Name: n}); w.Code != http.StatusCreated {
			t.Fatalf("create %s want 201, got %d", n, w.Code)
		}
	}
	first := env.do(http.MethodGet, "/api/v1/groups?limit=2", tok, nil, "")
	var p1 GroupList
	env.decode(first, &p1)
	if len(p1.Items) != 2 || p1.NextCursor == nil {
		t.Fatalf("page1 want 2 items + cursor, got %d items cursor=%v", len(p1.Items), p1.NextCursor)
	}
	second := env.do(http.MethodGet, "/api/v1/groups?limit=2&cursor="+*p1.NextCursor, tok, nil, "")
	var p2 GroupList
	env.decode(second, &p2)
	if len(p2.Items) != 1 || p2.NextCursor != nil {
		t.Fatalf("page2 want 1 item + no cursor, got %d items cursor=%v", len(p2.Items), p2.NextCursor)
	}
}

// ---------------------------------------------------------------------------
// Release handler error branches.
// ---------------------------------------------------------------------------

// TestReleaseCreateBadBodies covers handleCreateRelease's malformed-body and
// missing-required-fields 400 branches.
func TestReleaseCreateBadBodies(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	if w := env.do(http.MethodPost, "/api/v1/releases", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed release body want 400, got %d", w.Code)
	}
	// Missing all required fields -> 400 VALIDATION_FAILED. Sent as raw JSON
	// because OSType's marshaler rejects the empty enum value.
	if w := env.do(http.MethodPost, "/api/v1/releases", tok, []byte(`{}`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("missing fields want 400, got %d", w.Code)
	}
	if got := env.errCode(env.do(http.MethodPost, "/api/v1/releases", tok, []byte(`{}`), "application/json")); got != CodeValidationFailed {
		t.Fatalf("missing fields want VALIDATION_FAILED, got %s", got)
	}
}

// TestReleaseUnverifiedArtifact proves a release referencing an artifact that is
// not Verified is rejected 400 (the upload pipeline normally sets Verified; here
// we register an unverified artifact directly to isolate the guard).
func TestReleaseUnverifiedArtifact(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	// newArtifactDirect sets Verified:true, so craft an unverified one inline.
	id := env.srv.newID()
	if err := env.repo.CreateArtifact(context.Background(), store.Artifact{
		ArtifactID:  id,
		SHA256:      sha256Hex([]byte("unverified")),
		Size:        10,
		OSType:      otaprotocol.OSAndroid,
		TargetModel: "OrangePi5Max",
		Version:     "1.1.0",
		Verified:    false,
		UploadedAt:  env.srv.now(),
	}); err != nil {
		t.Fatalf("insert unverified artifact: %v", err)
	}
	w := env.doJSON(http.MethodPost, "/api/v1/releases", tok, ReleaseCreate{
		ArtifactID: id, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unverified artifact release want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeValidationFailed {
		t.Fatalf("want VALIDATION_FAILED, got %s", got)
	}
}

// TestReleaseUnparseableVersion proves the monotonicity branch's version-parse
// guard: with an existing release present, a syntactically invalid new version
// is rejected 400 (not 409), because CompareDotted errors before the comparison.
func TestReleaseUnparseableVersion(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	// First, publish a valid baseline release for the target.
	art := uploadValid(t, env, "1.1.0")
	if w := env.doJSON(http.MethodPost, "/api/v1/releases", tok, ReleaseCreate{
		ArtifactID: art, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	}); w.Code != http.StatusCreated {
		t.Fatalf("baseline release want 201, got %d", w.Code)
	}
	// Now attempt a release whose version is unparseable. Use a directly-inserted
	// verified artifact so the release-level version-parse branch is what fails.
	bad := env.newArtifactDirect("not-a-version")
	w := env.doJSON(http.MethodPost, "/api/v1/releases", tok, ReleaseCreate{
		ArtifactID: bad, Version: "not.a.version", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unparseable version want 400, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeValidationFailed {
		t.Fatalf("want VALIDATION_FAILED, got %s", got)
	}
}

// TestReleaseGetUnknown + list bad-limit branch.
func TestReleaseGetUnknownAndListBadLimit(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	if w := env.do(http.MethodGet, "/api/v1/releases/ghost", tok, nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("get unknown release want 404, got %d", w.Code)
	}
	if w := env.do(http.MethodGet, "/api/v1/releases?limit=0", tok, nil, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("list bad limit want 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Rollout handler error branches.
// ---------------------------------------------------------------------------

// TestRolloutBadBodiesAndEmptyPhases covers create's malformed-body and
// empty-phases 400 branches.
func TestRolloutBadBodiesAndEmptyPhases(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	if w := env.do(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed rollout body want 400, got %d", w.Code)
	}
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout", tok, RolloutCreate{Phases: nil}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty phases want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestRolloutGetUnknown covers handleGetRollout's 404 (no rollout for the
// deployment).
func TestRolloutGetUnknown(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()
	if w := env.do(http.MethodGet, "/api/v1/deployments/"+depID+"/rollout", tok, nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("get rollout before create want 404, got %d", w.Code)
	}
}

// TestRolloutEvaluateBadBodyAndUnknown covers evaluate's malformed-body 400 and
// the ErrNotFound -> 404 branch (evaluate before any rollout exists).
func TestRolloutEvaluateBadBodyAndUnknown(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	tok := env.adminToken()

	if w := env.do(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed verdict body want 400, got %d", w.Code)
	}
	// Evaluate with no rollout created yet -> engine ErrNotFound -> 404.
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments/"+depID+"/rollout/evaluate", tok,
		RolloutVerdict{SuccessRate: 0.9}); w.Code != http.StatusNotFound {
		t.Fatalf("evaluate with no rollout want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Deployment handler error branches.
// ---------------------------------------------------------------------------

// TestDeploymentCreateBadBodyAndMissingRelease covers the malformed-body and
// missing-release_id 400 branches.
func TestDeploymentCreateBadBodyAndMissingRelease(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	if w := env.do(http.MethodPost, "/api/v1/deployments", tok, []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed deployment body want 400, got %d", w.Code)
	}
	if w := env.doJSON(http.MethodPost, "/api/v1/deployments", tok, DeploymentCreate{Strategy: "all-targets"}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing release_id want 400, got %d", w.Code)
	}
}

// TestDeploymentGetUnknown covers handleGetDeployment's 404 branch.
func TestDeploymentGetUnknown(t *testing.T) {
	env := newTestEnv(t)
	if w := env.do(http.MethodGet, "/api/v1/deployments/ghost", env.adminToken(), nil, ""); w.Code != http.StatusNotFound {
		t.Fatalf("get unknown deployment want 404, got %d", w.Code)
	}
}

// TestDeploymentIdempotencyReplay proves the Idempotency-Key replay branch: a
// second create with the same key returns 200 with the SAME deployment id
// (instead of a fresh 201). Regression (ignoring the key) would 409-conflict or
// mint a new id.
func TestDeploymentIdempotencyReplay(t *testing.T) {
	env := newTestEnv(t)
	registerDevice(t, env, DeviceRegistration{HardwareID: "idem-hw", Model: "OrangePi5Max", OS: otaprotocol.OSAndroid})
	relID := createReleaseFor(t, env, "1.1.0")
	tok := env.adminToken()

	first := newAuthedReq(t, http.MethodPost, "/api/v1/deployments", tok,
		mustJSON(t, DeploymentCreate{ReleaseID: relID, Strategy: "all-targets"}))
	first.Header.Set("Idempotency-Key", "key-123")
	w1 := serveReq(env, first)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d (%s)", w1.Code, w1.Body.String())
	}
	var d1 Deployment
	env.decode(w1, &d1)

	second := newAuthedReq(t, http.MethodPost, "/api/v1/deployments", tok,
		mustJSON(t, DeploymentCreate{ReleaseID: relID, Strategy: "all-targets"}))
	second.Header.Set("Idempotency-Key", "key-123")
	w2 := serveReq(env, second)
	if w2.Code != http.StatusOK {
		t.Fatalf("idempotent replay want 200, got %d (%s)", w2.Code, w2.Body.String())
	}
	var d2 Deployment
	env.decode(w2, &d2)
	if d2.DeploymentID != d1.DeploymentID {
		t.Fatalf("idempotent replay returned a different deployment: %q != %q", d2.DeploymentID, d1.DeploymentID)
	}
}

// ---------------------------------------------------------------------------
// Auth handler error branches.
// ---------------------------------------------------------------------------

// TestAuthBadBodies covers login + refresh malformed-body and missing-field 400
// branches that the happy-path auth suite does not exercise.
func TestAuthBadBodies(t *testing.T) {
	env := newTestEnv(t)
	if w := env.do(http.MethodPost, "/api/v1/auth/login", "", []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed login body want 400, got %d", w.Code)
	}
	if w := env.do(http.MethodPost, "/api/v1/auth/refresh", "", []byte(`{`), "application/json"); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed refresh body want 400, got %d", w.Code)
	}
	// Missing refresh_token -> 400 VALIDATION_FAILED.
	if w := env.doJSON(http.MethodPost, "/api/v1/auth/refresh", "", RefreshRequest{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing refresh_token want 400, got %d", w.Code)
	}
	if got := env.errCode(env.doJSON(http.MethodPost, "/api/v1/auth/refresh", "", RefreshRequest{})); got != CodeValidationFailed {
		t.Fatalf("missing refresh_token want VALIDATION_FAILED, got %s", got)
	}
}

// TestRefreshUnknownToken covers handleRefresh's rotate-miss -> 401 branch (a
// refresh token the store never issued).
func TestRefreshUnknownToken(t *testing.T) {
	env := newTestEnv(t)
	w := env.doJSON(http.MethodPost, "/api/v1/auth/refresh", "", RefreshRequest{RefreshToken: "never-issued"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown refresh token want 401, got %d", w.Code)
	}
	if got := env.errCode(w); got != CodeUnauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// Audit handler param-validation branches.
// ---------------------------------------------------------------------------

// TestAuditListParamValidation covers handleListAudit's limit / since / until
// validation branches (each a 400 VALIDATION_FAILED) and a happy filtered read.
func TestAuditListParamValidation(t *testing.T) {
	env := newTestEnv(t)
	tok := env.adminToken()
	cases := []struct{ name, query string }{
		{"bad limit", "?limit=0"},
		{"non-numeric limit", "?limit=abc"},
		{"limit too large", "?limit=500"},
		{"bad since", "?since=not-a-time"},
		{"bad until", "?until=2026-13-99"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(http.MethodGet, "/api/v1/audit"+tc.query, tok, nil, "")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s want 400, got %d (%s)", tc.name, w.Code, w.Body.String())
			}
			if got := env.errCode(w); got != CodeValidationFailed {
				t.Fatalf("%s want VALIDATION_FAILED, got %s", tc.name, got)
			}
		})
	}

	// A well-formed filtered read with all optional params parses + returns 200.
	ok := env.do(http.MethodGet, "/api/v1/audit?limit=10&since=2020-01-01T00:00:00Z&until=2030-01-01T00:00:00Z&action=DEVICE_REGISTER&resource_type=device", tok, nil, "")
	if ok.Code != http.StatusOK {
		t.Fatalf("well-formed filtered audit read want 200, got %d (%s)", ok.Code, ok.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Recall handler malformed-body branch.
// ---------------------------------------------------------------------------

// TestRecallMalformedBody covers handleRecall's bindJSON-error 400 branch (the
// existing recall suite covers missing-field + 404s but not a malformed body).
func TestRecallMalformedBody(t *testing.T) {
	env := newTestEnv(t)
	setupDeployment(t, env, "1.0.0", "1.1.0")
	depID := activeDeploymentID(t, env)
	w := env.do(http.MethodPost, "/api/v1/deployments/"+depID+"/recall", env.adminToken(), []byte(`{`), "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed recall body want 400, got %d (%s)", w.Code, w.Body.String())
	}
}
