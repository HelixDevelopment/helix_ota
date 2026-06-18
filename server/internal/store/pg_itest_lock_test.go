//go:build integration

package store

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// lockPgIntegration serializes the shared on-demand PostgreSQL (fixed host port
// 55432, single compose project) across integration test BINARIES.
//
// `go test -tags integration ./...` runs each package's test binary in PARALLEL
// by default, so without this lock the store and rollout integration packages
// race on `compose up`/`down` of the SAME container on the SAME port — one
// package's teardown (Shutdown) removes the DB out from under the other (the
// observed ~0.5s fast-fail). An exclusive flock on a fixed lock file makes each
// package fully OWN the Postgres lifecycle for its turn (§11.4.119 single
// resource owner): boot → test → shutdown happen without overlap, then the next
// package boots a fresh DB.
//
// The unlock cleanup is registered BEFORE BootAll registers its Shutdown
// cleanup, so the LIFO t.Cleanup order runs Shutdown FIRST and the unlock LAST —
// the lock is held across the entire boot→test→shutdown window. Call this as the
// FIRST statement of any integration test that boots the shared Postgres.
func lockPgIntegration(t *testing.T) {
	t.Helper()
	lockPath := filepath.Join(os.TempDir(), "helix_ota_pg_integration.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("pg integration lock open %s: %v", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		t.Fatalf("pg integration flock: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	})
}
