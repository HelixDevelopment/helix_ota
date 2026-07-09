# Deferred-Brick Mirror-Fork Analysis & Safe-Convergence Brief

**Revision:** 1
**Last modified:** 2026-07-09T20:33:57Z
**Author:** Stream MF (autonomous READ-ONLY investigation subagent)
**Scope:** `submodules/llm_orchestrator` + `submodules/vision_engine`
**Constraint authority:** §11.4.113 (absolute no-force-push, merge-onto-latest-main) · §9.2 (zero commit loss) · §11.4.6 (no guessing)

---

## 0. Executive summary

Two owned Go bricks were deferred by the gofmt sweep because their git **mirrors have
diverged** — a pre-existing fork, NOT caused by the sweep. This brief establishes the
exact fork facts (bidirectional, with authorship + content), determines the safe
lossless convergence per §11.4.113, and states the single operator decision each needs.

| Brick | origin unique | github/upstream ahead by | Merge onto github base | Commit loss? | gofmt residual (github tip) |
|---|---|---|---|---|---|
| `llm_orchestrator` | 1 commit (`a484f7d`, **ours**) | +34 | **add/add CONFLICT** on `helix-deps.yaml` (trivial, content decision) | **None** | **0 files — sweep is a no-op** |
| `vision_engine` | 1 commit (`b417a40`, **ours**) | +12 | **CLEAN** (disjoint paths, no conflict) | **None** | **17 files — sweep applies after converge** |

Both origin-unique commits are **ours** (author `i@mvasic.ru`, Milos Vasic / Милош Васић),
**real changes** (not merge artifacts), and are **preserved** by the recommended
merge — nothing is lost in either direction. **Recommended canonical base for both:
the github/upstream tip** (the most-advanced mirror; identical `github`==`upstream`).

Neither convergence requires — nor permits — a force-push. Both are fast-forward pushes
after the merge because the merge commit descends from every mirror tip.

---

## 1. Shared facts (verified 2026-07-09, read-only `git fetch --all --prune`)

Both bricks are on detached `HEAD` at their parent-pinned commit. Remotes for each:

- `origin` fetch = `vasic-digital/<Repo>` · `origin` **push** = `HelixDevelopment/<Repo>`
- `github` = `upstream` = `HelixDevelopment/<Repo>` (identical tips)

| Brick | `origin/master` | `github/master` = `upstream/master` | Parent pins | Parent pin is… |
|---|---|---|---|---|
| `llm_orchestrator` | `a484f7d` | `ee229a7` | `a484f7d` | the **origin tip** |
| `vision_engine` | `b417a40` | `a97df79` | `0bf75ee` | the **merge-base** (behind BOTH) |

---

## 2. `llm_orchestrator` (branch `master`)

### 2.1 Fork geometry
- **Common ancestor (merge-base):** `d2a2151` — *"cascade: chore(1.1.8-dev-rc1 cascade)…"*, Милош Васић, 2026-06-02.
- `git merge-base --is-ancestor origin github` → **NO** (genuine divergence, not a stale-behind).
- **origin = merge-base + 1** commit; **github = merge-base + 34** commits.

### 2.2 origin's unique commit github lacks (`github..origin`) — exactly ONE
```
a484f7d  chore(deps): add §11.4.31 helix-deps.yaml dependency manifest
         Author: Milos Vasic <i@mvasic.ru>   Date: 2026-06-08 21:23:28 +0500
         Parent: d2a2151 (the merge-base)
         Adds: helix-deps.yaml | 32 insertions (new file)   Co-Authored-By: Claude Opus 4.8
```
**Ours / foreign:** OURS (`i@mvasic.ru`). **Real change / artifact:** real change — a
new 32-line `helix-deps.yaml` declaring the LLMProvider own-org dep
(`git@github.com:vasic-digital/LLMProvider.git`, `ref: main`, flat layout).

### 2.3 What github has that origin lacks (`origin..github`) — 34 commits
github is a genuine, far-more-advanced line: a `CONSTITUTION.md` full rewrite +
wrong-project ("HelixCode") decontamination, `§11.4.29` lowercase-upstreams migration,
anti-bluff test hardening, `§11.4.103–165` governance cascade, real bug-fixes
(`a446a3d` protocol ctx-cancel message loss, `4e08b93` RoundRobinSelector, `1fac8b1`
SimpleAgentPool.Release, `c6d6049` opencode Wait() hang), **and `974d784` "style: gofmt
-w hygiene across llm_orchestrator"** — i.e. the gofmt work the sweep intended is
**already on github's line**. All authored by our org. This is **not** a foreign
line — it is our own, later, decontaminated + gofmt-clean trunk that origin never
received.

### 2.4 The conflict (verified via `git merge-tree --write-tree github origin`)
```
CONFLICT (add/add): Merge conflict in helix-deps.yaml
```
**Both mirrors independently added `helix-deps.yaml`:**
- **origin** (`a484f7d`, 2026-06-08, 32 lines): declares `LLMProvider` as an own-org dep.
- **github** (`3ac8124`, 2026-06-17, 8 lines): `deps: []` with a `§11.4.74` catalogue-check
  note *"own-org deps: none"*.

These two files **semantically contradict** (LLMProvider-is-a-dep vs no-own-org-dep).
This is the one point that needs a human decision — see §4.1. It is a **content
reconciliation**, NOT a commit loss: merging preserves both `a484f7d` and all 34 github
commits in history; only the file's final bytes are chosen.

> Honest boundary (§11.4.6): I did **not** open `go.mod` to decide which manifest is
> factually correct (whether LLMProvider is still a live `replace`). That is the
> operator/author call flagged in §4.1 — I state it as UNKNOWN rather than guess.

### 2.5 gofmt residual
github tip `ee229a7` → `gofmt -l` = **0 files**. Once converged onto the github base the
gofmt sweep is a **no-op** for this brick.

---

## 3. `vision_engine` (branch `master`)

### 3.1 Fork geometry
- **Common ancestor (merge-base) = the parent pin `0bf75ee`** — *"Merge remote-tracking
  branch 'origin/master'"*, Милош Васић, 2026-06-16. Ancestor of BOTH mirrors (verified).
- `git merge-base --is-ancestor origin github` → **NO** (genuine divergence).
- **origin = merge-base + 1**; **github = merge-base + 12**. The **parent currently pins
  the common ancestor** — behind github by 12 and behind origin by 1.

### 3.2 origin's unique commit github lacks (`github..origin`) — exactly ONE
```
b417a40  chore(governance): add install_upstreams.sh + Upstreams/{GitHub,GitLab} (§11.4.36 / §6.W)
         Author: Милош Васић <i@mvasic.ru>   Date: 2026-07-04 16:45:18 +0300
         Parent: 0bf75ee (the merge-base)
         Adds: Upstreams/GitHub.sh (9) + Upstreams/GitLab.sh (9) + install_upstreams.sh (109) = 127 insertions
```
**Ours / foreign:** OURS (`i@mvasic.ru`). **Real change / artifact:** real change — adds a
root `install_upstreams.sh` (not present at github tip) plus **capital-`Upstreams/`**
recipe scripts.

### 3.3 What github has that origin lacks (`origin..github`) — 12 commits
Our own later line: wrong-project decontamination, `§11.4.29` **lowercase `upstreams/`
migration** (`9553a31` "lowercase upstream scripts; simplify push-all.sh"), anti-bluff
test hardening, Dockerfile non-root hardening (`f3ef1f2`), `§11.4.142-165` cascade. All
`i@mvasic.ru` / our org. Genuine trunk, not foreign.

### 3.4 The merge (verified via `git merge-tree --write-tree github origin`)
```
229c431e59f16d5bced0b2fb64beb6ef82504a60   ← tree OID only, NO conflict lines, exit 0
```
**CLEAN merge — zero conflicts.** origin's `b417a40` touches `Upstreams/*` (capital U) +
`install_upstreams.sh`; github's line touches `upstreams/*` (lowercase). On the
case-sensitive Linux host these are **disjoint paths**, so git auto-merges cleanly and
both lines' commits are preserved.

**Caveat for the operator (not a blocker):** the clean merge **reintroduces the
capital-`Upstreams/` directory**, which is a **§11.4.29 lowercase-naming concern** — the
github line deliberately migrated to lowercase `upstreams/`. `install_upstreams.sh` at
root is genuinely new and wanted (§11.4.36); the capital `Upstreams/GitHub.sh` +
`GitLab.sh` are redundant with the existing lowercase recipes. See §4.2.

### 3.5 gofmt residual
github tip `a97df79` → `gofmt -l` = **17 files** (identical set on origin tip — the gofmt
hygiene was never done on this brick's line): `cmd/visiondescribe/main.go`,
`pkg/analyzer/{i18n_defaults,types,types_test}.go`,
`pkg/config/i18n_callsites_test.go`, `pkg/graph/{graph,graph_automation_test}.go`,
`pkg/llmvision/{anthropic,astica,i18n_defaults,provider,provider_test}.go`,
`pkg/opencv/{i18n_defaults,orb_vision_test}.go`,
`pkg/remote/{deployer_test,remote,remote_test}.go`. The **gofmt sweep still applies to
this brick and must run AFTER convergence** (on the merged tree).

---

## 4. Operator decision needed

### 4.1 `llm_orchestrator` — resolve the `helix-deps.yaml` add/add conflict
The only real decision. Two contradictory manifests exist:
- **origin `a484f7d`:** LLMProvider IS an own-org dep (via `go.mod replace`).
- **github `3ac8124`:** `deps: []` — own-org deps: none.

Operator (or brick author) must confirm — from the brick's current `go.mod` — which is
factually correct, then the merge keeps that content. If LLMProvider is still a live
`replace`, keep origin's richer 32-line manifest; if the dep was removed, keep github's
`deps: []`. **UNKNOWN to this read-only investigation — flagged, not guessed (§11.4.6).**

### 4.2 `vision_engine` — capital `Upstreams/` vs lowercase `upstreams/`
The merge is clean but reintroduces capital `Upstreams/`. Operator chooses one:
- **(a)** Keep `install_upstreams.sh` (wanted, §11.4.36) but **drop / lowercase-rename**
  the capital `Upstreams/GitHub.sh`+`GitLab.sh` during the merge to honour §11.4.29 (the
  github line already has lowercase equivalents) — **recommended**; or
- **(b)** Accept the capital dir as-is (clean auto-merge, but a standing §11.4.29 nit).

---

## 5. Recommended safe convergence (§11.4.113 — merge-onto-latest, FF-push, NEVER force)

**Canonical base for BOTH bricks: the `github`/`upstream` tip** (most-advanced mirror;
`github`==`upstream`; already decontaminated; llm_orchestrator already gofmt-clean).

Per-brick procedure (executed by the conductor, not this read-only stream):

1. `git fetch --all --prune --tags` (all remotes).
2. Check out / base on the **github tip** (`ee229a7` / `a97df79`).
3. **Merge the origin unique commit** on top:
   - `llm_orchestrator`: merge `a484f7d`; resolve the `helix-deps.yaml` add/add per §4.1
     (no `-s ours`, no drop — union history, chosen content).
   - `vision_engine`: merge `b417a40` (auto-clean); apply the §4.2 choice.
4. `vision_engine` only: **run the gofmt sweep** on the merged tree (17 files) and commit.
5. Audit: no conflict markers, no file silently dropped, prior tests still green.
6. **Fast-forward push to ALL upstreams** (`origin` push→HelixDevelopment, `github`,
   `upstream`, + GitLab/GitFlic/GitVerse fan-out). Each push is a fast-forward because
   the merge commit descends from every mirror tip — **no force is ever needed**.
7. Advance the parent `.gitmodules` pointer to the new merged tip in the same change
   window.

**Commit-loss check (§9.2):** NONE in either direction. The merge preserves origin's 1
commit AND github's 34 / 12 commits. The recommended base flips each parent pin FORWARD
(llm_orchestrator a484f7d→merged, vision_engine 0bf75ee→merged) with zero history rewrite.

**Force-push:** not required and forbidden (§11.4.113) — the merge-onto-latest path makes
every push a fast-forward.

---

## 6. Verification trail (read-only commands run)

- `git fetch --all --prune` (both bricks) — remote tips captured.
- `git merge-base` / `--is-ancestor` — divergence + ancestor facts.
- `git log --oneline <a>..<b>` both directions — unique-commit sets.
- `git show --stat --format=…` — authorship + content of each origin-unique commit.
- `git ls-tree -r --name-only <tip>` — file presence (helix-deps.yaml, upstreams paths).
- `git merge-tree --write-tree <github> <origin>` — conflict simulation (no working-tree
  or ref mutation): llm_orchestrator = add/add CONFLICT; vision_engine = clean.
- `git archive <tip> | tar -x` to scratchpad + `gofmt -l` — residual gofmt (0 / 17),
  scratch cleaned after.

No ref, file, or working tree in either submodule or the parent was modified. Only this
document was written.
