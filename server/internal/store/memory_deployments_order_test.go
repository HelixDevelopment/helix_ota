package store

import (
	"context"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// TestActiveDeploymentForTargetIsDeterministic proves ActiveDeploymentForTarget
// returns the SAME (earliest-created) active deployment across repeated calls
// when two active deployments target the same os+target_model+group — the
// HB-3 finding.
//
// Root cause (fixed): ActiveDeploymentForTarget (and ListActiveDeployments)
// ranged directly over the m.deployments map, unlike every other
// matching/listing method in this file (ListDevices, ListReleases, etc.),
// which range a maintained insertion-order slice (devOrder/relOrder)
// specifically BECAUSE Go map iteration order is randomized per range
// statement — see memory_devices_order_test.go, which proves the same class
// of defect for ListDevices/pagination. With two active deployments matching
// one target, the map-range version could return a DIFFERENT deployment id
// across repeated calls against the SAME unchanged state — i.e. a polling
// device could be offered release A on one poll and release B on the next,
// non-deterministically. The fix adds a depOrder slice (populated in
// CreateDeployment, mirroring devOrder/relOrder) and iterates it instead of
// the raw map, so the EARLIEST-created matching active deployment is returned
// every time.
//
// Anti-tautology (§11.4.115/§11.4.43): a single call cannot expose this —
// map iteration order can coincidentally match the insertion order on any
// given range. The loop below calls ActiveDeploymentForTarget many times
// against completely unchanged state and asserts every call returns the
// identical result; on the pre-fix map-ranging code this reliably flips to a
// different deployment id at least once across enough iterations (captured
// RED evidence: `go test ./internal/store/ -run TestActiveDeploymentForTargetIsDeterministic -count=50`
// against the reverted map-range code produced real `--- FAIL` output; the
// same command against the depOrder-based fix is GREEN across all 50 runs).
func TestActiveDeploymentForTargetIsDeterministic(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	rel := Release{
		ReleaseID:   "rel-1",
		OSType:      otaprotocol.OSType("android"),
		TargetModel: "rk3588",
		Version:     "1.0.0",
		Status:      "published",
	}
	if err := r.CreateRelease(ctx, rel); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}

	// Two active deployments targeting the SAME os+target_model+group — the
	// precondition that exposes the nondeterminism. depA is created first, so
	// it is the deterministically-correct ("earliest-created") answer.
	depA := Deployment{DeploymentID: "dep-A", ReleaseID: rel.ReleaseID, Status: string(otaprotocol.DeploymentActive)}
	depB := Deployment{DeploymentID: "dep-B", ReleaseID: rel.ReleaseID, Status: string(otaprotocol.DeploymentActive)}
	if err := r.CreateDeployment(ctx, depA); err != nil {
		t.Fatalf("CreateDeployment depA: %v", err)
	}
	if err := r.CreateDeployment(ctx, depB); err != nil {
		t.Fatalf("CreateDeployment depB: %v", err)
	}

	const iterations = 500
	seen := make(map[string]int, 2)
	for i := 0; i < iterations; i++ {
		got, err := r.ActiveDeploymentForTarget(ctx, rel.OSType, rel.TargetModel, "")
		if err != nil {
			t.Fatalf("ActiveDeploymentForTarget iteration %d: %v", i, err)
		}
		seen[got.DeploymentID]++
	}

	if len(seen) != 1 {
		t.Fatalf("ActiveDeploymentForTarget returned %d distinct deployment ids across %d calls against UNCHANGED state: %v (want exactly 1 distinct id — deterministic)",
			len(seen), iterations, seen)
	}
	if count, ok := seen["dep-A"]; !ok || count != iterations {
		t.Fatalf("ActiveDeploymentForTarget consistently returned %v, want dep-A (the earliest-created deployment) on all %d calls", seen, iterations)
	}
}

// TestListActiveDeploymentsIsOrderStable proves ListActiveDeployments returns
// active deployments in the SAME (insertion) order across repeated calls
// against unchanged state — the sibling method fixed alongside
// ActiveDeploymentForTarget (both previously ranged m.deployments directly).
func TestListActiveDeploymentsIsOrderStable(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	rel := Release{ReleaseID: "rel-1", OSType: otaprotocol.OSType("android"), TargetModel: "rk3588", Version: "1.0.0", Status: "published"}
	if err := r.CreateRelease(ctx, rel); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}

	ids := []string{"dep-A", "dep-B", "dep-C", "dep-D", "dep-E"}
	for _, id := range ids {
		dep := Deployment{DeploymentID: id, ReleaseID: rel.ReleaseID, Status: string(otaprotocol.DeploymentActive)}
		if err := r.CreateDeployment(ctx, dep); err != nil {
			t.Fatalf("CreateDeployment %s: %v", id, err)
		}
	}

	var first []string
	for i := 0; i < 200; i++ {
		out, err := r.ListActiveDeployments(ctx)
		if err != nil {
			t.Fatalf("ListActiveDeployments iteration %d: %v", i, err)
		}
		got := make([]string, 0, len(out))
		for _, d := range out {
			got = append(got, d.DeploymentID)
		}
		if i == 0 {
			first = got
			if len(first) != len(ids) {
				t.Fatalf("ListActiveDeployments returned %d deployments, want %d", len(first), len(ids))
			}
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("iteration %d: ListActiveDeployments returned %d deployments, want %d (first call)", i, len(got), len(first))
		}
		for j := range first {
			if got[j] != first[j] {
				t.Fatalf("iteration %d: ListActiveDeployments order drifted at index %d: got %v, want %v (first call)", i, j, got, first)
			}
		}
	}
}
