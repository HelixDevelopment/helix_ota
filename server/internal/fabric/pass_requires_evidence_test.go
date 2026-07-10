// Package fabric -- pass_requires_evidence_test.go: proves CompleteRun
// enforces the documented "a PASS run MUST link >=1 non-empty artefact"
// invariant.
//
// Forensic root cause (§11.4.69 evidence ledger; §11.4.6 no-guessing): the
// design contract for this exact registry is spelled out twice in the
// tracked design docs:
//   - docs/design/emulation_fabric/SCHEMA.sql:73 "The evidence ledger
//     (§11.4.69): a PASS run MUST link >=1 non-empty artefact."
//   - internal/store/store.go FabricEvidence doc comment: "A PASS run MUST
//     link >=1 non-empty artefact (§11.4.69): ByteSize MUST be > 0, enforced
//     by a CHECK in pgx and a guard in memory."
//
// Both citations describe a TWO-PART invariant: (1) every evidence row is
// non-empty (ByteSize > 0) -- enforced by AttachEvidence's byteSize<=0 guard
// and the store's ErrEvidenceEmpty/CHECK -- and (2) a PASS run links AT
// LEast ONE such row. Before the fix accompanying this test, CompleteRun
// enforced ONLY part (1) (indirectly, via AttachEvidence) and NOTHING
// enforced part (2): a run could be marked PASS with ZERO evidence rows
// ever attached. This is the exact class of PASS-bluff the Helix
// Constitution §11.4 anti-bluff covenant forbids: a completed run citing
// "PASS" with no captured evidence backing that verdict.
package fabric

import (
	"context"
	"testing"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// TestCompleteRun_PassWithoutEvidence_MustReject proves CompleteRun rejects a
// PASS verdict for a run that has NO attached evidence -- the documented
// SCHEMA.sql / store.go invariant this package is specifically chartered to
// enforce (registry.go's own package doc: "the ota-protocol PASS-evidence
// rule are OTA-domain knowledge" is why this registry lives in-project
// rather than the generic containers submodule).
func TestCompleteRun_PassWithoutEvidence_MustReject(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)

	if err := g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T0", Tech: "ota-device-emulator"}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	if _, err := g.RecordRun(ctx, "r1", "t1", "e2e", "tests/emulator/lifecycle.sh"); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	// No AttachEvidence call at all -- zero evidence rows exist for r1.
	if evs, err := g.Evidence(ctx, "r1"); err != nil || len(evs) != 0 {
		t.Fatalf("test precondition: expected 0 evidence rows, got %d err=%v", len(evs), err)
	}

	_, err := g.CompleteRun(ctx, "r1", "PASS", "")
	if err == nil {
		t.Fatal("CompleteRun accepted a PASS verdict for a run with ZERO attached " +
			"evidence artefacts -- this violates the documented SCHEMA.sql/store.go " +
			"invariant 'a PASS run MUST link >=1 non-empty artefact' and is a " +
			"§11.4 PASS-bluff: the run is recorded as passing with no captured " +
			"proof anywhere backing it.")
	}
}

// TestCompleteRun_PassWithEvidence_Succeeds is the paired positive case: once
// at least one evidence artefact is attached, PASS is accepted normally --
// proving the fix does not over-reject.
func TestCompleteRun_PassWithEvidence_Succeeds(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)

	if err := g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T0", Tech: "ota-device-emulator"}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	if _, err := g.RecordRun(ctx, "r1", "t1", "e2e", "tests/emulator/lifecycle.sh"); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if err := g.AttachEvidence(ctx, "e1", "r1", "transcript", "docs/qa/r1/t.log", 2048, "sha"); err != nil {
		t.Fatalf("AttachEvidence: %v", err)
	}

	done, err := g.CompleteRun(ctx, "r1", "PASS", "")
	if err != nil {
		t.Fatalf("CompleteRun with evidence attached should succeed, got: %v", err)
	}
	if done.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS", done.Verdict)
	}
}

// TestCompleteRun_FailWithoutEvidence_Succeeds proves the evidence
// requirement is scoped to PASS only -- a FAIL/SKIP/PENDING_FORENSICS/
// OPERATOR-BLOCKED verdict does not need a captured artefact to be recorded
// (the honest-failure/honest-skip paths must not be blocked by this guard).
func TestCompleteRun_FailWithoutEvidence_Succeeds(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	g := newTestRegistry(ts)

	if err := g.RegisterTarget(ctx, store.FabricTarget{TargetID: "t1", Tier: "T0", Tech: "ota-device-emulator"}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}
	if _, err := g.RecordRun(ctx, "r1", "t1", "e2e", "tests/emulator/lifecycle.sh"); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	done, err := g.CompleteRun(ctx, "r1", "FAIL", "")
	if err != nil {
		t.Fatalf("CompleteRun FAIL without evidence should succeed, got: %v", err)
	}
	if done.Verdict != "FAIL" {
		t.Fatalf("Verdict = %q, want FAIL", done.Verdict)
	}
}
