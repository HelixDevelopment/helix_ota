# QA — OTA recall (forward-fix) lifecycle challenge

**Revision:** 1
**Last modified:** 2026-06-10T11:40:37Z

## What this covers

A new e2e/Challenge layer covering the Helix OTA **server-driven recall
(forward-fix rollback)** lifecycle:

- New script: `tests/e2e/recall_lifecycle.sh` (self-hosting, anti-bluff).
- New companion doc: `docs/scripts/recall_lifecycle.md` (§11.4.18).
- New HelixQA challenge: `HOTA-RECALL-LIFECYCLE` in
  `tools/helixqa/banks/helix_ota.yaml` (bank revision bumped 3 → 4).

The challenge dispatches to the real script and scores PASS only on its real
exit 0 + captured evidence (`tests/e2e/RECALL_EVIDENCE.txt`). It is fully
self-driving (§11.4.98): it builds + boots its own `ota-server` with an
ephemeral ed25519 key on a free-probed unique port, signs real artifacts,
stages release v1.1.0 + all-targets deployment D1, drives the operator recall
to forward-fix release v1.2.0, and tears the server down on every exit path.

## Authored-state validation (this parallel phase)

- `bash -n tests/e2e/recall_lifecycle.sh` → OK
- `sh -n tests/e2e/recall_lifecycle.sh` → OK
- `tools/helixqa/banks/helix_ota.yaml` parses as YAML; new entry present;
  existing entries undisturbed; bank revision = 4.
- Contract dry-run trace against `server/internal/api/handlers_recall.go` +
  `server.go` + `wire.go` + `ota-protocol/enums.go` confirms every asserted
  field/status/JSON-key matches the live handler (`kind="rollback"`,
  `from_release_id`/`to_release_id`/`recall_deployment_id`/`triggered_by`,
  `details.mode="forward-fix"`, recall 201, superseded `.status="superseded"`,
  recall deployment `.status="active"` = `DeploymentActive`, the 400/404
  negative controls).

The live run was intentionally NOT executed during this parallel phase because
the `server/` source may be mid-edit by another agent (§11.4.119
single-resource-owner). **The conductor MUST run it live against the clean
merged tree to capture the PASS evidence.**

## How the conductor runs it (capture live PASS evidence)

From the repo root, against the clean merged tree:

```bash
bash tests/e2e/recall_lifecycle.sh
```

Expected: `RESULT: PASS` with `0 failed`, and the full run captured to
`tests/e2e/RECALL_EVIDENCE.txt`. The script self-hosts the server (no manual
server start, no admin password input) on a free probed port and cleans up on
exit. To pin a port: `bash tests/e2e/recall_lifecycle.sh --port 8097`.

A `SKIP` (exit 3) occurs only if a prerequisite tool (`go`, `openssl` ed25519,
`xxd`, `base64`, `curl`, `jq`, `python3`) is unavailable — never a false PASS.

After the live run, the conductor should commit `tests/e2e/RECALL_EVIDENCE.txt`
as the captured PASS evidence for `HOTA-RECALL-LIFECYCLE` and may update the
challenge's `asserted_lines` summary counts to match the observed run.
