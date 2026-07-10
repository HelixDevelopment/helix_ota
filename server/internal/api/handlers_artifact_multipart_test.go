package api

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countMultipartSpillFiles counts leftover mime/multipart spill-to-disk temp
// files under dir. Go's mime/multipart spills a file part to
// os.CreateTemp(os.TempDir(), "multipart-*") whenever the part exceeds the
// in-memory threshold (engine.MaxMultipartMemory), so a leftover "multipart-*"
// file after a request completed is an orphaned spill file — a disk leak.
func countMultipartSpillFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "multipart-") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk temp dir %q: %v", dir, err)
	}
	return n
}

// TestArtifactUploadCleansSpilledMultipartTempFiles is a RED->GREEN regression
// guard for the multipart spill-to-disk temp-file leak in handleUploadArtifact
// (handlers_artifact.go).
//
// handleUploadArtifact calls c.MultipartForm() (which calls
// Request.ParseMultipartForm(engine.MaxMultipartMemory)) but never calls the
// resulting MultipartForm.RemoveAll(). When a multipart file part exceeds the
// in-memory threshold, Go's mime/multipart spills it to an
// os.CreateTemp(os.TempDir(), "multipart-*") file. Without RemoveAll(), every
// upload whose parts spilled leaves that temp file orphaned on disk — an
// unbounded disk leak / disk-DoS vector on a long-running server (reading the
// bytes via c.FormFile/readFilePart does NOT delete the spill file).
//
// The leak is forced deterministically (no 32 MB body) by:
//   - redirecting os.TempDir() to an empty, test-controlled directory via TMPDIR
//     (Go's os.CreateTemp honors TMPDIR on Linux), so spill files are counted
//     precisely, and
//   - setting engine.MaxMultipartMemory = 1 so ANY non-empty file part spills.
//
// Pre-fix (no RemoveAll): >=1 leftover "multipart-*" file after the handler
// returns -> test FAILS. Post-fix (RemoveAll on every exit path): 0 -> PASS.
// This single test is its own §11.4.115 polarity guard: removing ONLY the
// handler's `defer ... RemoveAll()` block re-reproduces the FAIL.
func TestArtifactUploadCleansSpilledMultipartTempFiles(t *testing.T) {
	// Create the controlled spill dir FIRST (under the real temp dir), THEN
	// redirect TMPDIR so only the multipart parse below spills into it.
	spillDir := t.TempDir()
	t.Setenv("TMPDIR", spillDir)

	// Sanity: os.TempDir() now resolves to our controlled dir on this platform;
	// otherwise the spill would land elsewhere and the test could not observe it.
	if got := os.TempDir(); got != spillDir {
		t.Skipf("os.TempDir() does not honor TMPDIR on this platform (got %q, want %q); "+
			"cannot deterministically observe multipart spill files", got, spillDir)
	}

	env := newTestEnv(t)
	// Force spill-to-disk for any non-empty file part (1-byte in-memory cap).
	env.router.MaxMultipartMemory = 1

	if pre := countMultipartSpillFiles(t, spillDir); pre != 0 {
		t.Fatalf("precondition: spill dir not empty before upload (%d multipart-* files)", pre)
	}

	payload := []byte("payload.bin contents large enough to spill past the 1-byte cap")
	file := zipStored(t, payload)
	meta := env.validMeta(file, "1.1.0")
	body, ct := uploadMultipart(t, file, meta)

	w := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	// 201 confirms the request reached and passed c.MultipartForm() — i.e. the
	// file part was parsed and (given the 1-byte cap) definitely spilled to disk.
	if w.Code != http.StatusCreated {
		t.Fatalf("upload want 201 (proves parse+spill ran), got %d (%s)", w.Code, w.Body.String())
	}

	// The handler has fully returned. Any spilled temp file MUST have been
	// removed by MultipartForm.RemoveAll(). A leftover proves the disk leak.
	if leftover := countMultipartSpillFiles(t, spillDir); leftover != 0 {
		t.Fatalf("multipart spill-to-disk temp-file leak: %d leftover 'multipart-*' file(s) "+
			"in %s after handleUploadArtifact returned; the handler must call "+
			"MultipartForm.RemoveAll() on every exit path", leftover, spillDir)
	}
}
