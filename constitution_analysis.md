# Constitution Submodule — Comprehensive Requirements Analysis

**Generated**: 2026-07-26
**Constitution Revision**: 39 (last modified 2026-07-11T12:00:00Z)
**Documents analyzed**: Constitution.md, AGENTS.md, CLAUDE.md, CLAUDE_ANCHORS_FULL.md, AGENT_GUARDRAILS.md, registry.yaml, subagent_tiering.yaml

---

## 1. BUILD / CI / CD REQUIREMENTS

### 1.1 Containerized Build
- **§11.4.173**: EVERY build of EVERY component MUST run INSIDE a specialized build container via `vasic-digital/containers` submodule — NEVER bare host
- **§11.4.173**: Build containers MUST be DISTRIBUTED to remote build host, artifacts brought back
- **§11.4.173**: Building outside a container or on bare host is FORBIDDEN — release blocker
- **§11.4.76**: ALL containerized workloads MUST use `vasic-digital/containers` submodule as sole orchestration layer
- **§11.4.76**: Container submodule consumed via `replace` directive + pinned commit SHAs in production
- **§11.4.76**: Boot infra on-demand via Submodule's `pkg/boot` + `pkg/compose` + `pkg/health` APIs (on-demand-infra invariant)
- **§11.4.76**: Extend the Containers submodule via upstream PR for missing features — never reimplement in-project
- **§11.4.161**: MUST use Podman in rootless mode for ALL containerized workloads
- **§11.4.161**: Docker rootful mode, sudo, or any escalation to root for container management is STRICTLY FORBIDDEN
- **§11.4.161**: Rootless exception only if target platform has no rootless option AND constraint documented per §11.4.112
- **§12.11**: Containerized build path uses maximal SAFE fraction of host capacity, computed DYNAMICALLY per host
- **§12.11**: Auto-detect nproc/MemTotal/RLIMIT_NPROC — never hardcode
- **§12.11**: Reserve explicit host headroom (cores+RAM); scale -j against BOTH memory model AND process rlimit
- **§12.11**: No fixed low -j for containerized path; §12.6 60% ceiling SCOPED to user.slice-resident work

### 1.2 CI/CD Disabled
- **§11.4.156**: ALL server-side CI/CD automation MUST be DISABLED — no GitHub Actions, GitLab pipelines, Jenkins, CircleCI, etc.
- **§11.4.156**: Zero active `.github/workflows/*.yml|yaml` / `.gitlab-ci.yml` at root of any governed repo
- **§11.4.156**: "Disabled" = push triggers ZERO runs (delete OR rename to non-trigger name)
- **§11.4.156**: No new CI may be added — release blocker
- **§11.4.156**: Pre-push verify `git ls-files | grep -E '^\.github/workflows/.*\.ya?ml$|^\.gitlab-ci\.yml$'` empty for authored repos
- **§11.4.75 Layer 5**: Remote CI surfaces DISABLED; enforcement migrated to LOCAL pre-build-verification + meta-test ritual

### 1.3 Build Hygiene
- **§11.4.30**: Build artifacts MUST NOT be versioned (`bin/`, `build/`, `dist/`, `target/`, `*.exe`, `*.so`, `*.class`, `*.pyc`)
- **§11.4.30**: Cache files MUST NOT be versioned (`__pycache__/`, `node_modules/`, `.gradle/`, `.terraform/`)
- **§11.4.30**: `.gitignore` line alone is insufficient — no file matching forbidden patterns may be currently tracked
- **§11.4.30**: Pre-commit inspect `git diff --staged` + `git status` BEFORE commit
- **§11.4.30**: Forbidden-class hits abort the commit until fixed
- **§11.4.77**: Every `.gitignore` entry excluding >~100 MiB OR essential artifact MUST carry documented + automated regeneration/re-obtainment mechanism
- **§11.4.77**: Required: `.gitignore-meta/<entry-slug>.yaml` + `scripts/setup.sh` entry + pre-build gate + README
- **§11.4.77**: No bare `.gitignore` additions without mechanism (release blocker)
- **§11.4.121**: No commit while build/packaging writes artifacts INTO tracked directories
- **§11.4.121**: Commit MUST be deferred until the writing step has COMPLETED
- **§11.4.82(D)**: Persistent build caches outside containers — ccache + Soong/sccache/Gradle daemon state bind-mounted to host
- **§11.4.82(E)**: Module-only rebuild for `CONFIG_*=m` driver patches
- **§11.4.82(H)**: Disable auto git-gc (`gc.auto 0`) in concurrent multi-agent repos
- **§11.4.96**: SAFE-during-build catalogue — docs, scripts, gates, on-device test scripts, constitution edits, pre-build + meta-test execution
- **§11.4.96**: UNSAFE during build — git checkout/reset, mass deletes/renames under source, submodule pointer updates, out/ mutations, make clean, container destruction
- **§11.4.96**: Conductor MUST dispatch ≥1 (A)-(K) item per pause point during long builds

### 1.4 Build Resource Tracking
- **§11.4.24**: Every build exceeding 1 minute MUST run host-side resource sampler
- **§11.4.24**: Samples `/proc/meminfo` + `/proc/loadavg` + `/proc/stat` + `/proc/diskstats` at fixed interval (recommended 5s)
- **§11.4.24**: Compute min / max / mean / p95 per metric on stop
- **§11.4.24**: Append one TSV row per build to registry; regenerate Markdown report (Stats.md)
- **§11.4.24**: Top of report surfaces ever-values (min / max / mean across all tracked builds)
- **§11.4.24**: Per-build entries most-recent-first with SUCCESS/FAIL/UNKNOWN + reason
- **§11.4.24**: Stats.md exported to Stats.html + Stats.pdf via normal export pipeline
- **§11.4.24**: Triple committed via lightweight doc-sync wrapper (§11.4.22)
- **§11.4.24**: Sampler MUST stay under 50 MB RSS / 5% CPU
- **§11.4.24**: Stop hook called from BOTH success AND failure paths of build wrapper
- **§11.4.24**: Gate `CM-BUILD-RESOURCE-STATS-TRACKER`

### 1.5 Build Readiness
- **§11.4.110**: Single deterministic READY-FOR-BUILD verdict gates the rebuild
- **§11.4.110**: Diff-driven change-impact + clash detector cross-checks every newly-introduced second-artifact dependency
- **§11.4.110**: Coverage-completeness is a gate — every changed file maps to ≥1 gate + ≥1 deployed-target test + ≥1 paired §1.1 mutation
- **§11.4.110**: Two-speed honesty — grep-speed always-on gates vs REQUIRES_BUILD heavy gates
- **§11.4.110**: Every gate + wired analyzer is anti-bluff by paired §1.1 mutation
- **§11.4.82(C)**: 30-second pre-flight before rebuild orchestrators (device reachable, sinks reachable, memory budget, no stale locks, no orphan processes)

---

## 2. SECURITY REQUIREMENTS

### 2.1 Credential Handling
- **§11.4.10**: Credentials NEVER live in any file git tracks
- **§11.4.10**: `.env`, `.env.*`, `*.env`, `.<service>.env`, `*.<service>.env` MUST be gitignored
- **§11.4.10**: `scripts/testing/secrets/*` with `.example` + `README.md` exception
- **§11.4.10**: Tests load credentials AT EXECUTION TIME from operator-populated files
- **§11.4.10**: If credential file is missing, test SKIPs — never proceeds without credentials
- **§11.4.10**: Test scripts MUST NEVER print, log, or include credentials in output
- **§11.4.10**: One file per service (per-service file separation)
- **§11.4.10**: `.env` files are `chmod 600`, parent directory is `chmod 700`
- **§11.4.10**: Rotation on suspected leak
- **§11.4.10**: `.gitignore` verified before every commit — `git ls-files --cached | grep -E "\\.env$"` MUST show no real `.env` files
- **§11.4.10.A**: Pre-store credential leak audit — grep every tracked file + entire git history for literal credential value(s) BEFORE storing
- **§11.4.10.A**: Report findings to operator BEFORE storing the new value
- **§11.4.10.A**: If leak found: open forensic incident + redact in-place + record OPERATOR ACTION REQUIRED for rotation
- **§11.4.10.A**: Extend pre-push hook credential-pattern grep in same commit as redaction
- **§11.4.10.A**: Gate `CM-PRE-STORE-CREDENTIAL-AUDIT` (recommended per consuming project)
- **§11.4.30**: Sensitive data files forbidden from version control (`.pem`, `.key`, `id_rsa*`, `.netrc`, `secrets/`, `api_keys.sh`)
- **§11.4.30**: `.env` leak is BOTH §11.4.30 and §11.4.10 violation — rotation + post-mortem required
- **§11.4.10**: Pre-push hook credential-pattern grep MUST catch escaped pattern classes

### 2.2 Secret-Free Indexing
- **§11.4.78/§11.4.79**: CodeGraph index MUST NEVER include credentials/secrets — `.env`, `.env.*`, `**/*.env`, `secrets/`, `**/*secret*`, keystores, signing-keys, service-account JSON
- **§11.4.78/§11.4.79**: A secret reaching the index is a §11.4.10 violation, never negotiable
- **§11.4.78**: CodeGraph `exclude` list MUST exclude every credential/secret path

### 2.3 Target Hardware Safety
- **§11.4.133**: Every change to the TARGET system MUST be safe for BOTH the System AND the hardware
- **§11.4.133**: MUST NOT brick, boot-loop, corrupt data, or render device unrecoverable
- **§11.4.133**: MUST NOT exceed safe electrical/thermal/voltage/clock limits
- **§11.4.133**: Reversible-first — verify irreversible high-blast-radius changes against known-good + capture pre-op backup BEFORE applying
- **§11.4.133**: NO unverified hardware-control writes (voltage/clock/regulator/thermal-throttle/current-limit)
- **§11.4.133**: Thermal/perf changes MUST respect device cooling design, validated by captured thermal evidence
- **§11.4.133**: Flashing MUST use sanctioned tool + freshly-built integrity-verified image
- **§11.4.133**: Unprovable-safety ⇒ blocked (treated as UNSAFE)

### 2.4 No Force-Push
- **§11.4.113**: Force-push is STRICTLY FORBIDDEN with NO exception against EVERY repository
- **§11.4.113**: No operator-approval path, no "after merge-first audit" path
- **§11.4.113**: Mandated 6-step integration: fetch → base = latest `main` → merge every change (union, preserve both sides) → resolve conflicts → commit merge → push to ALL upstreams (every push is fast-forward)
- **§9.2**: Force-pushing requires explicit user authorization every time
- **§9.2**: Automated push wrappers MUST NOT auto-force-push as fallback
- **§9.2**: Force-push to `main` only authorized after §9.1.5 gate passed
- **§9.2**: Every force-push event recorded in `docs/changelogs/<tag>.md` under "Force-push audit" section
- **§11.4.41**: Any force-push MUST be preceded by mechanical 4-step merge-first pipeline: (1) fetch every remote, (2) integrate every divergent commit locally, (3) audit the integrated tree (no conflict markers, no file dropped, tests still pass), (4) force-push
- **§11.4.41**: Two-gate composition — CONST-043 operator-approval AND §11.4.41 mechanical gate BOTH required
- **§11.4.41**: Verification artifact in `docs/changelogs/<tag>.md`
- **§11.4.75**: `commit-msg` hook enforces `Bypass-rationale: <reason>` footer on `--no-verify`
- **§11.4.75**: `docs/audit/bypass_events.md` accumulates audit trail
- **AGENTS.md top-level invariant 7**: Never force-push — requires explicit per-session human authorization AND green §9.1.5 post-op gate

### 2.5 Host Session Safety (§12)
- **§12**: MUST NOT use more than 60% of total system RAM
- **§12**: Heavy work wrapped in bounded execution scopes so OOM-kills only the scope
- **§12.1**: Forbidden: suspending the host, hibernating, logging out the user
- **§12.1**: Forbidden: unbounded-memory operations inside `user@<uid>.service` cgroup exceeding ~4 GiB RSS
- **§12.1**: Forbidden: programmatic rfkill toggles, lid-switch handlers, power-button handlers
- **§12.1**: Forbidden: disabling session managers "to make things faster"
- **§12.2**: Required safeguards: source host-safety helper, call pre-flight check and abort on failure, wrap >~4 GiB RSS in bounded scope, cap parallelism to available RAM
- **§12.3**: Container hygiene rules apply
- **§12.6**: 60% MAXIMUM memory ceiling — no escape hatch
- **§12.10**: `docs/CONTINUATION.md` MUST exist at project root, updated in same commit as any non-trivial state change
- **§12.12**: Check thread headroom (`ulimit -u` + live `ps -L` count) before scaling parallel subagents
- **§12.12**: Thread exhaustion = §12 host-safety event — YIELDS UNCONDITIONALLY
- **§12.12**: Never conflate EAGAIN/fork failure with OOM (§11.4.6)
- **§12.12**: Low headroom ⇒ SERIALIZE, never dispatch blind
- **§9**: Absolute codebase and data safety — zero risk, zero loss

---

## 3. DOCUMENTATION REQUIREMENTS

### 3.1 Document Sync & Export
- **§11.4.12**: Every auto-generated document MUST be regenerated in the SAME commit as any edit to its source
- **§11.4.12**: All three file types (.md + .html + .pdf) MUST stay in sync at all times
- **§11.4.12**: Enforced by pre-build gate checking mtime ordering and content hash agreement
- **§11.4.65**: Every Markdown document NOT in source-code tree MUST have synchronized `.html` and `.pdf` siblings
- **§11.4.65**: Scope includes project-root `*.md`, `docs/**/*.md`, `scripts/**/*.md`, owned-submodule top-level README.md/CLAUDE.md/AGENTS.md/CHANGELOG.md, `constitution/**/*.md`
- **§11.4.65**: Excludes `external/**`, `prebuilts/**`, `packages/modules/**`, `kernel-5.10/**`, `out/**`, `build/**`, source-code trees, third-party submodules
- **§11.4.65**: HTML + PDF mtime MUST be ≥ source `.md` mtime at all times
- **§11.4.65**: Every edit triggers regeneration via `scripts/testing/sync_all_markdown_exports.sh`
- **§11.4.65**: Pre-build gates `CM-UNIVERSAL-MARKDOWN-EXPORT-SYNC` + `CM-COVENANT-114-65-PROPAGATION`
- **§11.4.59**: README.md is a §11.4.12-class always-sync document — HTML + PDF exports refresh on every update
- **§11.4.59**: README carries §11.4.44 revision header and Documentation Map section
- **§11.4.59**: Pre-build gate `CM-README-EXPORT-SYNC` enforces mtime parity
- **§11.4.60**: Eight doc classes (Issues, Issues_Summary, Fixed, Fixed_Summary, CONTINUATION, README, every Status.md, every Status_Summary.md) MUST be in sync across `.md` + `.html` + `.pdf`
- **§11.4.60**: Composite pre-build gate `CM-DOCS-COMPOSITE-SYNC` walks all 8 classes
- **§11.4.60**: FAILs if ANY `.html` or `.pdf` mtime is older than its `.md`
- **§11.4.106**: Docs Chain (`vasic-digital/docs_chain` engine) is the canonical mechanical enforcer of documentation-sync mandates
- **§11.4.106**: Use the engine, never ad-hoc scripts — consumed by reference, NEVER copied
- **§11.4.106**: Consumer registers its chains as data via `.docs_chain/contexts/*.yaml`
- **§11.4.106**: `verify` is the deterministic CI/pre-build gate
- **§11.4.168**: Every exported document (HTML/PDF/DOCX) MUST pass INDEPENDENT validation across THREE layers: CONTENT, TEXTUAL, FULL VISUAL
- **§11.4.168**: CONTENT — export faithfully carries source's intent + data
- **§11.4.168**: TEXTUAL — human-readable, NO raw markup/diagram-source leaking as body text
- **§11.4.168**: FULL VISUAL — embedded diagrams render as IMAGES, no overlap/cut-off/garble
- **§11.4.168**: Analyzer self-validated golden-good/golden-bad per §11.4.107(10)
- **§11.4.168**: Gate `CM-COVENANT-114-168-PROPAGATION` + `CM-EXPORTED-DOC-VISUALLY-VALIDATED`
- **§11.4.153**: Feature Status docs add DOCX export to the §11.4.65 HTML+PDF set

### 3.2 Document Revision Headers
- **§11.4.44**: Every IN-scope tracked Markdown document carries `**Revision:** N` + `**Last modified:** YYYY-MM-DDTHH:MM:SSZ` below H1
- **§11.4.44**: Scope includes Issues.md, Issues_Summary.md, Fixed.md, Fixed_Summary.md, CONTINUATION.md, docs/guides/**, docs/research/**, docs/scripts/**, docs/changelogs/**, docs/superpowers/plans/**, docs/hardware/**, all other `docs/*.md`
- **§11.4.44**: CLAUDE.md / AGENTS.md / README / LICENSE / VERSION / rendered HTML / rendered PDF are OUT of scope
- **§11.4.44**: Agents MUST read the revision literal from document head, NEVER infer freshness from mtime or commit log
- **§11.4.44**: Agents writing MUST invoke `scripts/doc_revision_bump.sh <file>` before staging
- **§11.4.44**: Agents MUST NOT manually edit revision number — only bump script is authoritative
- **§11.4.44**: Pre-build gates `CM-DOC-REVISION-HEADER-PRESENT` + `CM-COVENANT-114-44-PROPAGATION`
- **§11.4.73**: Main specification document (if present) uses two-axis versioning: primary for major rewrites, secondary (`Revision`) for additive requirements
- **§11.4.73**: Every operator-mandated requirement MUST land in the spec as part of the work that implements it

### 3.3 Script Documentation
- **§11.4.18**: Every Bash/shell/POSIX-sh script MUST carry in-source documentation block (Purpose, Usage, Inputs, Outputs, Side-effects, Dependencies, Cross-references)
- **§11.4.18**: Every script MUST have external user guide under `docs/scripts/<script-name>.md`
- **§11.4.18**: Both in-source block AND external guide MUST be updated in SAME commit as script modification
- **§11.4.18**: Gate `CM-SCRIPT-DOCS-SYNC` enforces

### 3.4 Feature Status Documentation
- **§11.4.153**: Comprehensive feature Status document set under `docs/features/` — Status.md + Status_Summary.md
- **§11.4.153**: Enumerates EVERY system component, EVERY client app/binary/surface, EVERY feature
- **§11.4.153**: Per-feature fields: Component/Feature/Category/Implementation/Wiring/Real-use/Tests-coverage/Validation/Video-recording confirmation
- **§11.4.153**: Every user-visible "confirmed" claim backed by recorded real-use video
- **§11.4.153**: Video-analysis remediation loop — every defect surfaces triggers fix→retest→re-record→clean GO
- **§11.4.153**: Four-format export: HTML + PDF + DOCX
- **§11.4.153**: §11.4.86 drift-proof fingerprint (sha256 of sorted feature-key roster AND sorted video-artefact roster)
- **§11.4.153**: MP4 format REQUIRED (H.264)
- **§11.4.153**: Window-specific capture ONLY (§11.4.159(A))
- **§11.4.153**: Vision validation REQUIRED (§11.4.159(D))
- **§11.4.45**: Every non-trivial domain integration MUST have `docs/<domain>/<integration>/Status.md`
- **§11.4.45**: Auto-synced HTML+PDF + auto-colorized per §11.4.23
- **§11.4.45**: Captured-evidence-driven status table (PASS/FAIL/SKIP/PENDING_FORENSICS/OPERATOR-BLOCKED)
- **§11.4.45**: Pre-build gates `CM-COVENANT-114-45-PROPAGATION` + `CM-AF-INTEGRATION-STATUS-DOCS`
- **§11.4.86**: Status docs backed by tracked roster or asset corpus MUST resync out of the box when a member changes
- **§11.4.86**: Drift-proof fingerprint = sha256 of sorted member list (NOT mtime)
- **§11.4.86**: Pre-build gate FAILs when live fingerprint ≠ persisted

### 3.5 README Documentation
- **§11.4.57**: README.md MUST contain "Tracked-Items + Status Documents" section with table linking to Issues/Issues_Summary/Fixed/Fixed_Summary/CONTINUATION/all Status.md + Status_Summary.md pairs
- **§11.4.57**: Table columns: Document, Last modified, Revision, Markdown, HTML, PDF
- **§11.4.57**: Generator: `scripts/testing/update_readme_doc_links.sh`
- **§11.4.57**: Pre-build gates `CM-README-DOC-LINK-SECTION-PRESENT` + `CM-README-DOC-LINK-ROWS-COMPLETE` + `CM-README-DOC-LINK-FRESHNESS`
- **§11.4.57**: Gate `CM-COVENANT-114-57-PROPAGATION`

### 3.6 Procedure Documentation
- **§11.4.63**: Every workable-item action (open/update/close/reopen/migrate) MUST follow canonical procedure document at `docs/procedures/issues/<Action>.md`
- **§11.4.63**: Closed-set of 5 procedure docs: Creation.md, Updating.md, Resolution.md, Reopening.md, Migration.md
- **§11.4.63**: Each carries §11.4.44 revision header + HTML + PDF exports
- **§11.4.63**: Pre-build gate `CM-PROCEDURES-DOCS-PRESENT`

### 3.7 Changelog
- **§5**: Every tagged release MUST ship:
  1. New `docs/changelogs/<tag>.md` entry
  2. Exports to `docs/changelogs/<tag>.{html,json,txt}`
  3. Update to cumulative `docs/changelogs/CHANGELOG.md` (reverse chronological)
- **§5**: Project SHOULD provide a script (`scripts/testing/export_changelog.sh <tag>`)

### 3.8 Documentation Updates for Every Change
- **§6**: Every new feature, fix, or infrastructure change MUST update:
  1. CLAUDE.md / AGENTS.md Applied Fixes table
  2. `docs/guides/` for user-visible or developer-reachable behaviour changes
  3. Architecture diagrams, flowcharts, and reference docs
  4. Per-version changelog (§5)
- **§6**: Documentation drift after a fix is a Constitution violation

### 3.9 Research Documentation
- **§11.4.8**: Every non-trivial fix's commit message MUST cite research sources
- **§11.4.8**: At least one external link OR literal "NO external solution found — original work"
- **§11.4.8**: Research surface includes: official docs, vendor technical guides, open-source codebases, coding tutorials, issue trackers
- **§11.4.99**: Pre-commit cross-reference — agent MUST fetch LATEST official online docs for every operator-facing instruction doc
- **§11.4.99**: Cite source URL + date in "## Sources verified" footer
- **§11.4.99**: Re-verification cadence: 6 months (90 days for risk-classified services)
- **§11.4.99**: Risk-classified services (messengers/cloud/payment/AI/code-hosting/package-managers) — 90-day staleness limit
- **§11.4.150**: Deep multi-angle web research per change/issue BEFORE declaring fixed or structural
- **§11.4.150**: ≥2 genuinely-distinct angles per pass
- **§11.4.150**: ALWAYS in parallel with main stream
- **§11.4.150**: Gate `CM-COVENANT-114-150-PROPAGATION` + `CM-DEEP-RESEARCH-PER-ISSUE`

### 3.10 Session Resumption
- **§11.4.131**: Every project MUST maintain single canonical always-current session-resumption file at fixed project-declared standard path
- **§11.4.131**: SHORT + FULL variants + §11.4.65 export + §11.4.44 revision header
- **§11.4.131**: Gate `CM-COVENANT-114-131-PROPAGATION` + `CM-SESSION-RESUMPTION-FILE-PRESENT`
- **§11.4.127**: When fresh session needed, agent MUST ALWAYS prepare + provide ready-to-paste resumption prompt valid for that EXACT moment
- **§11.4.127**: Variants: SHORT first-sentence + FULL block on demand
- **§11.4.127**: Must point to live handoff docs, state PHASE + NEXT + terminal goal, embed exact live-state anchors, restate binding constraints
- **§11.4.127**: Moment-valid never generic — missing/stale/generic = violation

### 3.11 Cross-Document Consistency
- **§11.4.186**: Any project maintaining more than one representation of same tracked data MUST enforce cross-document CONSISTENCY as deterministic gate BEFORE any export/commit
- **§11.4.186**: Five decidable check families: DEDUP, TIMELINE, CROSS-DOC, INTEGRITY, STRUCTURAL
- **§11.4.186**: Drift-proof fingerprint (sha256 of sorted authoritative inputs)
- **§11.4.186**: Self-validated analyzer — golden-good + golden-bad + negative-control
- **§11.4.186**: Supersedes ad-hoc audit docs
- **§11.4.186**: Gate `CM-COVENANT-114-186-PROPAGATION` + `CM-DOC-INTEGRITY-VALIDATION`

---

## 4. TESTING REQUIREMENTS

### 4.1 Test Coverage Mandate (§1)
- **§1**: Every change MUST ship with tests that prove:
  1. Change is present in source (pre-build/pre-merge gate)
  2. Change survives compilation/packaging (post-build gate)
  3. Change behaves correctly at runtime (runtime/integration/on-device test)
  4. The gate itself is not bluffing (meta-test/mutation)
- **§1**: All four layers of coverage required before merging
- **§1.1**: Every new gate MUST be paired with a mutation entry that temporarily breaks the assertion and proves the gate now FAILs
- **§1.1**: If mutation round does not turn PASS → FAIL, the gate is a sham and must be rewritten
- **§1.1**: Single most important rule — every subsequent §11.4.x clause is downstream of it
- **§11.4.4(b)**: Every fix lands with four-layer test coverage: pre-build, post-build, runtime/on-device, meta-test paired mutation
- **§11.4.85**: Every fix MUST ship with full-automation stress + chaos test suites
- **§11.4.85**: Stress closed-set: sustained load (N≥100 or ≥30s), concurrent contention (N≥10 parallel), boundary conditions
- **§11.4.85**: Chaos closed-set per fix-class: process-death injection, network-fault injection, input-corruption injection, resource-exhaustion injection, state-corruption injection
- **§11.4.85**: Helper library `stress_chaos.sh` provides primitives
- **§11.4.85**: 4-layer per §11.4.4(b): pre-build gate + paired meta-test + on-device test + HelixQA Challenge
- **§11.4.169**: Closed enumerated test-type set (13 types) REQUIRED where domain warrants:
  1. Unit (ONLY layer mocks/stubs/fakes permitted)
  2. Integration (real components, no fakes beyond unit)
  3. E2E (full user journey)
  4. Full-automation (fully autonomous, re-runnable, NO manual step)
  5. Challenges (vasic-digital/challenges submodule)
  6. HelixQA (HelixDevelopment/HelixQA submodule)
  7. DDoS/load-flood
  8. Security (authn/authz, injection/taint, secret-leak, transport+crypto, dependency CVEs)
  9. Stress+chaos
  10. Concurrency/atomicity
  11. Race-condition/deadlock
  12. Memory (leak census over soak, peak-RSS ceilings)
  13. Benchmarking/performance (p50/p95/p99 latency + throughput + resource cost vs baseline)
- **§11.4.169**: Each type's PASS cites rock-solid captured physical evidence
- **§11.4.169**: §11.4.169 is strict expansion of §11.4.27
- **§11.4.169**: Gate `CM-COVENANT-114-169-PROPAGATION` + `CM-MANDATORY-TEST-TYPES-COVERED`

### 4.2 No-Fakes-Beyond-Unit-Tests
- **§11.4.27(A)**: Mocks/stubs/fakes/placeholders/TODOs/FIXMEs PERMITTED ONLY in unit-test sources
- **§11.4.27(A)**: Non-unit tests MUST exercise the real, fully implemented system
- **§11.4.27(A)**: Production code MUST NOT import mock paths
- **§11.4.27(A)**: Gate `CM-NO-FAKES-BEYOND-UNIT-TESTS`
- **§11.4.27(B)**: Codebase MUST be covered by every supported test type domain warrants
- **§11.4.27(B)**: Required dependency submodules: Challenges + HelixQA + any other functionality submodules under vasic-digital/HelixDevelopment orgs
- **§11.4.27(B)**: Pointers bumped to upstream HEAD in same commit as cascade work

### 4.3 Full Automation Coverage
- **§11.4.25**: Every feature/functionality/flow/use case/edge case/service/application MUST be covered by automation tests proving 6 invariants:
  1. Anti-bluff posture with captured runtime evidence
  2. Proof of working end-to-end on target topology
  3. Implementation matches documented promise
  4. No open issues/bugs surfaced
  5. Full documentation kept in sync
  6. Four-layer test floor per §1
- **§11.4.25**: Consuming projects publish coverage ledger (feature × platform × invariant × status)
- **§11.4.25**: No escape hatch
- **§11.4.98**: Every test MUST be self-driving end-to-end — PASS/FAIL/SKIP-with-reason without human action after startup
- **§11.4.98**: Single exception: one-time credential bootstrap OUTSIDE test execution
- **§11.4.98**: Manual-action commit BLOCKED at release-gate
- **§11.4.98**: NON-COMPLIANT test not rewritten in 30 days → §11.4.90 Obsolete
- **§11.4.52**: Every user-facing feature MUST have at least one autonomous validation path
- **§11.4.52**: Operator-attended tests are SUPPLEMENTARY, never PRIMARY
- **§11.4.52**: Coverage ledger classifies each feature: AUTONOMOUS_VERIFIED / AUTONOMOUS_DESIGNED / OPERATOR_ATTENDED_ONLY / NOT_APPLICABLE
- **§11.4.52**: OPERATOR_ATTENDED_ONLY blocks release until migrated
- **§11.4.52**: Pre-build gates `CM-COVENANT-114-52-PROPAGATION` + `CM-AF-AUTONOMOUS-PATH-PER-FEATURE`

### 4.4 Anti-Bluff Covenant
- **§11.4**: Bar for shipping is NOT "tests pass" but "users can use the feature"
- **§11.4**: Every PASS MUST carry positive evidence captured during execution
- **§11.4**: Metadata-only PASS, configuration-only PASS, absence-of-error PASS, grep-based PASS without runtime evidence = critical defects
- **§7.1**: Every runtime test MUST satisfy ALL FIVE constraints:
  1. Real ACTION — invoke at least one user-visible action via anti-bluff helper
  2. State DELTA — capture state BEFORE and AFTER, assert delta matches expectation
  3. POSITIVE EVIDENCE — PASS condition MUST be positive evidence, NEVER absence-of-error
  4. UNIQUE EVIDENCE TOKEN — embed per-run UUID into action for mutable state tests
  5. Audio/video features REQUIRE captured evidence
- **§11.4.1**: FAIL-bluffs equally forbidden — test MUST fail ONLY for genuine product defects
- **§11.4.1**: Script crashes (undefined variable, regex error, malformed assertion) must be fixed at source layer, not call sites
- **§11.4.2**: Every PASS for user-visible feature MUST be cross-checked against captured recording + action timeline
- **§11.4.2**: Project requires recording wrapper + action-timeline emitter + frame/audio analyzer
- **§11.4.5**: Audio quality analysis: presence, channel count, sample rate + bit depth, glitch census, coexistence-artifact census
- **§11.4.5**: Video quality analysis: presence (non-zero frame count), routing target, frame health, obstruction census, resolution + codec
- **§11.4.6**: No-guessing mandate — forbidden vocabulary: likely, probably, maybe, might, possibly, presumably, seems, appears, guess, seemingly, apparently, perhaps, supposedly, conjectured
- **§11.4.6**: Either prove cause with captured evidence or mark UNCONFIRMED/PENDING_FORENSICS with tracked-task ID
- **§11.4.7**: Demotion requires positive evidence captured under SAME conditions (same target, same build, same cycle position, same load profile)
- **§11.4.69**: Universal sink-side positive-evidence taxonomy — every user-visible feature maps to taxonomy entry
- **§11.4.69**: Closed-set taxonomy features: audio_output, audio_input, video_display, network_throughput, etc.
- **§11.4.69**: Canonical PASS helper: `ab_pass_with_evidence`
- **§11.4.69**: SKIP reasons closed-set: geo_restricted, operator_attended, hardware_not_present, topology_unsupported, network_unreachable_external, feature_disabled_by_config
- **§11.4.69**: Bare `ab_pass` deprecated — FAIL post-2026-06-19
- **§11.4.69**: Three pre-build gates: `CM-SINK-EVIDENCE-PER-FEATURE`, `CM-NO-FAIL-OPEN-SKIP`, `CM-AB-PASS-WITH-EVIDENCE-EVERYWHERE`
- **§11.4.107**: Single captured frame is NOT proof — prove LIVE, ADVANCING frames over a window
- **§11.4.107**: Independent frame-advance counter from platform's compositor/decoder telemetry must increase
- **§11.4.107**: Loading/buffering is distinct state — never false-FAIL a still-loading or false-PASS a spinner
- **§11.4.107**: Not-stale-from-previous cross-check (new first frame ≠ previous last frame)
- **§11.4.107**: No-flash-on-wrong-output — high-frequency sampling of non-target output during routing transitions
- **§11.4.107**: Drive through realistic feed/UI path, not deep-link shortcuts
- **§11.4.107**: Metamorphic relations solve oracle problem
- **§11.4.107**: Full-reference quality metrics (SSIM/VMAF/ΔE2000) for owned content
- **§11.4.107**: Mutation-test every analyzer with golden-good + golden-bad fixture pair
- **§11.4.107**: Per-channel audio RMS/loudness (EBU R128) + XRUN/underrun census
- **§11.4.107**: OCR overlay/subtitle detection needs per-word confidence floor + ROI
- **§11.4.107**: Thresholds calibrated on project's own fixtures, not hardcoded
- **§11.4.107**: 4-layer per §11.4.4(b): pre-build gate + runtime/on-device test + paired §1.1 mutation + HelixQA Challenge
- **§11.4.158**: Intensive all-feature/flow/edge-case video-recording + read-the-screen content-verification
- **§11.4.158**: System MUST ACTUALLY READ every shown log line/message/UI label and VERIFY genuine working result
- **§11.4.158**: HelixQA MUST drive the exercise→record→read→score pass
- **§11.4.158**: Vision analysis MANDATORY for every recording

### 4.5 Test Execution Discipline
- **§11.4.3**: Tests depending on environment topology MUST detect topology and dispatch topology-appropriate variant
- **§11.4.3**: SKIP-with-reason when required topology absent; PASS-by-default forbidden
- **§11.4.4**: Moment any defect is re-discovered/re-produced/newly identified during test cycle, cycle MUST stop
- **§11.4.4**: Protocol: systematic debugging → fix at root cause → four-layer coverage → full rebuild → re-deploy on every target → full retest from beginning
- **§11.4.4**: No "let it finish for the data" — data is contaminated
- **§11.4.9**: All source-side fixes that DON'T require runtime validation MUST be landed BEFORE next artifact rebuild
- **§11.4.9**: Exceptions documented with `REQUIRES_REBUILD: <reason>`
- **§11.4.14**: Every test MUST leave target in quiescent state — cleanup mandatory on EVERY exit path
- **§11.4.14**: Use `trap '<cleanup>' EXIT` or `try/finally`
- **§11.4.14**: Tests MUST verify cleanup succeeded via positive evidence
- **§11.4.14**: Orchestrator MUST run post-test sanity check between tests
- **§11.4.50**: Every PASS MUST be executed N times (N=3 normal, N=10 cycle-validation) against same firmware MD5 + same device + same topology
- **§11.4.50**: Divergent N-iter run = auto-FAIL — no "first PASSed = flake" path
- **§11.4.50**: Every public API path MUST have ≥1 dedicated test
- **§11.4.50**: Coverage threshold ratchets 70 → 85 → 95 → 99 over phases
- **§11.4.50**: Pre-build gates: `CM-COVENANT-114-50-PROPAGATION`, `CM-AF-RELIABILITY-CHECK-WIRED`, `CM-AF-FEATURE-COVERAGE-MATRIX`
- **§11.4.40**: Release tag MUST NOT be created until COMPLETE retest with ALL existing tests on clean baseline
- **§11.4.40**: Full retest comprises: pre-build full sweep, post-build full sweep, on-device 4-phase on EVERY device, meta-test full mutation sweep, Challenge bank full sweep, Issues/Fixed state audit, CONTINUATION sync check
- **§11.4.40**: Typically 12-48h — NOT optional, NOT abbreviated
- **§11.4.42**: Priority-ordered cycles — five mandatory steps per cycle
- **§11.4.42**: Select TOP + MIDDLE critical only; smoke gate (<30 min); only if smoke GREEN → full retest
- **§11.4.46**: Every post-flash run MUST first run recent-work validation pass
- **§11.4.46**: Full `test_all_fixes.sh` runs ONLY after 100% green on recent-work
- **§11.4.46**: Pre-build gates: `CM-COVENANT-114-46-PROPAGATION`, `CM-AF-RECENT-WORK-VALIDATION-GATE`, `CM-AF-VALIDATION-ARTIFACT-FILE`
- **§11.4.89**: Long-running tests (>30s) MUST run background via `nohup` + `disown`
- **§11.4.89**: Main stream proceeds immediately — "waiting for results" is ONLY acceptable idle reason
- **§11.4.89**: Foreground only for <30s OR operator authorisation
- **§11.4.89**: Pre-build gates `CM-COVENANT-114-89-PROPAGATION` + `CM-BACKGROUND-TEST-EXECUTION-WIRED`
- **§11.4.132**: Tests MUST run in RISK-DESCENDING order — highest-risk set FIRST
- **§11.4.132**: Risk factors: (a) most-recently-worked, (b) historically most-problematic, (c) highest crash/break/regress likelihood, (d) most-reopened per §11.4.55
- **§11.4.132**: Each highest-risk PASS requires real captured evidence — no metadata-only/config-only
- **§11.4.132**: Gate `CM-COVENANT-114-132-PROPAGATION` + `CM-RISK-ORDERED-VALIDATION-PRIORITY`
- **§11.4.118**: After fixing reported set, run discovery + stress pass across ALL devices beyond reported defects
- **§11.4.118**: Produce PROVABLE coverage — enumerated list of subsystems/journeys/stress-scenarios exercised
- **§11.4.118**: "We found no other issues" requires enumerated coverage list
- **§11.4.118**: Gate `CM-COVENANT-114-118-PROPAGATION` + `CM-DISCOVERY-COVERAGE-ENUMERATED`
- **§11.4.189**: Most-reopened cases get extra-depth live-testing scrutiny FIRST
- **§11.4.189**: For highest-reopens-count cases: re-run at EXTRA DEPTH, RE-INVESTIGATE defect, CONFIRM with rock-solid physical evidence
- **§11.4.189**: Gate `CM-COVENANT-114-189-PROPAGATION` + `CM-MOST-REOPENED-EXTRA-DEPTH-LIVE-TEST`

### 4.6 Specific Test Types
- **§11.4.48**: Every video playback test MUST traverse user-equivalent UI path (launcher → app → content list → tile → playback → pause/resume → stop)
- **§11.4.48**: NOT Intent/Broadcast shortcuts
- **§11.4.48**: Coverage: every video-capable app + every stream type + every codec
- **§11.4.48**: Pre-build gates: `CM-AF-UI-DRIVEN-VIDEO-COVERAGE` + `CM-COVENANT-114-48-PROPAGATION`
- **§11.4.49**: Every feature test MUST ship in TWO variants: UI-driven AND Intent/Broadcast-driven
- **§11.4.49**: Shared assertion base `tests/lib/dual_approach_test_base.sh`
- **§11.4.49**: Gates: `CM-COVENANT-114-49-PROPAGATION`, `CM-AF-DUAL-APPROACH-COVERAGE`, `CM-AF-KINOPOISK-5-1-DUAL-COVERAGE`
- **§11.4.68**: Audio/video routing tests MUST capture + verify positive sink-side or downstream evidence — never config-only, metadata-only, PCM-open-state-only
- **§11.4.68**: Closed enumeration of acceptable evidence: (1) sink-side codec-state, (2) PCM hw_ptr delta > 0, (3) ALSA ELD, (4) ffprobe non-zero frames, (5) recording-analyzer matched event, (6) tinycap RMS above floor
- **§11.4.68**: Gates: `CM-COVENANT-114-68-PROPAGATION` + `CM-POSITIVE-SINK-EVIDENCE-PER-AUDIO-TEST`
- **§11.4.13**: Whenever downstream consumer provides network-accessible introspection API, tests MUST consume that report as captured-evidence
- **§11.4.13**: On-source-side view alone is insufficient
- **§11.4.13**: Sink-side reports MUST be: identity-verified, topology-dispatched, cross-referenced with on-source state at matching wall-clock instant
- **§11.4.38**: Every user-distributable build artifact MUST be opened by tests/challenges to verify each user-visible asset is present and non-degenerate
- **§11.4.38**: One challenge script per artifact type
- **§11.4.136**: Any test asserting media playback MUST drive REAL content through user's path and assert genuine play via §11.4.107 liveness battery plus decoder-health census
- **§11.4.136**: Metadata-only / launch-only / registration-only / single-frame / config-only PASS forbidden
- **§11.4.117**: When accessibility hierarchy is blank/partial/unreliable, test MUST fall back to PIXEL ORACLE (CV/OCR)
- **§11.4.117**: Tool MUST both drive input AND read pixels
- **§11.4.117**: CV/OCR analyzer self-validated golden-good/golden-bad
- **§11.4.143**: Video streaming app tests MUST drive REAL end-user journey (launch → browse catalog → choose specific title → press Play/Resume → confirm chosen content plays)
- **§11.4.143**: NOT sample/demo/loop-clip/deep-link/intent shortcuts
- **§11.4.143**: Gate `CM-COVENANT-114-143-PROPAGATION` + `CM-VIDEO-REAL-JOURNEY-TEST`
- **§11.4.137**: Subtitle-correctness test MUST classify cue's content class — CHROME (FAIL) vs DIALOGUE (PASS)
- **§11.4.137**: CHROME if control/menu label, time/numeric chrome, not prose, outside safe-title band, OR static across window
- **§11.4.137**: DIALOGUE only when prose + not-denied + not-chrome + position-ok + cadence ≥2
- **§11.4.137**: Deny-list MUST be verified present in SHIPPED artifact
- **§11.4.137**: Gate `CM-COVENANT-114-137-PROPAGATION` + `CM-SUBTITLE-CONTENT-CORRECTNESS-ORACLE`
- **§11.4.81**: Per-OS tests REQUIRED — every gate test has `case "$(uname -s)"` branches with positive captured evidence per branch
- **§11.4.81**: SKIP-only when platform genuinely cannot enforce
- **§11.4.81**: Honest kernel-gap citation + adjacent equivalent test REQUIRED
- **§11.4.81**: Gate `CM-CROSS-PLATFORM-PARITY`
- **§11.4.135**: Standing regression-guard suite runs on EVERY build+deploy, BLOCKS release tag on failure
- **§11.4.135**: Every closed defect registers permanent §11.4.115 RED-on-broken regression test in same commit as fix
- **§11.4.135**: RED_MODE=1 captures historical defect on pre-fix artifact; RED_MODE=0 = standing GREEN guard
- **§11.4.135**: Regression suite runs FIRST in post-deploy cycle
- **§11.4.135**: Gate `CM-COVENANT-114-135-PROPAGATION` + `CM-REGRESSION-GUARD-REGISTERED` / `CM-REGRESSION-GUARD-SUITE-WIRED`

### 4.7 Test Fix Workflow
- **§11.4.43**: Every fix follows 5-step TDD-fix workflow: RED → LIVE-ADB-PROBE → GREEN → VERIFY → DOCUMENT
- **§11.4.43**: RED = failing test FIRST, real product defect, captured evidence
- **§11.4.43**: Test-after-fix is a §11.4 PASS-bluff
- **§11.4.115**: Every RED test MUST reproduce defect on CURRENT pre-fix artifact with positive evidence
- **§11.4.115**: Single source carries polarity switch (`RED_MODE`): 1 = reproduce-and-assert-defect-present, 0 = post-fix GREEN regression-guard
- **§11.4.115**: RED-on-broken + GREEN-on-fixed both captured on clean target
- **§11.4.115**: Gate `CM-COVENANT-114-115-PROPAGATION` + `CM-RED-POLARITY-SWITCH-PRESENT`
- **§11.4.51**: Every fix MUST be classified by rebuild-requirement before commit
- **§11.4.51**: If LIVE_ADB_TESTABLE: apply fix to running device first, capture PASS, THEN commit + rebuild + reflash
- **§11.4.51**: If REQUIRES_REBUILD: proceed directly to source-side + rebuild
- **§11.4.51**: Commit footer MUST be `LIVE_ADB_VALIDATED: yes` or `REQUIRES_REBUILD: <reason>`
- **§11.4.51**: Pre-build gates: `CM-COVENANT-114-51-PROPAGATION`, `CM-AF-CLASSIFY-FIX-HELPER-EXISTS`, `CM-AF-LIVE-ADB-FIRST-COMMIT-MARKER`
- **§11.4.146**: Three-step test workflow: (1) REPRODUCE-FIRST + INVESTIGATE (RED test gathers additional forensic data), (2) SAME-TEST-CONFIRMS-FIX (polarity switch), (3) EXTEND-TO-ALL-CASES (fan out across full case-space)
- **§11.4.146**: Gate `CM-COVENANT-114-146-PROPAGATION` + `CM-REPRODUCE-FIRST-THEN-EXTEND`
- **§11.4.114**: On regression: FIRST identify last release tag where feature was KNOWN-GOOD and diff/bisect against it
- **§11.4.114**: Gate `CM-COVENANT-114-114-PROPAGATION` + `CM-REGRESSION-ISOLATED-AGAINST-KNOWN-GOOD`
- **§11.4.120**: When correct fix causes pre-existing gate to FAIL: reconcile — rewrite gate to assert new mechanism, update paired §1.1 mutation
- **§11.4.120**: NEVER fake-pass the gate or revert the correct fix
- **§11.4.120**: Gate `CM-COVENANT-114-120-PROPAGATION` + `CM-GATE-RECONCILED-NOT-FAKE-PASSED`
- **§11.4.138**: Operator-found defect the GREEN suite passed triggers: §11.4.102 systematic-debugging → bluff-audit → permanent §11.4.135 regression guard → bluff-audit committed
- **§11.4.138**: Gate `CM-COVENANT-114-138-PROPAGATION` + `CM-OPERATOR-ESCAPE-BLUFF-AUDIT`
- **§11.4.108**: A fix is DONE only when its declared runtime signature verifies on a CLEAN deployment
- **§11.4.108**: Four distinct layers: (1) SOURCE, (2) ARTIFACT, (3) RUNTIME-ON-CLEAN-TARGET, (4) USER-VISIBLE
- **§11.4.108**: Every fix declares ONE machine-checkable runtime signature
- **§11.4.108**: Gates span all four layers — source, artifact (BYTES landed), runtime-on-clean-target, user-visible
- **§11.4.108**: ≥3 "fixed-but-not-working" discoveries in one cycle = architectural VERIFICATION flaw, not three independent bugs
- **§11.4.108**: Batch validated only after COMPREHENSIVE per-item runtime-signature verification on clean baseline
- **§11.4.108**: Gate `CM-COVENANT-114-108-PROPAGATION` + `CM-RUNTIME-SIGNATURE-REGISTRY`
- **§11.4.130**: After blocker fixed and new artifact deployed: FIRST re-test specific last-failing features BEFORE full validation
- **§11.4.130**: Gate `CM-COVENANT-114-130-PROPAGATION` + `CM-FIX-FIRST-AFTER-REDEPLOY`
- **§11.4.129**: On huge blocker during release validation: STOP all testing → fix ALL discovered issues → process all recorded data → land rock-solid fixes → author NEW tests → rebuild → reflash → RESTART full validation
- **§11.4.129**: Gate `CM-COVENANT-114-129-PROPAGATION` + `CM-HUGE-BLOCKER-FULL-RESTART`
- **§11.4.139**: Before post-deploy validation: assert running-artifact == built-artifact (clean target or no stale overlay)
- **§11.4.139**: Gate `CM-COVENANT-114-139-PROPAGATION` + `CM-CLEAN-ARTIFACT-RUNTIME-SIGNATURE`

### 4.8 Recordings
- **§11.4.154**: Every feature/QA video MUST capture ONLY the window/surface under test — NEVER whole desktop
- **§11.4.154**: When new recording run begins, agent's OWN prior in-scope stale recordings MUST be removed FIRST
- **§11.4.154**: `docs/qa/<run-id>/` curated evidence is durable record, NOT rotated
- **§11.4.154(C)**: MP4 auto-conversion REQUIRED — any `.cast` file auto-converted to `.mp4` via `agg` + `ffmpeg`
- **§11.4.155**: Every recording filename MUST start with project-name prefix (from `HELIX_RELEASE_PREFIX` or lowercased dir name)
- **§11.4.155**: Canonical form: `<PREFIX>---<feature-or-scope>---<run-id>.<ext>`
- **§11.4.155**: Same prefix for EVERY recording in a checkout
- **§11.4.155**: Gate `CM-COVENANT-114-155-PROPAGATION` + `CM-RECORDING-PROJECT-NAME-PREFIX`
- **§11.4.159**: Mandatory window-specific MP4 video recording + vision validation
- **§11.4.159**: (A) Window-specific ONLY, (B) MP4 REQUIRED (H.264), (C) Project-name prefix, (D) Mandatory vision validation after EVERY recording, (E) Terminal window cleanup, (F) Real results ONLY, (G) Re-runnable evidence, (H) Fresh-corpus rotation, (I) Content verification MANDATORY (not duration-based), (J) Expected-content specification BEFORE recording, (K) Content-verification recording workflow, (L) Root cause analysis REQUIRED for rejected recordings, (M) Real-time monitoring RECOMMENDED
- **§11.4.159**: Gate `CM-COVENANT-114-159-PROPAGATION` + `CM-WINDOW-VIDEO-VALIDATED`
- **§11.4.128**: For EVERY test/debug device, across EVERY transport — ALWAYS live-record all analysable data
- **§11.4.128**: Non-invasive read-only probes with bounded sampling
- **§11.4.128**: Background + parallel + subagent-driven — never blocks main stream
- **§11.4.128**: Token-conscious record-now-analyse-later — raw NOT processed without need
- **§11.4.128**: Raw git-ignored (with §11.4.77 regen mechanism) + code-intelligence-excluded
- **§11.4.144**: Every tracked device MUST be availability-FOLLOWED — detect drop, log honest offline event, wait for return (already-defined timings), re-attach + log online event, escalate to sanctioned recovery if not returned within defined timeout
- **§11.4.144**: Gate `CM-COVENANT-114-144-PROPAGATION` + `CM-DEVICE-AVAILABILITY-FOLLOWED`

---

## 5. CODE QUALITY REQUIREMENTS

### 5.1 Code Review
- **§11.4.142**: EVERY change to ANY repository MUST pass INDEPENDENT code-review BEFORE acceptance/commit/build
- **§11.4.142**: NO change class exempt — source, fixes, tests, gates, mutations, docs, doc-tooling, build/CI, config, governance, sub-agent output, refactors, single-line edits
- **§11.4.142**: Independence is load-bearing — reviewer structurally separated from author
- **§11.4.142**: Review iterates to clean GO per §11.4.134
- **§11.4.142**: Gate `CM-COVENANT-114-142-PROPAGATION` + `CM-EVERY-CHANGE-REVIEWED`
- **§11.4.125**: Code-review agent gate BEFORE pre-build sweep + main build
- **§11.4.125**: Review analyzes: all work done, all existing data+facts, existing codebase (blast radius), current git history
- **§11.4.125**: Determines quality + safety + will-it-REALLY-work
- **§11.4.125**: Validates+verifies tests genuinely exercise work-under-test with ZERO chance of false result
- **§11.4.125**: Any finding MUST be fixed BEFORE pre-build sweep + build
- **§11.4.125**: Review iterates until no blocking findings
- **§11.4.125**: Gate `CM-COVENANT-114-125-PROPAGATION` + `CM-CODE-REVIEW-GATE-BEFORE-BUILD`
- **§11.4.134**: After first review remediation: re-run review — and KEEP re-running until clean GO with ZERO new issues AND ZERO warnings
- **§11.4.134**: Every round's verdict + fix validation MUST carry rock-solid physical captured evidence
- **§11.4.134**: Gate `CM-COVENANT-114-134-PROPAGATION` + `CM-CODE-REVIEW-ITERATE-UNTIL-GO`
- **§11.4.145**: For EVERY fix/change/new feature — INDEPENDENT impact-research agents across EIGHT ANGLES: correctness/logic, regression, latent/dangerous-code, security, performance, safety, cross-feature interaction, business-logic conformance
- **§11.4.145**: Gate `CM-COVENANT-114-145-PROPAGATION` + `CM-IMPACT-RESEARCH-PER-CHANGE`
- **§11.4.92**: Every non-trivial change MUST pass 5-pass evaluation: main-task verification, regression-blast-radius, cross-feature interaction, deep-research validation, anti-bluff confirmation
- **§11.4.92**: Gate `CM-COVENANT-114-92-PROPAGATION` + `CM-MULTI-PASS-EVALUATION-EVIDENCE`

### 5.2 Code Architecture
- **§11.4.67**: Every shell script that may be invoked under a target shell other than its shebang MUST parse cleanly under that target shell
- **§11.4.67**: In-scope `*.sh` parses under `sh -n`
- **§11.4.67**: Bash-only constructs MUST be wrapped in `eval` OR guarded by bash-only loading
- **§11.4.67**: Shebangs honest — `#!/bin/bash` only if bash actually expected
- **§11.4.67**: Pre-build gate `CM-SCRIPT-TARGET-SHELL-PARSEABLE`
- **§11.4.29**: Every directory / submodule / file MUST use lowercase snake_case
- **§11.4.29**: Non-compliant names MUST be renamed; every reference updated atomically
- **§11.4.29**: Exceptions: language-mandated case, vendor third-party submodules, build artifacts
- **§11.4.29**: `Upstreams/` → `upstreams/` transition — support both, lowercase wins
- **§11.4.29**: Lacking both-dir support is release blocker
- **§11.4.11**: Files organised by purpose, not historical accident
- **§11.4.11**: Logs and forensic artifacts under operator-controlled directories — NEVER at repo root, NEVER tracked unless reference assets
- **§11.4.11**: 5 canonical tracker documents at `docs/` root by design (architectural constants)
- **§11.4.124**: Before removing seemingly-dead code: investigate via git history, capture as FACT where/how wired and when/how became dead, prove genuinely unneeded
- **§11.4.124**: Removal permitted ONLY with captured proof — its own separate commit
- **§11.4.124**: With NO proof: investigate where/how to wire properly + add tests
- **§11.4.124**: Gate `CM-COVENANT-114-124-PROPAGATION` + `CM-DEAD-CODE-INVESTIGATE-BEFORE-REMOVE`
- **§11.4.122**: No application/component/service/feature may be removed from existing codebase WITHOUT FIRST interactively asking operator
- **§11.4.122**: Silent removal = release blocker
- **§11.4.122**: Tracked DROP path: ask → operator approves → mark `Obsolete` with reason `feature-removed` + operator-approval citation → then remove
- **§11.4.112**: When deep research PROVES a goal structurally impossible on target platform: classify `Won't-fix` with reason `structurally-impossible`, document impossibility evidence, NOT re-attempted in future cycles
- **§11.4.112**: Gate `CM-COVENANT-114-112-PROPAGATION` + `CM-WONT-FIX-STRUCTURAL-IMPOSSIBILITY`
- **§11.4.111**: Any binding to hardware/device/resource MUST resolve by stable identifier (name/UUID/serial), NOT enumeration index/ordinal
- **§11.4.111**: Gate `CM-COVENANT-114-111-PROPAGATION` + `CM-RESOLVE-BY-NAME-NOT-INDEX`
- **§11.4.102(A)**: On ANY issue/bug/failure/gate failure — activate `superpowers:systematic-debugging` BEFORE proposing/writing/applying any fix (NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST)
- **§11.4.102(B)**: `superpowers:using-superpowers` MUST be loaded + applied at session start
- **§11.4.102(C)**: Every skill plugin/marketplace package MUST be installed + loadable BEFORE dependent work proceeds

### 5.3 Naming Conventions
- **§11.4.151**: Every release tag AND version name MUST be prefixed with project's release prefix (`<PREFIX>-<version>`)
- **§11.4.151**: Prefix resolution: (1) `HELIX_RELEASE_PREFIX` from `.env`, (2) fallback = lowercased snake_case project root dir name
- **§11.4.151**: Prefix MUST be IDENTICAL across main repo + all owned submodules in one release
- **§11.4.151**: Version codes increment monotonically within prefix — never reset, never skipped
- **§11.4.151**: Gate `CM-COVENANT-114-151-PROPAGATION` + `CM-RELEASE-PREFIX-NAMING`
- **§11.4.54**: Every workable item heading carries `[ATM-NNN]` (zero-padded ≥3 digits)
- **§11.4.54**: Allocated by `scripts/testing/assign_atm_ticket_ids.sh`
- **§11.4.54**: Identifiers monotonic, never renumbered, never reused, no gaps
- **§11.4.54**: Issues_Summary.md and Fixed_Summary.md carry `ATM ID` column as leftmost data column
- **§11.4.54**: Pre-build gates: `CM-ATM-TICKET-IDS-COMPLETE`, `CM-ATM-TICKET-IDS-UNIQUE`, `CM-ATM-TICKET-IDS-MONOTONIC`

### 5.4 Token Efficiency
- **§11.4.141**: MUST cut token spend toward 30-40% of current (60-70% reduction) without degrading quality
- **§11.4.141**: Measure set: (1) prompt-cache static governance prefix, (2) subagent model-tiering + output-to-file, (3) thin always-loaded INDEX + on-demand detail, (4) CodeGraph/retrieval-first, (5) output-token reduction, (6) tool-call batching + no re-reads, (7) compaction/context-editing
- **§11.4.141**: Mandatory measured proof — token-accounting harness BEFORE vs AFTER
- **§11.4.141**: AFTER reduction MUST show ZERO regression on pre-build sweep + meta-test mutation sweep + propagation gates + strong-model reasoning probe + cache-warm proof
- **§11.4.141**: Gate `CM-COVENANT-114-141-PROPAGATION` + `CM-TOKEN-EFFICIENCY`

---

## 6. SUBMODULE / SUBTREE REQUIREMENTS

### 6.1 Submodule Management
- **§3**: Submodule changes: (1) commit inside submodule first, (2) push submodule commit to ALL remotes, (3) then run parent commit wrapper
- **§3**: Skipping step 1 produces parent commits/tags pointing at old submodule HEADs
- **§4**: Every tag on main repo MUST be mirrored on every owned submodule at that submodule's currently-pointed-to HEAD
- **§4**: Same tag pushed to every remote of every owned repo
- **§11.4.26**: Before modifying governance files: fetch + pull from submodule first
- **§11.4.26**: Apply change with §11.4.17 classification + verbatim mandate quote
- **§11.4.26**: Validate before commit (`meta_test_inheritance.sh` + no merge-conflict markers + cross-file consistency)
- **§11.4.26**: Commit + push to ALL upstreams (stage only governance files)
- **§11.4.26**: Careful conflict resolution — force-push forbidden
- **§11.4.26**: Post-merge validation — `git submodule update --remote --init` + re-run cascade verifier
- **§11.4.26**: Bump consuming project's `.gitmodules` pointer to new HEAD in same commit
- **§11.4.31**: Every owned-by-us submodule MUST ship `helix-deps.yaml` listing own-org dependencies
- **§11.4.31**: Tooling `incorporate-submodule` adds at canonical path, reads helix-deps.yaml, recurses, aborts on conflicting refs, emits `.helix-manifest.yaml`
- **§11.4.31**: Each manifest paired with anti-bluff Challenge
- **§11.4.32**: After constitution submodule fetch+pull with content change: run `scripts/verify-all-constitution-rules.sh` BEFORE new HEAD treated as canonical
- **§11.4.32**: Pull-time invocation auto-triggered by `git submodule update --remote constitution`
- **§11.4.32**: Anti-bluff: sweep's own meta-test plants violation per gate, asserts sweep FAILs
- **§11.4.32**: This is the enforcement engine for every other rule

### 6.2 Submodule as Equal Codebase
- **§11.4.28(A)**: Every owned-by-us submodule is an EQUAL part of the consuming project's codebase
- **§11.4.28(A)**: Same engineering attention: analysis, extension, tests, gap-fill, bug-fix, documentation
- **§11.4.28(A)**: Round that improves main while leaving owned submodule deficiency unaddressed is a violation
- **§11.4.28(B)**: NEVER inject project-specific context INTO submodules
- **§11.4.28(B)**: Submodules stay project-not-aware, reusable, modular, testable
- **§11.4.28(C)**: Every dependency consumed by owned submodule lives at parent root (`<root>/<name>/` or `<root>/submodules/<name>/`)
- **§11.4.28(C)**: Nested own-org submodule chains FORBIDDEN — add dependency at parent root
- **§11.4.28(C) EXCEPTION**: Constitution submodule ITSELF MAY host depth-1 reusable-engine submodules
- **§11.4.28(C)**: Gates: `CM-OWNED-SUBMODULE-EQUAL-ENGINEERING`, `CM-OWNED-SUBMODULE-DECOUPLING`, `CM-OWNED-SUBMODULE-LAYOUT`
- **§11.4.74**: Before scaffolding new module: survey `vasic-digital` + `HelixDevelopment` orgs on GitHub + GitLab
- **§11.4.74**: Reuse existing Submodule when ≥80% functionality match; extend in-place via upstream PR when 80%+ but missing features
- **§11.4.74**: Document survey result with `Catalogue-Check: reuse|extend|no-match <org/repo>@<sha>`
- **§11.4.36**: Every clone/add of Git repository MUST be followed by `install_upstreams` invocation IF tree contains `upstreams/` with recipe files
- **§11.4.36**: Gate `CM-INSTALL-UPSTREAMS-ON-CLONE`

### 6.3 Submodule Dependency Manifest
- **§11.4.27(B)**: Required dependency submodules: Challenges + HelixQA + any other functionality submodules under vasic-digital/HelixDevelopment orgs
- **§11.4.27(B)**: Pointers bumped to upstream HEAD in same commit as cascade work

### 6.4 Submodule Propagation
- **§11.4.164**: Every fetch+pull of constitution submodule MUST trigger `constitution/scripts/post_update_hook.sh`
- **§11.4.164**: Detects changed files, registers new/modified skills, registers MCP servers, installs hooks, validates scripts syntax
- **§11.4.164**: Gate `CM-COVENANT-114-164-PROPAGATION` + `CM-CONSTITUTION-AUTO-PROPAGATION`

---

## 7. INFRASTRUCTURE REQUIREMENTS

### 7.1 Git Operations
- **§2**: All commit/push uses project's official multi-remote commit wrapper (e.g. `scripts/commit_all.sh`)
- **§2**: Direct `git commit` / `git push` / `git add` on main repo prohibited in normal workflow
- **§2**: Commit/push wrappers MUST hold advisory `flock` with self-cleaning lock files
- **§2.1**: Every project hosted on multiple Git providers MUST push every commit to ALL configured upstream remotes
- **§2.1**: Constitution submodule ships `install_upstreams.sh` helper + `Upstreams/` directory
- **§9.1**: Destructive operations MUST execute full protocol in strict order:
  1. Full hardlinked backup of `.git`
  2. Record critical metadata into backup dir
  3. Identify target state
  4. Run operation (NEVER `--no-verify`, NEVER auto-force)
  5. Post-operation verification gate (ALL must pass)
  6. Only if all checks pass may operation be successful
  7. Then force-push may proceed — and still per §9.2
- **§9.3**: Hardlinked backup is standard — zero excuse
- **§9.4**: After any `git filter-repo`: commit record under `docs/changelogs/` or `docs/history-rewrites/`
- **§11.4.37**: FIRST git-touching action of any session: `git fetch --all --prune`, check HEAD..@{u}, integrate before local edit
- **§11.4.37**: Scope: consuming project root + every owned submodule recursively + constitution submodule + any dependency
- **§11.4.37**: Gate `CM-FETCH-BEFORE-EDIT-AUDIT`
- **§11.4.71**: Pre-push cycle: (1) fetch, (2) pull each divergent remote, (3) investigate foreign commits, (4) integrate mandatory changes with four-layer coverage, (5) push to every configured remote
- **§11.4.71**: Gate `CM-COVENANT-114-71-PROPAGATION`
- **§11.4.88**: commit lock released IMMEDIATELY after `git commit` returns 0; push runs detached via `nohup` + `disown`
- **§11.4.88**: `push_all.sh` acquires per-remote flock — concurrent same-remote serializes, different-remote parallelises
- **§11.4.88**: Pre-build gates: `CM-COVENANT-114-88-PROPAGATION` + `CM-BACKGROUND-PUSH-WIRED`
- **§11.4.84**: Pre-`git add`: grep for mutation markers; cross-check `git status --porcelain` against declared scope — unaccounted entries → ABORT
- **§11.4.84**: Active mutation gates MUST be serialised — mutate → assert FAIL → restore → assert PASS — and tree clean BEFORE unrelated commits
- **§11.4.84**: Concurrent subagents same-checkout MUST use lockfile (`.git/MUTATION_IN_PROGRESS`) or `git worktree add`
- **§11.4.180**: Commit/push wrappers MUST auto-reap provably-stale locks before acquiring (holder PID dead, or no PID + older than threshold + no live process)
- **§11.4.180**: NEVER remove a lock whose holder is ALIVE
- **§11.4.180**: Gate `CM-COVENANT-114-180-PROPAGATION` + `CM-WRAPPER-STALE-LOCK-AUTO-REAP`

### 7.2 Mechanical Enforcement
- **§11.4.75**: FIVE independent mechanical enforcement layers:
  1. Local `pre-commit` git hook
  2. `commit_all.sh` integration with self-repair (`_constitution_sibling_check`)
  3. Local `pre-push` git hook
  4. `post-commit` auto-repair hook
  5. Local-only equivalent (remote CI DISABLED)
- **§11.4.75**: Mandatory helper contracts: `scripts/install_git_hooks.sh`, `scripts/git_hooks/{pre-commit,pre-push,post-commit,commit-msg}`, `_constitution_sibling_check`
- **§11.4.75**: Five pre-build gates: `CM-COVENANT-114-75-PROPAGATION`, `CM-GIT-HOOKS-INSTALL-SCRIPT`, `CM-GIT-HOOKS-SOURCE-DIR`, `CM-COMMIT-ALL-SIBLING-CHECK`, `CM-CI-WORKFLOW-PRESENT`
- **§11.4.75**: No escape hatch — `--no-verify` IS deliberate audit-trail bypass
- **§11.4.109**: PreToolUse guard hook `constitution/scripts/hooks/guard-forbidden-commands.sh` blocks forbidden commands at tool-call boundary
- **§11.4.109**: `constitution/docs/AGENT_GUARDRAILS.md` = canonical preamble for every subagent dispatch + Orchestrator Pre-Action Checklist
- **§11.4.109**: Hook inherited by reference — NEVER copied locally
- **§11.4.109**: Gates: `CM-ANTI-FORGETTING-ENFORCEMENT` + `CM-COVENANT-114-109-PROPAGATION`
- **§11.4.22**: Lightweight doc-sync commit path (separate from full-repo wrapper) — doc-set only, separate flock
- **§11.4.22**: Gate `CM-COMMIT-DOCS-EXISTS`

### 7.3 Work Tracking & Database
- **§11.4.15**: Every active item MUST carry `**Status:**` line within five lines of heading
- **§11.4.15**: Status closed-set: Queued, In progress, Ready for testing, In testing, Reopened, Fixed (→ Fixed.md)
- **§11.4.15**: Issues/Issues_Summary/Fixed all in sync (Markdown + HTML + PDF)
- **§11.4.16**: Every active item MUST carry `**Type:**` within eight non-blank lines
- **§11.4.16**: Type closed-set: Bug, Feature, Task
- **§11.4.16**: Pre-build gates: `CM-ITEM-TYPE-TRACKING` + `CM-COVENANT-114-16-PROPAGATION`
- **§11.4.19**: Fixed archive MUST mirror Issues along Status and Type axes
- **§11.4.19**: Pre-build gate `CM-FIXED-COLUMN-ALIGNMENT`
- **§11.4.21**: Operator-blocked is 7th Status — last-resort after exhausting self-resolution paths
- **§11.4.21**: Self-resolution paths: CLI/ADB/SSH/API, subagent delegation, existing tooling, captured fallback, documentation+research
- **§11.4.21**: Every Operator-blocked item carries `**Operator-Block-Details:**` (WHAT, WHY, UNBLOCK CONDITION, WHO)
- **§11.4.21**: Items re-evaluated every Nth tag cycle (≥3rd recommended)
- **§11.4.21**: Gates: `CM-ITEM-OPERATOR-BLOCKED-DETAILS`, `CM-OPERATOR-BLOCKED-SELF-RESOLUTION-AUDIT`
- **§11.4.23**: Visual-cue coloring + status grouping applied to HTML + PDF exports of tracked-item docs
- **§11.4.23**: Print-fidelity requirement — `print-color-adjust: exact`
- **§11.4.23**: Gate `CM-DOC-COLOR-GROUPING-DISCIPLINE`
- **§11.4.33**: Type-aware closure vocabulary: Bug→Fixed, Feature→Implemented, Task→Completed (each with `(→ Fixed.md)` suffix)
- **§11.4.33**: Gate `CM-CLOSURE-VOCAB-TYPE-AWARE`
- **§11.4.34**: Every Reopened item carries `**Reopened-Details:**` (By, On, Reason, Evidence)
- **§11.4.34**: Reason closed vocabulary: test-failed, manual-testing-detected, captured-evidence-contradicts, end-user-report, cycle-re-discovered, design-reconsidered
- **§11.4.34**: Issues_Summary Status column distinguishes Reopened sub-states by source
- **§11.4.34**: Gate `CM-ITEM-REOPENED-DETAILS`
- **§11.4.55**: Every item with reopens_count > 0 MUST have `docs/issues/<ATM-NNN>/Reopens.md` (+ HTML + PDF)
- **§11.4.55**: Issues_Summary and Fixed_Summary carry Reopens column; count > 0 hyperlinks to per-item Reopens.md
- **§11.4.55**: Pre-build gates: `CM-REOPENS-DOC-EXISTS-WHEN-COUNT-GT-ZERO`, `CM-REOPENS-DOC-REVISION-HEADER`, `CM-REOPENS-COL-IN-SUMMARIES`
- **§11.4.56**: Every Status.md has Status_Summary.md companion with two-audience format (Page 1 for the team, Page 2 for engineers)
- **§11.4.56**: Gates: `CM-STATUS-SUMMARY-EXISTS-FOR-EVERY-STATUS`, `CM-STATUS-SUMMARY-TWO-AUDIENCE`, `CM-STATUS-SUMMARY-REVISION-HEADER`
- **§11.4.53**: Fixed_Summary.md MUST be regenerated whenever Fixed.md changes — HTML + PDF exports travel with markdown
- **§11.4.53**: Pre-build gates: `CM-FIXED-SUMMARY-SYNC` + `CM-COVENANT-114-53-PROPAGATION`
- **§11.4.91**: Every summary entry MUST contain self-contained meaningful description ≥6 words OR ≥40 chars
- **§11.4.91**: Gate `CM-SUMMARY-CLARITY-DESCRIPTIONS`
- **§11.4.90**: Obsolete status added to closed-set — reasons: superseded-by-design-change, superseded-by-later-mandate, feature-removed, duplicate-of, unsupported-topology, not-reproducible
- **§11.4.90**: Every Obsolete heading carries `**Obsolete-Details:**` — triple-check non-negotiable
- **§11.4.90**: Gates: `CM-COVENANT-114-90-PROPAGATION`, `CM-ITEM-OBSOLETE-DETAILS`, `CM-OBSOLETE-COLORIZER-WIRED`
- **§11.4.93**: SQLite-backed SSoT for workable items at `docs/.workable_items.db`
- **§11.4.95**: DB at `docs/workable_items.db` is TRACKED, NEVER gitignored — WAL-checkpoint before commit
- **§11.4.148**: Workable-item integrity: no item without valid status+type+stable id on ALL three surfaces (DB, docs, external tracker)
- **§11.4.148**: Comprehensive structured description per item (WHAT, HOW, reproduce, acceptance criteria)
- **§11.4.148**: BLOCKED items carry WHY + enumerated unblock CHOICES
- **§11.4.148**: Regular never-missed bidirectional DB↔docs↔tracker sync
- **§11.4.148**: Generic idempotent external-tracker push
- **§11.4.148**: Gates: `CM-COVENANT-114-148-PROPAGATION`, `CM-ITEM-INTEGRITY-STATUS-TYPE-ID`, `CM-ITEM-COMPREHENSIVE-DESCRIPTION`, `CM-BLOCKED-UNBLOCK-CHOICES`, `CM-TRACKER-SYNC-IDEMPOTENT`
- **§11.4.149**: Per-workable-item testing diary — append-only `test_diary` table
- **§11.4.149**: Diary captures date_time, tested_by, result, observations, action_taken, evidence_path
- **§11.4.149**: Schema constraint: PASS row without non-empty evidence path is impossible
- **§11.4.149**: Four-format exports + external-tracker sub-task model
- **§11.4.149**: Gates: `CM-COVENANT-114-149-PROPAGATION`, `CM-TEST-DIARY-SYNC`, `CM-DIARY-PASS-REQUIRES-EVIDENCE`
- **§11.4.171**: Every workable item MUST carry comprehensive human-readable description (5-7 sentences, non-technical, covering WHAT/WHY/HOW/WHO/expected outcome)
- **§11.4.171**: Gate `CM-COVENANT-114-171-PROPAGATION`

### 7.4 Parallel Development Infrastructure
- **§11.4.58**: Parallel Work Unit (PWU) pipeline — 5-stage pipeline:
  1. DEVELOP (parallel PWU agents in isolated worktrees)
  2. MERGE (serial via `commit_all.sh` flock + merge-first)
  3. REBUILD+FLASH (parallel where hardware allows)
  4. VALIDATE (parallel D3+D4+meta-test+coverage)
  5. SWEEP (parallel HelixQA + Fixed migration + README refresh)
- **§11.4.58**: Stage 1 of round N+1 overlaps with Stages 4-5 of round N
- **§11.4.58**: 4-layer lock hierarchy (parent flock / per-submodule git / contention-path advisory locks / per-PWU worktree)
- **§11.4.58**: Anti-bluff merge-time enforcement (all four): RED-test captured, paired meta-test mutation, deterministic consistency, captured-evidence
- **§11.4.58**: Pre-build gates: `CM-PWU-LOCK-HIERARCHY`, `CM-PWU-ANTI-BLUFF-COVERAGE`, `CM-PWU-MERGE-QUEUE-DISCIPLINE`, `CM-PWU-PARALLEL-AGENT-LIMIT`
- **§11.4.82(G)**: Subagent scope ≤30 min + worktree isolation + single-responsibility
- **§11.4.82(F)**: Parallel multi-device testing — every owned device runs autonomous cycle concurrently
- **§11.4.103**: Main stream FREE — ALL commits AND pushes run detached
- **§11.4.103**: ≥3 parallel background streams at all times + auto-backfill
- **§11.4.103**: Standing band 3–6, bounded by §12.6 60% memory + §11.4.58 6-agent cap
- **§11.4.103**: Most-critical + most-visible first; audio always top per §11.4.72
- **§11.4.119**: Exactly ONE stream owns each shared/exclusive hardware resource at a time
- **§11.4.119**: Ownership enforced by advisory lock/token — event-driven claim/release
- **§11.4.119**: Gate `CM-COVENANT-114-119-PROPAGATION` + `CM-SINGLE-RESOURCE-OWNER-PARTITION`
- **§11.4.176**: Exactly-once work-item/logical-group claim registry
- **§11.4.176**: Capability-aware deadlock-proof device-lock (all-or-nothing + non-blocking + TTL-reap)
- **§11.4.176**: Gates: `CM-WORK-DIVISION-EXCLUSIVE-CLAIM`, `CM-DEVICE-LOCK-DEADLOCK-FREE`, `CM-COVENANT-114-176-PROPAGATION`
- **§11.4.178**: Parallel work streams on shared resources MUST use track-qualified identity (`<project>__<track>__<role>`)
- **§11.4.178**: Gate `CM-COVENANT-114-178-PROPAGATION` + `CM-TRACK-QUALIFIED-IDENTITY`
- **§11.4.179**: When corruption-isolation required: each stream own `.git` (own object store, own index, own lock namespace) — NOT shared `git worktree`
- **§11.4.179**: Gate `CM-COVENANT-114-179-PROPAGATION` + `CM-GIT-STREAMS-OWN-COMMON-DIR`
- **§11.4.181**: One feature/logical group maps to EXACTLY ONE canonical branch name across main repo + all owned submodules
- **§11.4.181**: Canonical name recorded ONCE in §11.4.176 claim registry / §11.4.93 workable-items DB
- **§11.4.181**: Gate `CM-COVENANT-114-181-PROPAGATION` + `CM-BRANCH-NAME-CONSISTENCY`
- **§11.4.182**: EVERY agent/subagent label + operator-facing work-stream reference MUST start with `(T<N>/<branch> - <alias>)` prefix
- **§11.4.182**: PreToolUse guard hook BLOCKS dispatch without label
- **§11.4.182**: Gate `CM-COVENANT-114-182-PROPAGATION` + `CM-TRACK-BRANCH-LABEL`
- **§11.4.183**: Every track/work-stream MUST maximize multi-agent working approaches (subagent-driven, independent review agents, parallel background streams)
- **§11.4.183**: Every track MUST apply ENTIRE constitution — nothing skipped
- **§11.4.183**: Gate `CM-COVENANT-114-183-PROPAGATION` + `CM-MAX-AGENTS-PER-TRACK`
- **§11.4.167**: Every BIG work item MUST be developed as its own isolated feature work-stream (own sibling project copy, own `feature/<slug>` branch, own per-feature builds + tags)
- **§11.4.167**: CoW/reflink clone — not deep copy
- **§11.4.167**: Own branch + own tags greppable per §11.4.151; NEVER merged to trunk until §11.4.40 full retest GREEN
- **§11.4.167**: Trunk merged INTO every live stream FREQUENTLY (per trunk tag / daily)
- **§11.4.167**: Single-builder build queue (FIFO + advisory lock) + finite-device test queue
- **§11.4.167**: Stream registry tracks branch/base-tag/out-state/last-trunk-merge/merge-approval
- **§11.4.167**: Gates: `CM-COVENANT-114-167-PROPAGATION`, `CM-FEATURE-WORKSTREAM-COW-CLONE`, `CM-FEATURE-WORKSTREAM-NO-MERGE-UNTIL-APPROVED`, `CM-FEATURE-WORKSTREAM-TRUNK-SYNC-CADENCE`, `CM-FEATURE-WORKSTREAM-SUBMODULE-CASCADE`
- **§11.4.187**: Every project MUST ship automatic multi-track ruler orchestration as universal, automatic, out-of-the-box, inherited capability
- **§11.4.187**: Headless spawn via `claude -p --output-format stream-json`
- **§11.4.187**: Per-subscription auth + rate-limit fallback (rebind, provider fallback, bounded-park)
- **§11.4.187**: Crash-resilient ruler self-supervisor (durable write-temp-then-rename state + watchdog)
- **§11.4.187**: Engine at `constitution/scripts/multitrack/` inherited BY REFERENCE
- **§11.4.187**: Gates: `CM-COVENANT-114-187-PROPAGATION` + multiple mechanism gates + paired §1.1 mutations
- **§11.4.147**: Every dispatched agent/subagent tracked through full lifecycle with durable registry
- **§11.4.147**: Registry status closed set: {dispatched, in-flight, crashed, respawned, complete}
- **§11.4.147**: Mechanical crash-detection + respawn-until-complete
- **§11.4.147**: Partial-state preserve → §11.4.84-check → resume-or-clean-restart
- **§11.4.147**: "crash ≠ done" — unit is DONE only when agent reaches `complete` with captured evidence landed
- **§11.4.147**: Gate `CM-COVENANT-114-147-PROPAGATION` + `CM-CRASHED-AGENT-RESPAWN-TRACKED`
- **§11.4.188**: Every long-lived feature branch AND every parallel-development track MUST regularly `git merge origin/main` INTO its own branch THROUGHOUT the work
- **§11.4.188**: Cadence: after every trunk tag / ≥ daily / before any significant new work chunk
- **§11.4.188**: MERGE (never rebase a shared/tagged branch), fetch-first, pre-op backup before large merges
- **§11.4.188**: Post-merge smoke GREEN + zero conflict-marker scan + no-lost-commit
- **§11.4.188**: Gate `CM-COVENANT-114-188-PROPAGATION` + `CM-REGULAR-MAIN-MERGE-INTO-FEATURE`
- **§11.4.191**: Every logical group binds to EXACTLY ONE canonical (track, branch) — NO change may land on wrong track/branch
- **§11.4.191**: Preventive PreToolUse hook `guard-work-track-binding.sh`
- **§11.4.191**: Detective gate `CM-WORK-TRACK-BINDING-ENFORCED`
- **§11.4.191**: Gate `CM-COVENANT-114-191-PROPAGATION`
- **§11.4.192**: FREE track MUST be IMMEDIATELY assigned next-highest-priority domain work-set — NEVER idle
- **§11.4.192**: Source-side advances even when on-device validation is gated
- **§11.4.192**: Gate `CM-COVENANT-114-192-PROPAGATION` + `CM-CONTINUOUS-MULTITRACK-BACKFILL`
- **§11.4.177**: No project-specific script/hook/alias may be wired into global/shared developer-tooling PATH
- **§11.4.177**: Shared tooling MUST operate on invocation directory — never hardcoded project path
- **§11.4.177**: Gate `CM-COVENANT-114-177-PROPAGATION` + `CM-TOOLING-PROJECT-DECOUPLED`

### 7.5 Code Intelligence
- **§11.4.78**: Every consuming project worked on by AI coding agents MUST install, initialize, and use CodeGraph
- **§11.4.78**: Install globally via npm — no sudo
- **§11.4.78**: `.codegraph/config.json` tracked; `.codegraph/codegraph.db` gitignored
- **§11.4.78**: Wire `codegraph serve --mcp` into every CLI agent
- **§11.4.78**: Universal mandatory-exclude baseline set: build outputs, caches/deps, credentials/secrets, QA/recording corpora
- **§11.4.78**: ALL source code MUST be indexable (including AOSP platform trees)
- **§11.4.78**: Cover integration with anti-bluff verification suite
- **§11.4.78**: Document everything in `docs/CODEGRAPH.md`
- **§11.4.79**: Own-org submodules MUST be INCLUDED in CodeGraph index; third-party MUST be EXCLUDED
- **§11.4.80**: Scripts: `scripts/codegraph_update.sh` + `scripts/codegraph_sync.sh`
- **§11.4.80**: `docs/codegraph/Status.md` + `Status_Summary.md` ledgers
- **§11.4.80**: Weekly update cadence floor inherited by reference
- **§11.4.184**: SonarQube scanner CLI (`sonar-scanner`) MUST be installed + durably PATH-discoverable
- **§11.4.184**: Shared `constitution/scripts/sonarqube/` tooling consumed BY REFERENCE (never copied)

### 7.6 Work Tracking
- **§11.4.47**: Before every bigger working round: execute `scripts/firebase/review_round.sh`
- **§11.4.47**: Queries Crashlytics (fatals + non-fatals + ANRs) + Analytics + Performance
- **§11.4.47**: Five mandatory elements: 5-trigger cadence, 3-source query, Issues.md output with Firebase metadata, 3-tier dedup, root-cause analysis per Issue
- **§11.4.47**: Pre-build gates: `CM-COVENANT-114-47-PROPAGATION`, `CM-AF-FIREBASE-REVIEW-CADENCE`, `CM-AF-FIREBASE-ISSUE-XREF`
- **§11.4.47**: No `--skip-firebase-review` flag
- **§11.4.152**: Continuous monitoring of ALL four Crashlytics surfaces — fatal crashes, ANRs, performance traces, AND non-fatals
- **§11.4.152**: Systematic-debugging of each (reproduce-before-fix); fix/improvement for every confirmed issue
- **§11.4.152**: Permanent §11.4.135 regression guard per closed issue — same commit
- **§11.4.152**: Gate `CM-COVENANT-114-152-PROPAGATION` + `CM-CRASHLYTICS-ISSUE-FULLY-COVERED`
- **§11.4.172**: Living production-readiness planning document with realistic timeline projections from measured velocity
- **§11.4.172**: Identifies danger zones, HW delivery timelines, licensing delays, external dependency risks
- **§11.4.172**: Defines critical path; includes workstation/build-capacity analysis
- **§11.4.172**: Updated monthly or when item count changes by ≥10%
- **§11.4.172**: Cross-referenced against workable-items database
- **§11.4.172**: Document lives at `docs/research/production_planning_<YYYYMMDD>/ANALYSIS.md`

### 7.7 Web/UI Quality
- **§11.4.162**: Every project producing user-facing interfaces MUST use OpenDesign as mandatory UI design-and-refinement system
- **§11.4.162**: Use design tokens/themes for color palette (light+dark), typography, spacing, component-level tokens
- **§11.4.162**: Every UI component MUST ship light + dark theme variants
- **§11.4.162**: Elements MUST NOT overlap, fonts MUST NOT collide, labels MUST NOT overlay labels
- **§11.4.162**: All UI changes covered by visual regression tests
- **§11.4.170**: Every UI surface change MUST be proven by device-independent host-side RENDERED PIXELS (PNG on host via Compose→Roborazzi/Paparazzi, web→Playwright/Storybook, etc.)
- **§11.4.170**: Verified by BOTH golden image-diff AND OCR/vision oracle reading rendered text+labels+control bounds
- **§11.4.170**: Value/token-equality / property-assertion UI tests FORBIDDEN as the sole proof
- **§11.4.170**: "device offline" is NEVER valid skip — host-render IS device-independent path
- **§11.4.170**: Gate `CM-COVENANT-114-170-PROPAGATION` + `CM-HOST-RENDERED-UI-VISUAL-PROOF`
- **§11.4.190**: Every project website MUST be: fully responsive (all browsers/OSes/device classes/screen sizes), completely SEO-optimized, uniquely OpenDesign-authored, bleeding-edge enterprise-quality in light+dark — each PROVEN with captured evidence
- **§11.4.190**: Responsiveness proven by host-rendered screenshots across breakpoint×engine matrix + OCR layout oracle
- **§11.4.190**: SEO proven by automated audit (Lighthouse SEO + structured-data + meta/OG/sitemap/robots) meeting defined score floor
- **§11.4.190**: Gate `CM-COVENANT-114-190-PROPAGATION` + `CM-WEBSITE-RESPONSIVE-PROVEN` / `CM-WEBSITE-SEO-OPTIMIZED` / `CM-WEBSITE-OPENDESIGN-UNIQUE`

### 7.8 Independent Verification
- **§11.4.165**: Every code/media/docs/config output MUST pass INDEPENDENT verifier (structurally separate from author), iterating to GO per §11.4.134
- **§11.4.165**: CODE: review + build+test + paired §1.1 mutations + §11.4.108 runtime-signature
- **§11.4.165**: MEDIA: §11.4.163 pipeline + genuine-content check + format check
- **§11.4.165**: DOCS: exports current §11.4.65 + revision header §11.4.44
- **§11.4.165**: CONFIG: syntax + schema + §11.4.10 leak check
- **§11.4.165**: Gate `CM-COVENANT-114-165-PROPAGATION` + `CM-INDEPENDENT-VERIFICATION-AGENT`
- **§11.4.163**: Every recorded artifact MUST pass MEDIA VALIDATION pipeline — OCR for video/screenshots, transcription for audio, text parsing
- **§11.4.163**: Compare extracted content against SPECIFY-phase expected patterns
- **§11.4.163**: Self-validated analyzer with golden-good/golden-bad fixture pair
- **§11.4.163**: Gate `CM-COVENANT-114-163-PROPAGATION` + `CM-MEDIA-VALIDATION-PIPELINE`
- **§11.4.160**: Every video recording processed through vision/OCR pipeline that confirms expected results BEFORE acceptance
- **§11.4.160**: Bridge feeding captured frames to HelixQA for automated read-the-screen verification at ≤5s intervals
- **§11.4.160**: Gate `CM-COVENANT-114-160-PROPAGATION` + `CM-VISION-VERIFIED-RECORDING-BRIDGE`

### 7.9 Miscellaneous Infrastructure
- **§11.4.72**: Audio fixes are ALWAYS top priority in main working stream
- **§11.4.72**: Any time conductor faces audio vs non-audio choice on SAME serial resource, audio wins
- **§11.4.82**: 9 mandatory speedup disciplines: Phase 1 forensic, Live-ADB-First, pre-flight, persistent build caches, module-only rebuild, parallel multi-device testing, subagent scope discipline, lock-file hygiene, cycle telemetry
- **§11.4.82**: Gate `CM-ITERATION-SPEEDUP-DISCIPLINE`
- **§11.4.83**: Every shipped feature MUST carry recorded e2e communication transcript under `docs/qa/<run-id>/`
- **§11.4.101**: In autonomous mode, minimize operator-blocking — make safe/reliable/reversible decision autonomously when all hold: (a) reversible OR pre-op backup, (b) safe choice determinable, (c) wrong-choice blast radius bounded
- **§11.4.101**: BLOCK ONLY when action irreversible AND high-blast-radius AND safe choice undeterminable
- **§11.4.101**: Maximize-progress-while-blocked — unavoidable block parks one work unit, not the loop
- **§11.4.101**: Gate `CM-COVENANT-114-101-PROPAGATION`
- **§11.4.140**: Universal action-prefix system — five equivalent forms: `ACTION ::`, `PREFIX::ACTION ::`, `/ACTION`, `/PREFIX::ACTION`, `ACTION --->`
- **§11.4.140**: Two registered actions: BACKGROUND (background subagent-driven parallel work) and REMINDER (re-surface status-uncertain work)
- **§11.4.140**: Action registry at `constitution/actions/registry.yaml`
- **§11.4.35**: Constitution submodule's 3 files ARE the canonical root; consumer files are extensions
- **§11.4.35**: Universal rules → constitution submodule; project-specific → consumer's file
- **§11.4.35**: Pre-build gate `CM-CANONICAL-ROOT-CLARITY`
- **§§**: Five mirror carriers: GEMINI.md locked in step with CLAUDE.md / AGENTS.md / QWEN.md (§11.4.157)
- **§§**: Indices for CodeGraph: own-org submodules INCLUDED, third-party EXCLUDED (§11.4.79)
- **§§**: Subagent model-tiering: mechanical tasks to Haiku-class, judgment tasks to Opus-class — NEVER tier down a verdict (§11.4.141, subagent_tiering.yaml)

---

## 8. MISSING / UNIMPLEMENTED FEATURES THAT ARE REQUIRED

### 8.1 Planned Gates (Not Yet Implemented)
- `CM-CODEGRAPH-WIRED` (§11.4.78) — not yet implemented (planned)
- `CM-GITIGNORE-REGEN-MECHANISM` (§11.4.77) — not yet implemented (planned)
- `CM-CONTAINERS-USED` (§11.4.76) — not yet implemented (planned)
- `CM-SUBAGENT-DELEGATION-AUDIT` (§11.4.20) — not yet implemented (per consuming project)
- `CM-FETCH-BEFORE-EDIT-AUDIT` (§11.4.37) — not yet implemented (per consuming project)
- `CM-ITERATION-SPEEDUP-DISCIPLINE` (§11.4.82) — not yet implemented (when implemented)
- `CM-CROSS-PLATFORM-PARITY` (§11.4.81) — not yet implemented (when implemented)
- `CM-HUMAN-READABLE-DESCRIPTION` (§11.4.171) — not yet implemented (when implemented)
- `CM-SUMMARY-CLARITY-DESCRIPTIONS` (§11.4.91) — not yet implemented (when implemented)
- `CM-PROCEDURES-DOCS-PRESENT` (§11.4.63) — planned
- Various recommended gates throughout §11.4: listed as "recommended" or "when implemented"

### 8.2 Partially Implemented / In Progress
- **§11.4.106 Docs Chain**: Engine Phases 1-3 AND CLI/YAML loader (Phase 4) implemented + tested; NOT yet registered as git submodule (Phase 6 remaining — PLANNED + OPERATOR-GATED)
- **§11.4.93 SQLite SSoT for workable items**: Migration pending — Go binary `cmd/workable-items/` planned
- **§11.4.149 Testing diary**: Schema and exports planned
- **§11.4.148 External-tracker sync**: Generic idempotent push planned
- **§11.4.116 Real-time conductor↔test-framework sync channel**: Owned project-agnostic submodule planned
- **§11.4.186 Cross-document consistency gate**: Reusable engine under constitution submodule planned (depth-1 carve-out per §11.4.28(C))

### 8.3 Known Issues / Stale Items
- **Constitution.md header "Issues"**: ToC is currently stale (missing entries for §11.4.71/.72/.73/.74/.75/.76) — separate clean-up commit needed per §11.4.61
- **§11.4.166 REPEALED**: Universal Semgrep static-analysis mandate repealed by operator decision (2026-06-22)

### 8.4 Ongoing Obligations with No Explicit Implementation Deadline
- **§11.4.69**: Bare `ab_pass` deprecated — WARN pre-grace, FAIL post-grace (2026-06-19 deadline passed)
- **§11.4.99**: Re-verification cadence — 6 months (90 days for risk-classified)
- **§11.4.98**: NON-COMPLIANT manual-interaction tests not rewritten in 30 days → §11.4.90 Obsolete
- **§11.4.42 update cadence**: Items re-evaluated every Nth tag cycle (≥3rd recommended)
- **§11.4.172**: Planning document updated monthly or on ≥10% item-count change
- **§11.4.80**: CodeGraph weekly update floor
- **§11.4.47**: Firebase review on 5-trigger cadence (pre-build/pre-flash/pre-tag blocking; daily/burn-in non-blocking)
- **§11.4.123**: "unclear how to validate" MUST trigger deep web research — ongoing
- **§11.4.150**: Deep multi-angle research for every change — ongoing
- **§11.4.183**: Full constitution application on every track — ongoing, zero exceptions

---

## 9. CROSS-CUTTING REQUIREMENTS

### 9.1 Propagation Gates (All CM-COVENANT-114-NNN-PROPAGATION)
Every §11.4.N anchor has a corresponding propagation gate that verifies the literal `11.4.N` token exists across the consumer fleet (~42-44 files). These are:
- `CM-COVENANT-114-16-PROPAGATION` through `CM-COVENANT-114-192-PROPAGATION` (and beyond where numbered)
- Each requires paired §1.1 meta-test mutation that strips the literal → gate FAILs

### 9.2 Governance Mirrors
- **§11.4.157**: GEMINI.md is first-class governance carrier EQUAL to CLAUDE.md/AGENTS.md/QWEN.md — five-carrier lockstep
- **§11.4.26**: Constitution/CLAUDE/AGENTS edits follow strict fetch→edit→validate→commit→push→validate pipeline

### 9.3 Manual QA Final Confirmation
- **§11.4.185**: No scope of work / release considered fully completed until manual QA-team testing as FINAL step
- **§11.4.185**: Agent hands off and waits — never self-certifies manual step
- **§11.4.185**: Gate `CM-COVENANT-114-185-PROPAGATION` + `CM-MANUAL-QA-FINAL-CONFIRMATION`

### 9.4 Autonomous Loop Default
- **§11.4.126**: Endless fully-autonomous loop is DEFAULT working mode from first prompt
- **§11.4.126**: Loop continues until release scope tag published OR non-release scope fully completed
- **§11.4.126**: Stops ONLY on explicit operator STOP, empty queue, or §12 host-safety
- **§11.4.126**: No mimicking/imitation/bluff — ABSOLUTELY FORBIDDEN
- **§11.4.87**: Endless-loop covenant obligations: continue until all items terminal, dispatch background subagents, four-layer coverage with physical proofs, anti-bluff operative end-to-end, loop terminates only on conditions-met/operator STOP/host-safety
- **§11.4.94**: Zero-idle priority-first parallel-by-default operating mode always-on
- **§11.4.97**: Maximum-use-of-idle-time — every minute of progressable idle time = violation
- **§11.4.97**: Emit 1-line operator update at every commit/subagent-return/anchor-landed
- **§11.4.185**: Manual QA as final confirmation step — the §11.4.126 release-tag terminal condition now requires this

### 9.5 Data Safety (§9)
- **§9**: Every destructive operation on repository is safety-critical
- **§9.1**: Full safety protocol for destructive operations (backup, record, identify, run, verify, restore on fail)
- **§9.2**: Force-push requires explicit user authorization every time
- **§9.3**: Hardlinked backup is standard — no excuse
- **§9.4**: Commit-message audit trail for history rewrites

### 9.6 Enforcement (§10)
- **§10**: Any commit/tag/release violating §1-§9 is non-compliant
- **§10**: Fix is to amend/revert and re-land
- **§10**: Data-safety violations (§9) and host-session-safety violations (§12) = catastrophic — block entire release cycle

### 9.7 Quality Bar (§8)
- **§8**: Acceptance bar: user on freshly-deployed system sees expected effect — confirmed end-to-end with captured evidence on actual target environment
- **§8**: Forbidden: source rebrand without shipped artifact rebuild
- **§8**: Forbidden: tests pass but feature broken
- **§8**: Forbidden: configuration-only tests
- **§8**: Forbidden: "works on my workstation"
- **§8**: Forbidden: "will fix in next cycle" without documentation

### 9.8 False Success Prevention (§7)
- **§7**: Every gate reports concrete PASS/FAIL/SKIP with explicit reason text
- **§7**: Every SKIP reason mechanically distinguishable from PASS
- **§7**: FAILs counted towards exit status — script hiding FAILs in logs = bug
- **§7**: Meta-tests catch any gate that always PASSes
- **§7**: Runtime results cross-referenced against host-side evidence

### 9.9 Overall Document Quality
- **§11.4.61**: Mandatory Markdown metadata table + structured-doc ToC for every tracked document
- All documents carry status header: Field | Value with Revision, Created, Last modified, Status, Status summary, Issues, Issues summary, Fixed, Fixed summary, Continuation
