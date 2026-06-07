package api

import (
	"net/http"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// uploadValid uploads a valid artifact and returns its id.
func uploadValid(t *testing.T, env *testEnv, version string) string {
	t.Helper()
	payload := []byte("payload for " + version)
	file := zipStored(t, payload)
	meta := env.validMeta(file, version)
	body, ct := uploadMultipart(t, file, meta)
	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var art Artifact
	env.decode(w, &art)
	return art.ArtifactID
}

func TestReleaseCreateAndGet(t *testing.T) {
	env := newTestEnv(t)
	artID := uploadValid(t, env, "1.1.0")

	w := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: artID, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Notes: "first",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create release want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var rel Release
	env.decode(w, &rel)
	if rel.Status != "published" {
		t.Fatalf("release status want published, got %q", rel.Status)
	}

	g := env.do(http.MethodGet, "/api/v1/releases/"+rel.ReleaseID, env.adminToken(), nil, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get release want 200, got %d", g.Code)
	}
}

func TestReleaseMonotonicity(t *testing.T) {
	env := newTestEnv(t)
	art1 := uploadValid(t, env, "1.1.0")
	w := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art1, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("first release want 201, got %d", w.Code)
	}

	// Equal version -> 409 VERSION_NOT_MONOTONIC. (Use a fresh artifact id; the
	// upload of an equal version would itself fail S4, so register the artifact
	// directly via the store to isolate the release-level check.)
	art2 := env.newArtifactDirect("1.1.0")
	w = env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art2, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("equal-version release want 409, got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.errCode(w); got != CodeVersionNotMonotonic {
		t.Fatalf("want VERSION_NOT_MONOTONIC, got %s", got)
	}
}

func TestReleaseUnknownArtifact(t *testing.T) {
	env := newTestEnv(t)
	w := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: "nope", Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown artifact want 404, got %d", w.Code)
	}
}

func TestReleaseList(t *testing.T) {
	env := newTestEnv(t)
	art := uploadValid(t, env, "1.1.0")
	env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})

	w := env.do(http.MethodGet, "/api/v1/releases?os=android", env.adminToken(), nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", w.Code)
	}
	var list ReleaseList
	env.decode(w, &list)
	if len(list.Items) != 1 {
		t.Fatalf("want 1 release, got %d", len(list.Items))
	}
}
