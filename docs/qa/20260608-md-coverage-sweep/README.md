# §11.4.65 Export-sibling coverage sweep — docs/research/main_specs/

**Run ID:** 20260608-md-coverage-sweep
**Date (UTC):** 2026-06-08T15:34Z
**Scope:** tracked non-source Markdown under `docs/research/main_specs/`,
EXCLUDING the `additions/` source corpus and any `_exports/` dirs.
**Mechanism:** `pandoc <f>.md -s -o <f>.html` then `weasyprint <f>.html <f>.pdf`,
each output `file`-verified.

## Tools (confirmed present)

- pandoc 3.9.0.2 (`/opt/homebrew/bin/pandoc`)
- WeasyPrint 66.0 (`/opt/homebrew/bin/weasyprint`)

## Result summary

- Candidates examined: 66 tracked `.md`
- Already up-to-date (`.html` current → SKIPPED): 20
- Generated this run: 46 `.html` + 46 `.pdf` siblings
- Generation failures: 0 (every pandoc + weasyprint step OK; every output
  `file`-verified as "HTML document text" / "PDF document")

Raw evidence: `run.log` (tool versions, per-file status table, per-file
pandoc/weasyprint output, FAIL grep = ZERO, per-sibling `file` + byte sizes).

## Siblings produced (by directory)

| Directory | HTML+PDF pairs |
|---|---|
| `docs/research/main_specs/` (README, CONTINUATION) | 2 |
| `docs/research/main_specs/00-master/` | 7 |
| `docs/research/main_specs/00-master/diagrams/` | 1 |
| `docs/research/main_specs/1.0.0-mvp/` (VALIDATION_EVIDENCE) | 1 |
| `docs/research/main_specs/1.0.0-mvp/api/` (endpoints) | 1 |
| `docs/research/main_specs/1.0.0-mvp/client_android/` | 3 |
| `docs/research/main_specs/1.0.0-mvp/database/` | 1 |
| `docs/research/main_specs/1.0.0-mvp/deployment/` | 2 |
| `docs/research/main_specs/1.0.0-mvp/security/` | 3 |
| `docs/research/main_specs/1.0.0-mvp/server/` | 3 |
| `docs/research/main_specs/1.0.0-mvp/tests/` | 1 |
| `docs/research/main_specs/1.X-linux/` | 1 |
| `docs/research/main_specs/1.X-other-os/` | 1 |
| `docs/research/main_specs/1.X-windows/` | 1 |
| `docs/research/main_specs/research/` (ota_landscape_report) | 1 |
| `docs/research/main_specs/research/adr/` | 5 |
| `docs/research/main_specs/research/stacks/` | 12 |
| **Total** | **46** |

## Skipped — `.html` already current (not regenerated, per task)

20 files: `00-master/threat_model.md`; `1.0.0-mvp/api/{implemented_endpoints,
operational_endpoints,spec_impl_alignment}.md`; `1.0.0-mvp/dashboard/
dashboard_design.md`; `1.0.0-mvp/security/validation_chain_order.md`;
all six `1.0.1-staged-rollout/*.md`; `1.0.2-rollback/README.md`;
both `1.0.3-delta-updates/*.md`; `research/additions_analysis/{01,02,03}_analysis.md`;
`research/additions_synthesis.md`; `research/repo_audit.md`.

## Skipped — out of scope (per task)

- `docs/research/main_specs/additions/` source corpus (manages own exports)
- any `_exports/` directories

## Tool-absence skips

None — both pandoc and weasyprint were present and functional.
