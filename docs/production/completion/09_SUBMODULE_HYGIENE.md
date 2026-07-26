# 09 — Submodule + Tracking Hygiene (Ongoing)

**Revision:** 1
**Parent:** `00_MASTER_INDEX.md`
**Prerequisites:** None (can run anytime, recommended throughout all stages)

---

## Overview

This stage covers submodule governance, tracking tooling, and documentation hygiene. These are NOT production-blocking individually but cumulatively represent significant technical debt. Perform throughout the project; do not batch at the end.

---

## I-01 [AGENT] — Add helix-deps.yaml to All 6 ota-* Bricks

**Effort:** S (~30 min)
**Source:** DELTA_ANALYSIS §2 SUB-NEW-1

### Current state:
NONE of the 6 ota-* bricks (`ota-protocol`, `ota-telemetry-schema`, `ota-artifact-validator`, `ota-rollout-engine`, `ota-update-engine-bridge`, `ota-android-agent`) has a `helix-deps.yaml`. All other owned submodules DO have one.

### What to do:
1. For each brick, create `helix-deps.yaml`:
   ```yaml
   # helix-deps.yaml — machine-readable dependency manifest (§11.4.31)
   repo: HelixDevelopment/<brick-name>
   type: library
   language: go  # or kotlin for ota-android-agent / ota-update-engine-bridge
   dependencies: []
   ```
2. Commit and push to each brick's origin.

---

## I-02 [AGENT] — Add upstreams/ Recipes to All 6 ota-* Bricks

**Effort:** S (~30 min)
**Source:** DELTA_ANALYSIS §2 SUB-NEW-1

### Current state:
The 6 ota-* bricks have only a single `origin` remote. Per §11.4.36, each brick should push to all 4 mirrors (GitHub, GitLab, GitFlic, GitVerse).

### What to do:
1. For each brick, create `upstreams/install_upstreams.sh` following the pattern from other submodules (e.g., `containers/upstreams/`).
2. Run the script: `bash upstreams/install_upstreams.sh`.
3. Verify: `git remote -v` shows all 4 remotes.
4. Commit and push.

---

## I-03 [AGENT] — Add Wire Schema-Version Field

**Effort:** M (~2h)
**Source:** DELTA_ANALYSIS §2 SUB-NEW-2

### Current state:
Wire payloads carry no `SchemaVersion`/`ProtocolVersion` field. The only versioning is the coarse REST URL path `/api/v1`. Cross-version device↔server compatibility is unnegotiable.

### What to do:
1. **ota-protocol:** Add `SchemaVersion int` to `UpdateAvailable`, `UpdateCheckRequest`, `TelemetryEvent`, `DeviceRegisterRequest`. Default to `1` for v1.0.0.
2. **Server:** Accept `SchemaVersion: 1`, reject unknown versions with `400 UNSUPPORTED_SCHEMA_VERSION`.
3. **ota-telemetry-schema:** Add `SchemaVersion int` to `TelemetryEvent`.
4. **Kotlin DTOs:** Add `schemaVersion` field to DTOs in `ota-android-agent/core/.../Dtos.kt`.
5. **Tests:** `TestSchemaVersionNegotiation`, `TestRejectsUnknownSchemaVersion`.

---

## I-04 [AGENT] — Resolve Mirror-Fork Canonicalization

**Effort:** M (~2h, after A-04 decision)
**Source:** PRODUCTION_READINESS_PLAN.md K1

### Affected submodules:
- `vision_engine` — 6 fixes at `1ec0c57`, publish blocked
- `llm_orchestrator` — 3 fixes + 3 tests, scratchpad only
- `llm_provider` — 1 fix, scratchpad only

### After A-04 resolves the canonical org + branch:
1. **Clone the canonical mirror** as the base.
2. **UNION-merge** the divergent branches (never keep-ours, never force-push §11.4.113):
   - Apply the preserved fixes on top of the canonical base
   - Run tests on the merged tree
   - Push to all 4 mirrors from the canonical branch
3. **Retire** non-canonical branches (tag as `archive/<branch>` if needed for history).
4. **Bump parent gitlinks** in the main helix_ota repo to the new canonical commits.
5. **Verify:** All 4 mirrors show the same HEAD for the canonical branch.

---

## I-05 [AGENT] — Adopt Constitution workable-items Engine

**Effort:** M (~2h)
**Source:** DELTA_ANALYSIS §3 KI-2

### Current state:
The in-repo `cmd/workable-items/main.go` is a simplified reimplementation. Its `db-to-md` sync is an explicit stub (`main.go:937-938`). The canonical engine lives at `constitution/scripts/workable-items/cmd/workable-items/`.

### What to do:
1. **Remove** the local `cmd/workable-items/` directory (retire the simplified copy).
2. **Reference** the constitution engine by path: `constitution/scripts/workable-items/cmd/workable-items`.
3. **Update scripts** that call the local tool to call the constitution engine instead.
4. **Verify:** `db-to-md` sync works bidirectionally (DB ↔ Issues.md ↔ Fixed.md byte-identical round-trip).
5. **Per §11.4.177:** The constitution tooling is project-agnostic; it operates on the invocation directory — no hardcoded project path.

---

## I-06 [AGENT] — Extend workable_items.db Schema

**Effort:** M (~1h)
**Source:** DELTA_ANALYSIS §3 KI-3

### Missing columns/tables:
- `items` table missing: `created_by`, `assigned_to` (§11.4.104), `canonical_track` + `branch` (§11.4.181/§11.4.191), `reopens_count` (§11.4.55)
- Missing: `logic_groups` table (§11.4.191), `test_diary` table (§11.4.149)

### What to do:
1. **Add columns:**
   ```sql
   ALTER TABLE items ADD COLUMN created_by TEXT NOT NULL DEFAULT '';
   ALTER TABLE items ADD COLUMN assigned_to TEXT NOT NULL DEFAULT '';
   ALTER TABLE items ADD COLUMN canonical_track TEXT NOT NULL DEFAULT '';
   ALTER TABLE items ADD COLUMN branch TEXT NOT NULL DEFAULT '';
   ALTER TABLE items ADD COLUMN reopens_count INTEGER NOT NULL DEFAULT 0;
   ```

2. **Add tables:**
   ```sql
   CREATE TABLE IF NOT EXISTS logic_groups (
       group_name TEXT PRIMARY KEY,
       track TEXT NOT NULL,
       branch TEXT NOT NULL UNIQUE,
       description TEXT NOT NULL DEFAULT ''
   );
   
   CREATE TABLE IF NOT EXISTS test_diary (
       id TEXT PRIMARY KEY,
       item_id TEXT NOT NULL REFERENCES items(ota_id),
       run_date TEXT NOT NULL,
       verdict TEXT NOT NULL CHECK (verdict IN ('PASS','FAIL','SKIP')),
       evidence_path TEXT NOT NULL DEFAULT '',
       notes TEXT NOT NULL DEFAULT ''
   );
   ```

3. **Run migration** via `scripts/testing/migrate_workable_items.sh`.

---

## I-07 [AGENT] — Fix §11.4.33 Type-Aware Closure Vocabulary

**Effort:** S (~30 min)
**Source:** DELTA_ANALYSIS §3 KI-4

### Current state:
12 closed items use the wrong closure vocabulary:
- 7 Features marked as `Fixed` → should be `Implemented (→ Fixed.md)`
- 5 Tasks marked as `Fixed` → should be `Completed (→ Fixed.md)`
- Only 1 Bug (OTA-020) uses the correct `Fixed (→ Fixed.md)`

### What to do:
```sql
UPDATE items SET status = 'Implemented (→ Fixed.md)' WHERE type = 'Feature' AND status = 'Fixed (→ Fixed.md)';
UPDATE items SET status = 'Completed (→ Fixed.md)' WHERE type = 'Task' AND status = 'Fixed (→ Fixed.md)';
```

### Verify:
```bash
sqlite3 docs/workable_items.db "SELECT type, status, count(*) FROM items WHERE status LIKE '%Fixed.md%' GROUP BY type, status"
# Should show:
#   Bug|Fixed (→ Fixed.md)|1
#   Feature|Implemented (→ Fixed.md)|7
#   Task|Completed (→ Fixed.md)|5
```

---

## I-08 [AGENT] — Reconcile Track/Branch Drift + Record Canonical Bindings

**Effort:** S (~30 min)
**Source:** PRODUCTION_READINESS_PLAN.md §2.9

### Current drift:
- `config/multitrack/the-factory.yaml`: T2 = web dashboard/website, T3 = containers-hardening, T4 = vm-emu-hardening
- Operator directive: claude1 → T2 accounts, claude3 → T4 website
- Accounts delivery plan: canonical branch `feature/multi-tenant-accounts` (not `feature/accounts-web`)

### What to do:
1. **Reconcile:** Update `the-factory.yaml` to match the canonical bindings:
   - T2 / `feature/multi-tenant-accounts` (Accounts)
   - T3 / `feature/containers-hardening` (Containers)
   - T4 / `feature/website` (Website)
2. **Record in registry:** Insert into `logic_groups` table (from I-06):
   ```sql
   INSERT INTO logic_groups VALUES ('multi-tenant-accounts', 'T2', 'feature/multi-tenant-accounts', 'Multi-tenant Accounts feature');
   INSERT INTO logic_groups VALUES ('containers-hardening', 'T3', 'feature/containers-hardening', 'Containers hardening');
   INSERT INTO logic_groups VALUES ('website', 'T4', 'feature/website', 'Marketing website');
   ```
3. **Enforce:** The `§11.4.181` gate (`CM-BRANCH-NAME-CONSISTENCY`) fails if ANY submodule touched by a logic group uses a divergent branch name.

---

## I-09 [AGENT] — Bump llm_orchestrator Parent Gitlink

**Effort:** S (~10 min, after I-04)
**Source:** PRODUCTION_READINESS_PLAN.md §2.6

### Current state:
Parent gitlink `7e3e6da` lags the submodule HEAD by 2 commits. After I-04 resolves the mirror fork and the canonical HEAD is established, bump the gitlink:

```bash
cd submodules/llm_orchestrator
git pull origin main  # or canonical branch
CANONICAL_HEAD=$(git rev-parse HEAD)
cd ../..
git add submodules/llm_orchestrator
git commit -m "chore: bump llm_orchestrator gitlink to $CANONICAL_HEAD"
```

---

## I-10 [AGENT] — Fix Stale Comments

**Effort:** S (~20 min)
**Source:** PRODUCTION_READINESS_PLAN.md K5, K6

### K5 (VAL-1):
**File:** `submodules/ota-artifact-validator/stages.go:15`
**Current:** Comment claims `ota-protocol.ValidateArtifactMeta` enforces a 255-char Version bound.
**Fix:** Change comment to name the real in-package `ValidateVersion` guard.

### K6 (fabric-lease):
**Files:** `server/internal/store/postgres_fabric.go:137-139`, `store.go:220-229`
**Current:** Comments still describe old non-exclusive lease behavior (fixed by HB-2 `03340896`).
**Fix:** Update comments to describe current uniform exclusive-lease behavior.

---

## I-11 [AGENT] — Remove LLMProvider Symlink

**Effort:** S (~10 min)
**Source:** DELTA_ANALYSIS §3 KI-5

### Current state:
`submodules/LLMProvider` is a symbolic link to `llm_provider` (NOT a duplicate directory as OTA-027 originally framed).

### What to do:
1. **Verify nothing resolves `submodules/LLMProvider`** path:
   ```bash
   grep -r "LLMProvider\|LLMProvider" --include="*.go" --include="*.mod" --include="*.yaml" server/ submodules/ 2>/dev/null
   ```
2. **If no references:** Remove the symlink:
   ```bash
   rm submodules/LLMProvider
   ```
3. **If references exist:** Update them to use `llm_provider` (the tracked submodule), then remove.

---

## Verification Checklist

| Step | Action | Expected Result |
|------|--------|----------------|
| I-01 | helix-deps.yaml in all 6 bricks | Each brick has the manifest |
| I-02 | upstreams/ in all 6 bricks | 4 remotes per brick |
| I-03 | Schema version field | Wire payloads carry version, server rejects unknown |
| I-04 | Mirror forks resolved | All submodules canonical, fixes published |
| I-05 | Constitution engine adopted | db-to-md sync works bidirectionally |
| I-06 | DB schema extended | New columns + tables exist |
| I-07 | Closure vocabulary fixed | 12 items retyped correctly |
| I-08 | Track/branch bindings recorded | logic_groups table populated |
| I-09 | llm_orchestrator gitlink bumped | Parent points to canonical HEAD |
| I-10 | Stale comments fixed | Comments match current behavior |
| I-11 | LLMProvider symlink removed | No stray references |

---

## Honest Boundary (§11.4.6)

- Steps I-01 through I-03 are independent of any other stage and can be done immediately.
- Step I-04 (mirror forks) is gated on operator decision A-04. The fixes exist (committed or scratchpad-preserved) but cannot be published until canonical mirrors are chosen.
- Steps I-05 and I-06 (workable-items tooling) are tooling improvements — they don't block production but should be done before the next release.
- Step I-07 (closure vocabulary) is a data fix — no code changes needed.
- Step I-11 (symlink) was verified this session: `submodules/LLMProvider` is indeed a symlink (not a duplicate directory).
