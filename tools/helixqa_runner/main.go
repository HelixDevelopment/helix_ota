// Command helixqa_runner drives the Helix OTA HelixQA bank through the
// HelixQA Go orchestrator's testbank.Dispatcher against the LIVE OTA system.
//
// It is the consumer-side DeviceExec host-exec adapter the bank's
// `dispatches_to` challenges need: the Dispatcher's Exec hook runs each
// challenge's script via os/exec from the repo root, captures its exit code
// (GATE 1: non-zero exit => FAIL), then the EvidenceResolver enforces every
// RequiredEvidence token resolves to a real non-empty artefact (GATE 2 /
// §11.4.69). The result is the orchestrator's native QAReport — crash/exit
// detection + evidence-ledger verdicts — not a shell tally.
//
// Usage: helixqa_runner <bank.yaml> <repo-root> [id-substring-filter]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/testbank"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: helixqa_runner <bank.yaml> <repo-root> [id-filter]")
		os.Exit(2)
	}
	bankPath, repoRoot := os.Args[1], os.Args[2]
	filter := ""
	if len(os.Args) > 3 {
		filter = os.Args[3]
	}

	mgr := testbank.NewManager()
	if err := mgr.LoadFile(bankPath); err != nil {
		fmt.Fprintf(os.Stderr, "load %s: %v\n", bankPath, err)
		os.Exit(1)
	}

	disp := &testbank.Dispatcher{
		// Real host-exec: run the challenge script from the repo root,
		// surface its real exit code (§11.4 anti-bluff — no simulation).
		Exec: testbank.DeviceExecFunc(func(ctx context.Context, command string) (string, int, error) {
			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = repoRoot
			out, err := cmd.CombinedOutput()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return string(out), ee.ExitCode(), nil
				}
				return string(out), -1, err
			}
			return string(out), 0, nil
		}),
		// Evidence ledger (§11.4.69): a token resolves only to real,
		// non-empty artefact paths (file >0 bytes OR a non-empty dir).
		Evidence: testbank.EvidenceResolverFunc(func(token string) ([]string, error) {
			p := token
			if !filepath.IsAbs(p) {
				p = filepath.Join(repoRoot, token)
			}
			cands, _ := filepath.Glob(p)
			if len(cands) == 0 {
				cands = []string{p}
			}
			var ok []string
			for _, c := range cands {
				fi, err := os.Stat(c)
				if err != nil {
					continue
				}
				if fi.IsDir() {
					entries, _ := os.ReadDir(c)
					if len(entries) > 0 {
						ok = append(ok, c)
					}
				} else if fi.Size() > 0 {
					ok = append(ok, c)
				}
			}
			return ok, nil
		}),
		Timeout: 30 * time.Minute,
	}

	ctx := context.Background()
	type row struct {
		ID, Challenge, Verdict, Reason string
		Exit                           int
		Evidence                       map[string][]string
	}
	var rows []row
	pass, fail, skip := 0, 0, 0

	for _, tc := range mgr.All() {
		if filter != "" && !strings.Contains(tc.ID, filter) {
			continue
		}
		if tc.DispatchesTo == "" && len(tc.RequiredEvidence) == 0 {
			continue // not a dispatcher-owned case
		}
		res := disp.Run(ctx, tc)
		v := strings.ToUpper(fmt.Sprintf("%v", res.Verdict))
		// Project convention (§11.4.3): a dispatched script exit 3 is an
		// off-topology SKIP, not a FAIL. The Dispatcher maps any non-zero
		// exit to FAIL, so re-classify exit-3 here at the consumer layer —
		// an honest SKIP-with-reason, never a fake PASS.
		if res.ExitCode == 3 && strings.Contains(v, "FAIL") {
			v = "SKIP"
			res.Reason = "off_topology_exit_3 (script self-gate)"
		}
		switch {
		case strings.Contains(v, "PASS"):
			pass++
		case strings.Contains(v, "FAIL"):
			fail++
		default:
			skip++
		}
		rows = append(rows, row{tc.ID, res.ChallengeID, v, res.Reason, res.ExitCode, res.EvidencePaths})
		fmt.Printf("%-30s %-6s exit=%-3d %s\n", tc.ID, v, res.ExitCode, res.Reason)
	}

	fmt.Printf("\nORCHESTRATOR LEDGER: %d PASS / %d FAIL / %d SKIP (dispatched %d)\n",
		pass, fail, skip, len(rows))

	data, _ := json.MarshalIndent(rows, "", "  ")
	_ = os.WriteFile(filepath.Join(repoRoot, "tools/helixqa_runner/orchestrator_report.json"), data, 0o644)

	if fail > 0 {
		os.Exit(1)
	}
}
