# Helix OTA — AGENTS.md

> Base agent rules live at `constitution/AGENTS.md` and the
> `constitution/Constitution.md` it references. **READ THOSE FIRST.**
> The base file is authoritative for any topic not covered here. This
> file extends them with project-specific rules; it never weakens them.

## Critical base rules restated (for agents that don't follow @imports)

- **No bluffing.** Every PASS carries positive evidence. Constitution §11.4.
- **Mutation-paired gates.** Every new gate has a paired mutation
  proving it catches regressions. Constitution §1.1.
- **No guessing language.** `likely`, `probably`, `maybe`, `seems`,
  `appears` etc. are forbidden when reporting causes. Constitution §11.4.6.
- **Credentials never tracked.** `.env` patterns git-ignored;
  runtime-load only; per-service file separation. Constitution §11.4.10.
- **Never force-push.** Force-push requires explicit per-session
  authorization. Constitution §11.4.113.
- **Hardlinked backup before any destructive op.** Constitution §9.
- **CONTINUATION document kept in sync.** Constitution §12.10. The live
  handoff for this project is `docs/research/main_specs/CONTINUATION.md`
  (and `.remember/remember.md` when present).
- **60% RAM cap.** Heavy work wrapped in bounded execution scope.
  Constitution §12.6.
- **Per-feature Status + video confirmation.** Constitution §11.4.153.
  `docs/features/Status.md` + `Status_Summary.md` enumerate every feature
  with per-row video-recording confirmation; HTML+PDF+DOCX exports in sync.
- **Window-scoped capture + fresh-corpus rotation.** Constitution §11.4.154.
  Recordings capture only the app window/surface, not whole desktop; a new
  run removes its own prior in-scope recordings first.
- **Project-prefixed recording filenames.** Constitution §11.4.155. Every
  recording filename starts with `<PREFIX>---` (resolved from
  `HELIX_RELEASE_PREFIX` env var or project-root dir name), not a single
  hyphen.
- **All CI/CD disabled.** Constitution §11.4.156. No GitHub Actions or
  GitLab pipelines active; enforcement is local-only via pre-build gates
  and git hooks.
- **GEMINI.md lockstep.** Constitution §11.4.157. GEMINI.md kept in sync
  with CLAUDE.md/AGENTS.md/QWEN.md at all times.
- **Intensive recording + read-the-screen content verification.**
  Constitution §11.4.158. Every feature/flow/edge-case recorded; every
  recording's on-screen content machine-read and verified as a genuine
  working result.
- **Window-specific MP4 + vision validation.** §11.4.159. Every recording must be window-specific MP4 with expected-content specification BEFORE recording, vision validation AFTER recording, terminal window cleanup, and content-verification workflow.
- **Vision-verified recording + HelixQA bridge.** Constitution §11.4.160.
  Recordings processed through a vision/OCR bridge confirming expected
  results before acceptance.
- **Rootless containers.** Constitution §11.4.161. The `containers/` infra
  runs Podman rootless via the `vasic-digital/containers` submodule; no
  Docker-rootful / sudo / root escalation.
- **OpenDesign UI system.** Constitution §11.4.162. Any user-facing UI uses
  OpenDesign design tokens (light+dark, visual-regression covered); latent
  until a UI surface ships.
- **Media validation pipeline.** Constitution §11.4.163. Every recorded
  artifact validated (OCR / transcription / parse vs SPECIFY-phase patterns,
  self-validated analyzer) before acceptance.
- **Constitution auto-propagation hook.** Constitution §11.4.164. After
  every constitution pull run `constitution/scripts/post_update_hook.sh`
  (inherited by reference) to register/install changed skills/MCP/hooks.
- **Independent verification agent.** Constitution §11.4.165. Every code
  change OR media artifact passes an independent verifier iterating to a
  zero-finding GO.
- **Semgrep static analysis.** Constitution §11.4.166. Semgrep installed +
  on PATH; `semgrep scan --config auto --error` runs before every commit /
  push and blocks on findings; scripts inherited by reference from
  `constitution/scripts/semgrep/*`.
- **Big-work-item feature work-stream lifecycle.** Constitution §11.4.167.
  Every BIG feature/large fix develops as its own isolated feature
  work-stream (CoW/reflink project copy, own `feature/<slug>` branch +
  tags, own builds), separate from trunk until operator-approved after
  full retest; trunk merged in regularly; submodule branch/tag cascade
  (composes §11.4.58/.103/.113/.142/.145). Gate
  `CM-COVENANT-114-167-PROPAGATION`.
- **Exported-document independent visual validation.** Constitution
  §11.4.168. Every exported document (HTML/PDF/DOCX) passes independent
  validation (reviewer structurally separate from the generator) across
  CONTENT, TEXTUAL, and FULL-VISUAL layers, verified by rendering + OCR
  with a self-validated golden-good/golden-bad analyzer (composes
  §11.4.65/.107/.117/.134/.142). Gate `CM-COVENANT-114-168-PROPAGATION`.
- **Mandatory comprehensive test-type coverage.** Constitution §11.4.169.
  Every project covers the closed enumerated test-type set (unit/
  integration/e2e/full-automation/Challenges/HelixQA/DDoS/security/
  stress+chaos/concurrency/race-deadlock/memory/benchmarking), each PASS
  citing rock-solid captured physical evidence (composes §11.4.25/.27/
  .50/.52/.69/.85/.107). Gate `CM-COVENANT-114-169-PROPAGATION`.
- **Host-rendered UI visual-proof mandate.** Constitution §11.4.170.
  Every UI-surface change is proven by device-independent host-rendered
  pixels (Compose/Playwright/Storybook/snapshot-testing class) per
  screen×state×{light,dark} theme, validated by golden image-diff AND an
  OCR/vision layout oracle; value/token-equality unit tests are FORBIDDEN
  as the sole proof (composes §11.4.107/.153/.158/.159/.160/.162/.168).
  Gate `CM-COVENANT-114-170-PROPAGATION`.
- **Human-readable workable-item descriptions.** Constitution §11.4.171.
  Every workable item carries a ≥5-7-sentence plain-language description
  (what/why/how/who-benefits/expected-outcome) understandable by
  non-developers; the SQLite DB `description` column is the single
  source of truth mirrored into every derived doc (composes §11.4.15/.16/
  .91/.93/.148). Gate `CM-COVENANT-114-171-PROPAGATION`.
- **Production-readiness planning mandate.** Constitution §11.4.172. Every
  project maintains a living planning document with realistic timeline
  projections from measured velocity, danger-zone/risk identification,
  and critical-path analysis, updated monthly or on ≥10% item-count
  change (composes §11.4.6/.40/.42/.93/.108). Gate
  `CM-COVENANT-114-172-PROPAGATION`.
- **Containerized + distributed build mandate.** Constitution §11.4.173.
  Every build of every component runs inside a specialized build
  container via the containers submodule, distributed to the designated
  remote build host (never the bare host), artifacts brought back for
  use/flashing (composes §11.4.28/.74/.76/.108/.161). Gate
  `CM-COVENANT-114-173-PROPAGATION`.
- **Shared-host process-ownership verification.** Constitution §11.4.174.
  Before inspecting or acting on any process/build/daemon/port/lock on a
  shared host, positively verify the target is OURS (cwd/argv/
  recorded-PID/lock-path) — never a loose name-match `pgrep`; never kill
  a process not positively ours (composes §11.4.66/.101/.133/.147). Gate
  `CM-COVENANT-114-174-PROPAGATION`.
- **Multi-track work-division + device-lock arbitration.** Constitution
  §11.4.176. (A) exactly-once work-item/logical-group claim registry;
  (B) capability-aware deadlock-proof device-lock (all-or-nothing,
  non-blocking, TTL-reap breaking Coffman conditions); (C) universal/
  decoupled + evidence-based resource tuning (composes §11.4.58/.116/
  .119/.147/.167). Gate `CM-COVENANT-114-176-PROPAGATION`.
- **Developer-tooling project-decoupling.** Constitution §11.4.177. No
  project-specific script/hook/alias may be wired into a global/shared
  developer-tooling PATH; shared tooling is project-agnostic and operates
  on the invocation directory, never a hardcoded project path (composes
  §11.4.28/.29/.35). Gate `CM-COVENANT-114-177-PROPAGATION`.
- **Track-qualified identity for parallel streams.** Constitution
  §11.4.178. Parallel work streams sharing hardware/git/session/lock
  namespaces are addressed by a track-qualified identity
  (`<project>__<track>__<role>`), never a bare basename — prevents
  session/lock/log/device cross-wiring (composes §11.4.111/.119/.176).
  Gate `CM-COVENANT-114-178-PROPAGATION`.
- **Corruption-isolated parallel git streams.** Constitution §11.4.179.
  Parallel git work streams requiring corruption-isolation are each an
  independent repo with its OWN `.git` (own object store/index/lock
  namespace), NOT `git worktree` checkouts sharing one common `.git`
  (composes §11.4.58/.119/.167/§9.2). Gate `CM-COVENANT-114-179-PROPAGATION`.
- **Stale-lock auto-reap before commit/push.** Constitution §11.4.180.
  Every commit/push wrapper auto-reaps a git lock whose recorded holder
  PID is dead (or, absent a PID, stale past a defined threshold with no
  live holder) before acquiring its own lock — NEVER removes a lock whose
  holder is alive (composes §11.4.84/.88/§9.2/.179). Gate
  `CM-COVENANT-114-180-PROPAGATION`.
- **Consistent feature-branch naming.** Constitution §11.4.181. One
  feature/logical group of workable items maps to EXACTLY ONE canonical
  branch name used identically on the main repo AND every touched owned
  submodule, recorded once in the claim registry, never re-invented
  (composes §11.4.29/.93/.113/.167/.176). Gate
  `CM-COVENANT-114-181-PROPAGATION`.
- **Track+branch work-stream identity label.** Constitution §11.4.182.
  Every agent/subagent label and operator-facing work-stream reference
  starts with a `(T<N>/<branch> - <alias>)` prefix, deterministically
  derived (never guessed), mechanically enforced by a PreToolUse guard
  hook (composes §11.4.75/.109/.178). Gate `CM-COVENANT-114-182-PROPAGATION`.
- **Maximal multi-agent utilization + full-constitution application.**
  Constitution §11.4.183. Every track maximizes applicable multi-agent
  working approaches (subagent-driven dev, independent review agents,
  parallel background streams) and applies the ENTIRE constitution —
  nothing skipped — with zero false/faulty/unverified results anywhere
  (composes §11.4.20/.58/.70/.125/.142/.165). Gate
  `CM-COVENANT-114-183-PROPAGATION`.
- **SonarQube CLI + tooling mandate.** Constitution §11.4.184. The
  SonarQube scanner CLI (`sonar-scanner`) is installed AND durably
  PATH-discoverable via shell rc, plus the shared
  `constitution/scripts/sonarqube/` tooling consumed by reference (never
  copied); proven GREEN by install-check exit 0, never assumed; rootless
  podman only (composes §11.4.28/.74/.161/.166). Gate
  `CM-COVENANT-114-184-PROPAGATION`.
- **Manual QA final confirmation mandate.** Constitution §11.4.185. No
  scope of work / release is considered fully completed until it has
  received MANUAL testing confirmation by the project's QA team as the
  FINAL step — every automated gate is necessary but NOT sufficient; the
  agent hands off and waits, never self-certifies the manual step
  (composes §11.4.40/.52/.108/.126). Gate `CM-COVENANT-114-185-PROPAGATION`.
- **Anti-divergence cross-document consistency gate.** Constitution
  §11.4.186. Any project maintaining more than one representation of the
  same tracked data enforces cross-document consistency as a
  deterministic PASS/FAIL/SKIP gate running BEFORE any export render,
  doc/DB sync verify, or doc-set commit — never an after-the-fact audit;
  five decidable check families + self-validated analyzer (composes
  §11.4.86/.93/.106/.107/.148). Gate `CM-COVENANT-114-186-PROPAGATION`.
- **Process/thread-limit (RLIMIT_NPROC) awareness.** Constitution §12.12.
  Heavy parallel subagent/multi-process work is bounded by the OS
  per-user process/thread limit (`ulimit -u` / `RLIMIT_NPROC`); check
  thread headroom before scaling parallelism and treat exhaustion as a
  §12 host-safety event that yields unconditionally — an orthogonal axis
  to the §12.6 memory ceiling (composes §12.6/.7/§11.4.58/.101/.103/
  .122). Gate `CM-COVENANT-12-12-PROPAGATION`.

## Project-specific agent rules

### Allowed CLI tools

- `go`, `gofmt`, `go vet` (server + Go bricks)
- `gradle` (Android bricks), `git`, `gh`, `glab`
- `redocly`, `kubeconform`, `mmdc`, `pandoc` (spec/doc validation)

### Project-specific workflow

- Commit + push to ALL four upstreams regularly; merge to `main` when a
  milestone is done.
- New reusable submodule repos are created PUBLIC on GitHub + GitLab
  under the HelixDevelopment / vasic-digital orgs (pre-authorized).
- `docs/research/main_specs/additions/` files are authoritative input —
  always analyze + fold them in.

### Inheritance verification

Before a build/merge the inheritance gate MUST pass:
`bash tests/test_constitution_inheritance.sh` (gate + §1.1 paired
mutation via `constitution/meta_test_inheritance.sh`).
