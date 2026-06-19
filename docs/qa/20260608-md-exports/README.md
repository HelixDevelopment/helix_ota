# QA Evidence — §11.4.65 Markdown sibling exports (2026-06-08)

**Run ID:** 20260608-md-exports
**Mandate:** Constitution §11.4.65 (universal Markdown export — every non-source-tree `.md` must have synchronized `.html` and `.pdf` siblings).
**Anti-bluff:** §11.4 / §11.4.65 — only formats whose files were physically produced + type-verified are claimed.

## Toolchain (real, probed at runtime)

- `pandoc 3.9.0.2` — Markdown → standalone HTML
- `WeasyPrint version 66.0` — HTML → PDF (no LaTeX engine required; matches the repo's existing sibling `.html`+`.pdf` convention)

## Mechanism

The project `scripts/export_docs.sh` writes into per-dir `_exports/` subfolders and
uses a LaTeX `--pdf-engine`. The repo's tracked convention for these spec docs is
**sibling** `.html`+`.pdf` next to each `.md` (e.g. `…/REST_API_SPECIFICATION.html`
sits beside its `.md`). To match that convention and honor §11.4.65 ("synchronized
`.html` and `.pdf` siblings"), exports were produced directly as siblings:

```
pandoc <f>.md -s --metadata title=<base> -o <f>.html
weasyprint <f>.html <f>.pdf
```

## Result — 13/13 markdown files, both formats produced

All candidate files lacked any `.html`/`.pdf` sibling beforehand. Each now has both.

| Markdown source | HTML | PDF |
|---|---|---|
| 1.0.0-mvp/api/operational_endpoints.md | OK (85,445 B) | OK (145,003 B) |
| 1.0.0-mvp/dashboard/dashboard_design.md | OK (53,974 B) | OK (94,977 B) |
| 1.0.0-mvp/security/validation_chain_order.md | OK (23,133 B) | OK (124,063 B) |
| 1.0.1-staged-rollout/rollout_engine.md | OK (66,949 B) | OK (173,640 B) |
| 1.0.1-staged-rollout/migration_002_design.md | OK (35,949 B) | OK (101,251 B) |
| 1.0.1-staged-rollout/README.md | OK (10,856 B) | OK (34,011 B) |
| 1.0.2-rollback/README.md | OK (20,341 B) | OK (56,996 B) |
| 1.0.3-delta-updates/README.md | OK (17,475 B) | OK (49,524 B) |
| research/additions_synthesis.md | OK (33,817 B) | OK (75,402 B) |
| research/repo_audit.md | OK (15,554 B) | OK (71,938 B) |
| research/additions_analysis/01_analysis.md | OK (46,349 B) | OK (83,805 B) |
| research/additions_analysis/02_analysis.md | OK (50,285 B) | OK (92,377 B) |
| research/additions_analysis/03_analysis.md | OK (47,225 B) | OK (124,104 B) |

**Totals:** html_ok=13 html_fail=0 pdf_ok=13 pdf_fail=0.
**SKIPPED:** none — both pandoc and weasyprint present, every format produced.

Type verification (`file -b`): every `.html` reports `HTML document text, UTF-8`;
every `.pdf` reports `PDF document, version 1.7`. See `run.log` for the full
command output captured live during this run.
