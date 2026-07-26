---

description: "Task list for Pending Work Completion — DB sync, merge, carrier files, build fix, hardware documentation, final verification"

---

# Tasks: Pending Work Completion & Impeccable State

**Input**: Design documents from `specs/002-pending-completion/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story (US1=Impeccable, US2=Software, US3=Hardware)

---

## Phase 1: Setup (State Audit)

**Purpose**: Verify current project state and establish the baseline for completion

- [x] T001 Verify current branch and HEAD: `git rev-parse --abbrev-ref HEAD` and `git log --oneline -5`
- [x] T002 [P] Count Queued items in DB: `sqlite3 docs/workable_items.db "SELECT COUNT(*) FROM items WHERE status IN ('Queued','In progress');"`
- [x] T003 [P] Verify build passes before starting: `cd server && go build ./... 2>&1 | grep -c error` — capture output
- [x] T004 [P] Count uncommitted changes: `git status --short | wc -l` — must be 0 or documented
- [x] T005 [P] Run full test suite baseline: `cd server && go test ./... -count=1 -timeout 120s 2>&1 | tail -5` — save to `qa-results/pending-baseline/test.log`

---

## Phase 2: User Story 1 - Impeccable Completion State (Priority: P1)

**Goal**: Project state is consistent — DB synced, branches merged, build clean, carrier files updated.

**Independent Test**: DB Queued count ≤6 (hardware-gated only), build exits 0, main has all feature branch commits, all 5 carrier files reference 1.0.0.

### DB Sync

- [x] T006 [US1] Sync OTA-028 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T007 [P] [US1] Sync OTA-029 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T008 [P] [US1] Sync OTA-030 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T009 [P] [US1] Sync OTA-031 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T010 [P] [US1] Sync OTA-039 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T011 [P] [US1] Sync OTA-040 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T012 [P] [US1] Sync OTA-044 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T013 [P] [US1] Sync OTA-058 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T014 [P] [US1] Sync OTA-061 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T015 [P] [US1] Sync OTA-067 in `docs/workable_items.db` — set status to `Fixed (→ Fixed.md)`, modified_at to now
- [x] T016 [P] [US1] Sync OTA-069 in `docs/workable_items.db` — set status to `Fixed (→ Fixed.md)`, modified_at to now
- [x] T017 [P] [US1] Sync OTA-070 in `docs/workable_items.db` — set status to `Fixed (→ Fixed.md)`, modified_at to now
- [x] T018 [P] [US1] Sync OTA-072 in `docs/workable_items.db` — set status to `Completed (→ Fixed.md)`, modified_at to now
- [x] T019 [US1] Verify DB sync: `sqlite3 docs/workable_items.db "SELECT COUNT(*) FROM items WHERE status = 'Queued';"` — must be ≤6

### Build Fix

- [x] T020 [US1] Fix unused import `otatelemetry` in `server/internal/api/wire.go` — read the file, remove the unused import if present, run `go build ./...` to verify
- [x] T021 [US1] Verify build is clean: `cd server && go build ./...` — must exit 0 with zero errors

### Branch Merge per §11.4.188

- [x] T022 [US1] Fetch all upstreams: `git fetch --all --prune --tags`
- [x] T023 [US1] Merge main into feature branch: `git checkout feature/production-readiness && git merge origin/main` — resolve any conflicts, ensure no markers committed
- [x] T024 [US1] Run post-merge smoke: `cd server && go test ./... -count=1 -timeout 120s` — must PASS
- [x] T025 [US1] Checkout main and merge feature: `git checkout main && git merge feature/production-readiness` — must be fast-forward or clean merge
- [x] T026 [US1] Push main to ALL upstreams: `git push github main && git push gitlab main && git push gitflic main && git push gitverse main`

### Carrier Files Update per §11.4.157

- [x] T027 [P] [US1] Update `CLAUDE.md` at project root — bump Revision, set Last modified to today, update Fixed section with 16 newly-closed items and post-1.0.0 state
- [x] T028 [P] [US1] Update `AGENTS.md` at project root — bump Revision, set Last modified, update Fixed/Status sections with post-gap-closure evidence paths
- [x] T029 [P] [US1] Update `GEMINI.md` at project root — bump Revision, set Last modified, mirror CLAUDE.md changes per five-carrier lockstep
- [x] T030 [P] [US1] Update `QWEN.md` at project root (if exists, else create minimal version referencing 1.0.0) — bump Revision, set Last modified
- [x] T031 [US1] Verify five-carrier lockstep: diff `CLAUDE.md` and `GEMINI.md` for matching key sections — both must reference `helix_ota-1.0.0`

### README & Documentation

- [x] T032 [P] [US1] Update `README.md` — bump Revision to 3, update Last modified, refresh Tracked-Items table with current doc timestamps
- [x] T033 [P] [US1] Update `docs/RESUMPTION.md` — set to post-completion state: main branch, all gap closures, 1.0.0 released, 16 DB syncs done
- [x] T034 [P] [US1] Update `docs/Issues.md` if any item migrations are pending — OTA-003 already migrated, verify no other items need migration
- [x] T035 [US1] Capture evidence: `cp docs/workable_items.db qa-results/pending-baseline/workable_items-post-sync.db` — checkpoint the DB state

---

## Phase 3: User Story 2 - Software-Only Issues (Priority: P2)

**Goal**: Address remaining software-actionable items (OTA-059 VM hardening, OTA-071 QEMU e2e).

**Independent Test**: OTA-059 and OTA-071 either closed with evidence or documented with explicit remaining scope.

### VM/Emu Hardening

- [x] T036 [US2] Audit VM/emu hardening backlog: read `submodules/containers/pkg/vm/` and any hardening-related files referenced in OTA-059
- [x] T037 [P] [US2] Address any SW1-2 or SW2-2 hardening items that are software-actionable: read `submodules/containers/` for hardening branch remnants, merge cleanups
- [x] T038 [P] [US2] If VM hardening items are fully resolved by containers hardening merge (OTA-058), mark OTA-059 as `Completed (→ Fixed.md)` with evidence reference to OTA-058 merge commit

### QEMU E2E Integration

- [x] T039 [P] [US2] Investigate QEMU e2e gap: read `submodules/containers/pkg/vm/` for any QEMU-related test files
- [x] T040 [P] [US2] If QEMU e2e tests exist as stubs, upgrade them: add real `qemu-system` integration test with `-nographic` mode
- [x] T041 [P] [US2] If QEMU binary is available on host: `which qemu-system-x86_64` — if found, create a minimal e2e test booting a test image
- [x] T042 [P] [US2] If QEMU is NOT available on host, document OTA-071 as `Operator-blocked` with unblock condition: "Install qemu-system-x86_64 >= 7.0"

### Code Quality

- [x] T043 [P] [US2] Verify no remaining `// TODO` or `// SCAFFOLD` in `server/internal/api/handlers_artifact.go` — grep and report
- [x] T044 [P] [US2] Verify no remaining `// TODO` or `// SCAFFOLD` in `server/internal/api/handlers_branches.go` — grep and report
- [x] T045 [P] [US2] Verify no remaining `// TODO` or `// SCAFFOLD` in `server/internal/api/webhook.go` — grep and report
- [x] T046 [P] [US2] Sync OTA-059 and OTA-071 in DB with appropriate status (Closed or Operator-blocked)

---

## Phase 4: User Story 3 - Hardware-Gated Documentation (Priority: P3)

**Goal**: Every hardware-gated item has documented unblock conditions.

**Independent Test**: All 4 hardware-gated items (OTA-038, 041, 042, 043) have updated descriptions with required hardware and unblock conditions.

- [x] T047 [P] [US3] Update OTA-038 description in `docs/workable_items.db` — add unblock condition: "RK3588 Orange Pi 5 Max running Linux with U-Boot bootloader accessible"
- [x] T048 [P] [US3] Update OTA-041 description in `docs/workable_items.db` — add unblock condition: "Physical RK3588 device with U-Boot fw_setenv accessible; research complete"
- [x] T049 [P] [US3] Update OTA-042 description in `docs/workable_items.db` — add unblock condition: "RK3588 Orange Pi 5 Max with both A/B slots functional and boot_control HAL accessible"
- [x] T050 [P] [US3] Update OTA-043 description in `docs/workable_items.db` — add unblock condition: "RK3588 Orange Pi 5 Max running Android 15 AOSP build with ADB debugging enabled"
- [x] T051 [US3] Verify all 4 hardware-gated items have unblock conditions: `sqlite3 docs/workable_items.db "SELECT ota_id, description FROM items WHERE ota_id IN ('OTA-038','OTA-041','OTA-042','OTA-043');"` — output must contain "RK3588" or "hardware" for each

---

## Phase 5: Polish & Final Verification

**Purpose**: Constitution sweep, full test suite, evidence capture, final commit and push.

- [x] T052 Run constitution inheritance gate: `bash tests/test_constitution_inheritance.sh` — capture PASS evidence in `qa-results/pending-final/constitution-inheritance.txt`
- [x] T053 Run constitution sweep: `bash tests/pre_build_verification.sh 2>&1 | tail -20` — capture in `qa-results/pending-final/constitution-sweep.log`
- [x] T054 Run core server test suite: `cd server && go test ./... -count=1 -timeout 120s 2>&1 | tail -10` — capture in `qa-results/pending-final/server-tests.log`
- [x] T055 Verify gap tracker is current: `bash scripts/track_velocity.sh` — append updated velocity to `docs/research/production_planning_20260726/velocity.tsv`
- [x] T056 Run feature evidence collection: `bash scripts/collect_feature_evidence.sh 2>&1 | tail -20` — capture in `qa-results/pending-final/feature-evidence.log`
- [x] T057 Verify zero uncommitted work: `git status --short` — must be empty (all work committed)
- [x] T058 Commit all pending work: `git add -A && git commit -m "feat: pending work completion — DB sync (13 items), carrier files updated, build fix, hardware documentation"`
- [x] T059 Push to ALL upstreams: `git push github main && git push gitlab main && git push gitflic main && git push gitverse main`
- [x] T060 Verify post-completion DB state: `sqlite3 docs/workable_items.db "SELECT status, COUNT(*) FROM items GROUP BY status ORDER BY COUNT(*) DESC;"` — capture final counts in `qa-results/pending-final/db-final-state.txt`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — verifies current state
- **US1 Impeccable (Phase 2)**: Depends on Setup — DB sync, merge, build fix, carrier files
- **US2 Software (Phase 3)**: Can run in parallel with US1 after Build Fix (T020-T021)
- **US3 Hardware (Phase 4)**: Can run in parallel with US1 + US2 (independent documentation)
- **Polish (Phase 5)**: Depends on ALL previous phases

### Within Each Phase

- US1 DB sync (T006-T018): ALL [P] — can run fully parallel
- US1 Merge (T022-T026): SEQUENTIAL — must follow exact order
- Carrier files (T027-T030): ALL [P] — different files
- US2 Hardware doc (T047-T050): ALL [P] — independent items

### Parallel Opportunities

- Phases 2, 3, and 4 can all run in parallel within US1 (after T002 baseline)
- All 13 DB sync tasks (T006-T018) are fully parallel
- All 4 carrier file updates (T027-T030) are fully parallel
- All 4 hardware doc updates (T047-T050) are fully parallel

---

## Implementation Strategy

### Quick Win (MVP)

1. Complete Phase 1: Setup
2. Complete Phase 2: US1 — all DB syncs, build fix, merge, carrier files
3. **STOP and VALIDATE**: DB count ≤6, build clean, main merged
4. Complete Phase 3-4: US2 + US3 in parallel (software + hardware docs)
5. Complete Phase 5: Polish, commit, push to all upstreams
