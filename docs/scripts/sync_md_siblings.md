# sync_md_siblings.sh — companion guide

**Revision:** 1
**Last modified:** 2026-06-11T00:00:00Z

## Overview

`scripts/sync_md_siblings.sh` walks the §11.4.65 ("Universal Markdown
export mandate") INCLUDED Markdown corpus and (re)generates synchronized
`.html` + `.pdf` siblings for every in-scope `.md` source. It is the
out-of-the-box mechanism that keeps non-source-tree Markdown documents
in sync across `.md` / `.html` / `.pdf` per §11.4.65 + §11.4.12.

The conversion pipeline is the sanctioned one documented in
`docs/qa/20260608-md-exports/README.md`, applied per file:

```
pandoc <f>.md -s --metadata title=<base> -o <f>.html
weasyprint <f>.html <f>.pdf
```

## Prerequisites

- `pandoc` (tested with 3.9.0.2) — Markdown → standalone HTML.
- `weasyprint` (tested with 66.0) — HTML → PDF.
- `timeout` / `gtimeout` (optional) — bounds each conversion to 60s.

If `pandoc` or `weasyprint` is absent, the script logs an honest SKIP for
the whole run and exits `0` **without faking any artifact** (§11.4.6
no-guessing). It never emits a fabricated or empty PDF.

## Usage examples

```bash
# Generate missing/stale siblings across the included corpus:
bash scripts/sync_md_siblings.sh

# Preview the work without writing anything:
bash scripts/sync_md_siblings.sh --dry-run

# Force regeneration even when siblings are newer than sources:
bash scripts/sync_md_siblings.sh --force
```

The script resolves the repo root itself (`git rev-parse
--show-toplevel`), so it can be invoked from any working directory.

## Scope (what it includes / excludes)

**INCLUDE** (per §11.4.65):

- project-root `*.md`
- `docs/**/*.md` — EXCLUDING per-run evidence dirs `docs/qa/<run-id>/**`
  (those evidence READMEs carry no siblings by established convention;
  `git ls-files 'docs/qa/**/README.html' | wc -l` is `0`)
- `scripts/**/*.md`
- owned-submodule top-level `README.md` / `CLAUDE.md` / `AGENTS.md` /
  `CHANGELOG.md` (constitution, containers, the six `ota-*` bricks,
  helixqa, challenges)

**EXCLUDE**: `external/`, `prebuilts/`, `packages/modules/`, `kernel*/`,
`out/`, `build/`, `node_modules/`, `.git/`, third-party submodule
internals (e.g. `submodules/http3`), and the `docs/qa/<run-id>/`
evidence dirs.

Source files are taken from git's tracked set (`git ls-files`), so
untracked / ignored noise is never processed.

## Internal behaviour

1. Argument parse (`--force`, `--dry-run`, `--help`).
2. Repo-root resolution + `cd`.
3. Dependency preflight — honest SKIP if pandoc/weasyprint missing.
4. Build the in-scope file list (4 include classes), de-duplicated.
5. Per file: skip when both siblings exist and are `>=` source mtime
   (unless `--force`); otherwise run pandoc then weasyprint.
6. Anti-bluff post-check — a file counts as `generated` ONLY if both
   `.html` and `.pdf` exist and are **non-empty** afterwards (§11.4.6).
7. Print `SUMMARY: generated=N skipped=M failed=K`; exit non-zero iff
   any failure occurred.

Each conversion is bounded by `timeout 60` (when available). Temp list
files are removed on every exit path via `trap` (§11.4.14).

## Edge cases

- **Missing tools** → whole-run SKIP, exit 0, nothing written.
- **weasyprint CSS warnings** on the pandoc default template are benign
  and do not fail the conversion (stderr is suppressed).
- **Newer siblings** are left untouched unless `--force`.
- **Empty post-conversion sibling** is reported `FAIL`, never counted as
  generated.

## Related scripts

- `docs/qa/20260608-md-exports/README.md` — origin of the sanctioned
  pandoc+weasyprint pipeline.
- Project doc-sync wrappers (`scripts/testing/sync_*`) — the broader
  §11.4.12 / §11.4.53 / §11.4.59 export machinery this complements.

## Last verified

2026-06-11 — run produced `generated=52 skipped=99 failed=0`; two
generated PDFs spot-verified as valid (`file` → `PDF document, version
1.7`, non-zero size). pandoc 3.9.0.2 + weasyprint 66.0.
