# Helix OTA — New Constitution Tech Adoption Checklist (2026-07-09)

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z

## Context

The `constitution` submodule working tree is at `e60cbde2` (59 commits ahead
of what the parent repo's git tree currently records). This checklist is
grounded in direct inspection of the constitution's new anchors + tooling
dirs, cross-checked against what THIS project (`helix_ota`) actually has
wired, per §11.4.6 (verify, don't assume).

**FACT #0 — submodule pointer itself is stale (§11.4.26 violation).**
```
$ git -C constitution rev-parse HEAD
e60cbde2e33e7d0542a937331763f6be7e739c9a
$ git ls-tree HEAD constitution
160000 commit fc4e9b8ec30909ba7e4aaeb4ab31f9ec15ab5fde  constitution
```
The parent repo's tracked submodule commit (`fc4e9b8e`) is BEHIND the
submodule working tree (`e60cbde2`). Per §11.4.26 step 7, the consuming
project's pointer MUST be bumped in the same commit as any cascade work —
this is currently out of sync and is itself the first item to fix.

---

## New anchors summarized (what each REQUIRES)

| Anchor | Requires |
|---|---|
| **§11.4.176** | Multi-track work-division: (A) exactly-once work-item/logical-group claim registry so two tracks never claim the same item; (B) capability-aware, deadlock-proof device-lock (all-or-nothing, non-blocking, TTL-reap). Only binds when the project runs ≥2 concurrent tracks/worktrees on shared resources. |
| **§11.4.182** | Every agent/subagent dispatch description AND every operator-facing work-stream reference MUST start with `(T<N>/<branch> - <alias>)`. Mechanically enforced by a **PreToolUse guard hook** (`guard-track-branch-label.sh`) that BLOCKS any Agent/Task dispatch whose description doesn't match `^\(T[0-9]+/[^)]+\) `. Requires the hook wired in `.claude/settings.json` + a convention doc. |
| **§11.4.183** | Every track/work-stream MUST maximize multi-agent usage (subagent-driven dev, independent review agents, parallel background streams) AND apply the **entire** constitution, nothing skipped — bounded by §12.6 memory + §11.4.58 agent cap + genuine parallelizability. No new tooling; it's a working-mode discipline, verified via propagation gate only. |
| **§11.4.184** | `sonar-scanner` CLI MUST be installed AND durably PATH-discoverable (`.bashrc`/`.zshrc`), proven GREEN by `sonarqube_install_check.sh` exit 0. Mandates **tooling availability**, NOT a scan on every build (distinct from the repealed §11.4.166 Semgrep mandate). Rootless podman only (§11.4.161). |
| **§11.4.185** | No scope of work/release is "done" until it receives **manual QA-team confirmation** as the FINAL step, in addition to (never instead of) every automated gate. The autonomous-loop release-tag terminal condition (§11.4.126) now REQUIRES this human confirmation; the agent must hand off and WAIT (while progressing other non-blocked work in parallel), never self-certify. |
| **§11.4.186** | Any project with more than one representation of the same tracked data (this project: `docs/workable_items.db` + `Issues.md`/`Fixed.md` + summaries + md/html/pdf/docx exports) MUST run a deterministic cross-document consistency gate (DEDUP/TIMELINE/CROSS-DOC/INTEGRITY/STRUCTURAL checks) **BEFORE** any export render, doc/DB `verify`, or doc-set commit — never an after-the-fact human audit. Engine = `constitution/scripts/doc_integrity` (Go, depth-1 reusable, consumer supplies a `checkset.yaml`). |
| **§12.12** | Before scaling parallel subagents/processes, check `ulimit -u` (soft+hard) + live thread count (`ps -L -u "$USER" \| wc -l`); thread exhaustion (`EAGAIN`/`failed to create new OS thread`) is a §12 host-safety event requiring unconditional yield/serialize, distinct from the §12.6 memory ceiling. |

---

## Tooling inventory (constitution submodule — all PRESENT, verified by `ls`)

All of the following exist under `constitution/scripts/` and `constitution/submodules/` — real files, not aspirational:

- `sonarqube/` — `sonarqube_install_check.sh`, `sonarqube_run_scan.sh`, `sonarqube_container.sh`, `sonarqube_lib.sh`, `compose/`
- `doc_integrity/` — Go module (`cmd/`, `internal/`, `wire/`), `README.md`, `NOTES_FOR_CONDUCTOR.md`; ships `wire/CM-DOC-INTEGRITY-VALIDATION.gate.sh` + `wire/doc_integrity_gate.sh` as ready-to-wire gate scripts
- `gates/` — only 3 files present (`cm_covenant_114_162_propagation.sh`, `cm_opendesign_ui_system.sh` + mutation test) — **no gate scripts yet exist here for §11.4.176/182/183/184/185/186/12.12** (those gates are "recommended" in the anchor text but not yet scaffolded as standalone files in `gates/`; §11.4.184/185/186 compliance is verified via the sonarqube/doc_integrity tool's own exit codes + a propagation grep, not a `gates/` file)
- `hooks/` — `guard-forbidden-commands.sh` (§11.4.109), `guard-track-branch-label.sh` (§11.4.182), `action_prefix_expand.sh`, `post-merge`, `test_guard_track_branch_label.sh`
- `multitrack/` — full toolkit: `multitrack_claim.sh`, `multitrack_device_lock.sh`, `multitrack_registry.sh`, `multitrack_config.sh`, `multitrack_build.sh`, `multitrack-up`/`multitrack-down`, `track_label_audit.sh`, etc. (implements §11.4.176/177/178/179/180/181/182)
- `scheduled-work-engine/` — Go engine (`cmd/`, `internal/`, `test/`), `DESIGN.md`
- `token_accounting.sh`, `subagent_tier.sh`, `enable_prompt_caching_check.sh` — §11.4.141 token-efficiency helpers
- `constitution/submodules/token_optimizer/` — Go module (`pkg/`, `helix-deps.yaml`) — depth-1 reusable engine
- `constitution/submodules/session_orchestrator/` — Go module (`alias/`, `claim/`, `helix-deps.yaml`) — depth-1 reusable engine, session/alias/claim primitives underlying `multitrack_*`

**Verified working (ran the real check script, not assumed):**
```
$ bash constitution/scripts/sonarqube/sonarqube_install_check.sh
PASS  scanner: /home/milos/Factory/software/sonar-scanner/bin/sonar-scanner — SonarScanner CLI 8.0.1.6346
PASS  podman: podman version 5.7.1
PASS  compose runtime: podman-compose
PASS  curl: present
PASS  host vm.max_map_count=2147483642 (ES-ready)
=== install check exit 0 ===
```

---

## What THIS project currently wires — verified, not assumed

| Item | Status | Evidence |
|---|---|---|
| `.claude/settings.json` (repo root) | **MISSING** | `ls .claude/settings.json` → no such file |
| §11.4.109 PreToolUse `guard-forbidden-commands.sh` hook | **NOT WIRED** | No settings file exists to register it; not present in `~/.claude/settings.json` either (global settings only has a `UserPromptSubmit` → `codegraph prompt-hook`) |
| §11.4.182 PreToolUse `guard-track-branch-label.sh` hook | **NOT WIRED** | Same — no hook registration anywhere (project or global) |
| `sonar-scanner` on PATH | **YES** | `which sonar-scanner` → `/home/milos/Factory/software/sonar-scanner/bin/sonar-scanner`; durable export found in `~/.bashrc:287-288` tagged `§11.4.184 — durable sonar-scanner PATH (ATMOSphere, added 2026-07-09)` |
| `sonarqube_install_check.sh` | **PASSES (exit 0)** | Ran it live — see output above |
| `sonar-project.properties` (project scan config) | **MISSING** | `find . -iname sonar-project.properties` → none. Not required by §11.4.184 (tooling-availability mandate, not scan-on-every-build), but needed if/when a scan is actually run |
| `docs/AGENT_GUARDRAILS.md` (project-level, per §11.4.109) | **MISSING** | Only exists at `constitution/docs/AGENT_GUARDRAILS.md` — the consuming project has never created its own copy/pointer doc |
| Project carrier files' highest anchor (`CLAUDE.md`/`AGENTS.md`/`GEMINI.md`) | **STALLED at §11.4.166** | `grep -oE '§11\.4\.[0-9]+' CLAUDE.md \| sort -u \| tail` → highest is `§11.4.166`; same for `AGENTS.md` and `GEMINI.md`. Anchors §11.4.167 through §11.4.186 (20 anchors) + §12.12 are **absent** from all three project carrier files |
| `QWEN.md` | **MISSING ENTIRELY** | `ls QWEN.md` → no such file (violates the four-carrier fleet pattern §11.4.157 establishes: CLAUDE/AGENTS/QWEN/GEMINI) |
| `tests/pre_build_verification.sh` propagation gates | **STOPS at §11.4.166** | The script literally has `run_gate "CM-COVENANT-114-153-PROPAGATION" ... "11.4.153"` through `...-166-...`; no gate lines for 167–186 or 12.12 exist |
| `docs/workable_items.db` (§11.4.93/.95 SSoT) | **PRESENT** (45KB) | This project DOES maintain multiple tracked-data representations (DB + Issues.md + Fixed.md + summaries + exports) — §11.4.186 squarely applies |
| `.{consumer}/doc_integrity/checkset.yaml` | **MISSING** | No consumer checkset registered anywhere in the project tree |
| doc_integrity gate wired into export/sync/commit seams | **NOT WIRED** | No reference to `doc_integrity`/`doc-integrity` outside `constitution/` |
| multitrack tooling in active use | **NOT WIRED / LIKELY N/A** | No project-level reference to `multitrack_*` outside `constitution/`; this project does not appear to run multiple concurrent git tracks/worktrees today, so §11.4.176/177/178/179/180/181/182 claim-registry + device-lock machinery is latent, not yet triggered — but the label convention (§11.4.182) still applies to every subagent dispatch regardless of track count |
| `docs/features/Status.md` (§11.4.153) | **PRESENT** | Already exists — good, unrelated pre-existing compliance |
| scheduled-work-engine / token_optimizer / session_orchestrator | **NOT CONSUMED** | These are constitution-internal depth-1 engines (each carries its own `helix-deps.yaml`); no project-level integration point references them. Given this project doesn't run multi-track parallel work today, these are lower priority than the propagation/hook/doc-integrity gaps |

---

## Prioritized adoption checklist

### P0 — Release blockers (mechanical propagation + verifiable gaps)

1. **Bump the constitution submodule pointer.** `git -C constitution` is at `e60cbde2`; the parent tree still records `fc4e9b8e`. Per §11.4.26 step 7 this MUST be bumped in the same commit as any cascade work.
   - Command: `git add constitution && git commit` (via `scripts/commit_all.sh`, never raw `git commit` per project convention) after the propagation work below lands.

2. **Propagate anchors §11.4.167–§11.4.186 + §12.12 into all four carrier files** (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, and the still-missing `QWEN.md`) — currently stalled at §11.4.166. Required by §11.4.35/§11.4.157 (GEMINI.md lockstep — extends to the whole carrier fleet) and directly gated by `CM-COVENANT-114-1XX-PROPAGATION` for every one of the 20 new anchors.
   - Concrete gap: `tests/pre_build_verification.sh` has explicit `run_gate "CM-COVENANT-114-153-PROPAGATION"` … `"-166-"` lines and nothing beyond. Add the missing `run_gate` lines for 167–186 + `12-12`, and restate each anchor's literal token in the four carrier files.

3. **Create `QWEN.md`** at the project root — it does not exist at all. Every propagation gate pattern in the new anchors (`CLAUDE.md + AGENTS.md + QWEN.md + GEMINI.md mirrors updated in lockstep`) assumes its existence.

4. **Wire the §11.4.109 PreToolUse guard hook** (`constitution/scripts/hooks/guard-forbidden-commands.sh`) into a project-level `.claude/settings.json` — the file does not exist at all today, so this hook (mandatory since an earlier round, not just this batch) has never been mechanically enforced for this project.
   - Command/config: create `.claude/settings.json` with a `PreToolUse` hook entry invoking `constitution/scripts/hooks/guard-forbidden-commands.sh` (inherited by reference, never copied, per §11.4.177).

5. **Wire the §11.4.182 PreToolUse guard hook** (`constitution/scripts/hooks/guard-track-branch-label.sh`) into the same `.claude/settings.json`, blocking any Agent/Task dispatch whose `description` doesn't start with `^\(T[0-9]+/[^)]+\) `.
   - Note: even for a single-track project, `<N>` and `<branch>` resolve honestly (`T1`/current branch) or `?` per §11.4.6 — the label convention still applies to every subagent dispatch this project makes.

6. **Create `docs/AGENT_GUARDRAILS.md`** at the project level (currently only exists inside `constitution/docs/`) containing the `SUBAGENT CONSTITUTIONAL PREAMBLE` + `ORCHESTRATOR PRE-ACTION CHECKLIST` sections the orchestrator pastes into every subagent dispatch, per §11.4.109.

7. **Wire the §11.4.186 doc-integrity anti-divergence gate** — this project maintains ≥2 representations of the same tracked data (`docs/workable_items.db` + `Issues.md`/`Fixed.md` + their summaries + md/html/pdf/docx exports), so §11.4.186 squarely applies and is currently completely unwired.
   - Steps: (a) `go build -o doc-integrity ./cmd/doc_integrity` from `constitution/scripts/doc_integrity/`; (b) create `.helix_ota/doc_integrity/checkset.yaml` registering the DB + Issues/Fixed + summaries as sources (schema in `constitution/scripts/doc_integrity/README.md` + `DESIGN.md §1.4`); (c) wire `doc-integrity verify` as a blocking pre-step before `scripts/export_docs.sh`, before any `sync_issues_docs`-class regeneration, and before any doc-set commit; (d) run `doc-integrity selfcheck` to prove the golden-good/golden-bad/negative-control fixtures pass (§11.4.107(10) self-validation) before trusting the gate.

### P1 — Should-wire soon (concrete but lower immediate blast radius)

8. **Add SonarQube scan config + wire into `tests/pre_build_verification.sh`.** The CLI is installed and PATH-durable (§11.4.184 tooling-availability requirement is already SATISFIED), but there is no `sonar-project.properties` and no gate invoking `sonarqube_install_check.sh` from the project's own verification pipeline. Add a `run_gate "CM-SONARQUBE-CLI-INSTALLED" bash constitution/scripts/sonarqube/sonarqube_install_check.sh` line, plus a `sonar-project.properties` at repo root (project key `helix_ota`, sources `server/`, `submodules/`) if/when an actual scan (not just tooling-presence) is desired.

9. **Document the §11.4.183 maximal-multi-agent-utilization discipline** explicitly in the project's own working-mode notes (e.g. `docs/CONTINUATION.md` or a project-specific addendum) — it's a discipline anchor with no standalone tooling to wire, but the propagation gate still needs the literal `11.4.183` present in the carrier files (covered by item 2 above) and the *practice* (parallel subagent dispatch, independent review agents) should be visibly followed in session logs.

10. **Add a §11.4.185 manual-QA-final-confirmation checkpoint** to the release/tag workflow docs (`docs/procedures/issues/Resolution.md` or the release-tag script) — currently the release-tag terminal condition (§11.4.126) has no explicit "hand off to QA team, wait for manual confirmation" step documented anywhere in this project's own procedures.

11. **Add a §12.12 thread-headroom pre-flight check** before dispatching >2-3 parallel subagents — e.g. a one-line `ulimit -u` + `ps -L -u "$USER" | wc -l` check in whatever pre-flight/pre-dispatch helper this project uses (none currently exists; could be a small addition to `scripts/` or referenced inline in `docs/AGENT_GUARDRAILS.md` once created per item 6).

### P2 — Latent / lower priority until the project actually needs them

12. **Multitrack tooling (§11.4.176–§11.4.181)** — not currently applicable since this project does not appear to run ≥2 concurrent git tracks/worktrees on shared resources today. Wire `constitution/scripts/multitrack/*` (by reference, per §11.4.177 — never copy) only when/if parallel-track work on this project actually starts. Until then, only the §11.4.182 label-prefix convention (item 5, P0) needs to apply.

13. **scheduled-work-engine / token_optimizer / session_orchestrator submodules** — these are constitution-internal depth-1 Go engines with their own `helix-deps.yaml`. No project-level consumer exists. Evaluate for adoption only if this project adopts scheduled autonomous-loop work queuing or wants the token-efficiency (§11.4.141) tooling (`token_accounting.sh`/`subagent_tier.sh`/`enable_prompt_caching_check.sh`) wired into its own session workflow — none of those are wired today either, but §11.4.141 predates this 59-commit batch and is out of this checklist's primary scope.

---

## Honest boundary (§11.4.6)

This checklist proves what is/isn't wired via direct file reads, `which`,
`grep`, and running the real `sonarqube_install_check.sh` script — it does
NOT itself constitute the fixes. Every P0/P1 item above still needs its own
§11.4.43 RED→GREEN implementation, §11.4.125/§11.4.142 independent review,
and captured evidence before being marked closed.
