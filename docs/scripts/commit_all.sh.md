# `scripts/commit_all.sh` — user guide

**Revision:** 1
**Last modified:** 2026-07-10T22:10:00Z

## Overview

The official commit wrapper for the **main repo** (per the project MANDATORY
COMMIT & PUSH CONSTRAINTS — never `git add`/`commit`/`push` the main repo directly).
It stages, runs the pre-build gate, commits, and fans the push out to all four
upstreams (github/gitlab/gitflic/gitverse) in the background (§11.4.88).

## Prerequisites

- Run from anywhere inside the repo; it resolves `PROJECT_ROOT` itself.
- All four upstream remotes configured (`install_upstreams`).
- A clean-ish working tree you understand — see **`--paths` isolation** below.

## Usage

```bash
# Full commit (stages everything: git add -A), prompts for message if -m absent:
bash scripts/commit_all.sh -m "feat(x): ..."

# Explicit-paths commit — stage + commit ONLY the listed paths (space-separated):
bash scripts/commit_all.sh --paths "docs/A.md docs/A.html server/x.go" -m "..."

# Docs-only variant (§11.4.22) — the canonical doc-set:
bash scripts/commit_all.sh --docs-only -m "docs: sync"

# Synchronous push (force-push audit paths / §11.4.41 — NOTE force-push itself is
# forbidden §11.4.113; --sync-push only makes the push foreground, never forces):
bash scripts/commit_all.sh --sync-push -m "..."

# Dry run:
bash scripts/commit_all.sh --dry-run --paths "..."
```

## Inputs

| Flag | Effect |
|---|---|
| `-m <msg>` | Commit message (prompted if omitted). |
| `--paths "<p1 p2 ...>"` | Stage + commit ONLY these paths (index-isolated — see below). |
| `--docs-only` | Commit the canonical tracked doc set only (§11.4.22). |
| `--sync-push` | Push in the foreground (default is background/detached, §11.4.88). |
| `--auto-cascade` | Also `git add -A` + commit + push each dirty owned submodule first. |
| `--dry-run` | Show what would happen; no commit/push. |

## Outputs / Side-effects

- One commit on the current branch of the main repo.
- A **detached background push** to all four upstreams (§11.4.88); failures land in
  `qa-results/push_failures/<ts>_push.log`. Exit code reports COMMIT success, not push.
- Exit `3` = nothing to commit (informational).

## `--paths` index isolation (item E fix, 2026-07-10) — IMPORTANT

`--paths` commits **exactly** the listed paths and nothing else. Internally the
commit is a bare `git commit`, which snapshots the *entire* index; so the wrapper
first runs `git reset -q` (clears the parent index to HEAD, **working tree
untouched**) and then `git add -- <listed paths>`. This guarantees that anything
another actor pre-staged in the shared checkout (e.g. a subagent's `git rm`
deletions) does **not** leak into a `--paths` commit — it stays as an uncommitted
working-tree change for its proper commit.

Forensic origin: on 2026-07-10 a docs-only `--paths` commit swept in 3
subagent-staged `clients/` deletions because the bare commit snapshotted the whole
index (incident `docs/research/incident_1184_paths_leak_20260710/`). The `git reset -q`
line closes this. Belt-and-braces: the conductor still runs a
`git diff --cached --name-only` clean-precheck before every `--paths` commit, and
subagents must never run any git command (§11.4.20/§11.4.70).

## Edge cases

- `--paths` never affects submodule indexes (separate `.git`); submodule pointer
  bumps must be listed explicitly if intended.
- The `git reset -q` unstages ALL parent staging before staging the listed paths —
  intended: in `--paths` mode, anything not listed is by definition not for this commit.

## Related scripts

- `scripts/push_all.sh` — the per-remote background pusher (§11.4.88).
- `scripts/export_docs.sh` — regenerates HTML/PDF/DOCX siblings (§11.4.65).
- `scripts/pre_build_verification.sh` — the pre-build gate invoked before commit.

## Test

`tests/test_commit_all_paths_isolation.sh` — hermetic regression guard for the
`--paths` index isolation (reproduces the leak in a temp repo, proves the fix
excludes actor-staged changes) + a structural assertion that the `git reset -q`
line is present in the `--paths` branch.

**Last verified:** 2026-07-10.
