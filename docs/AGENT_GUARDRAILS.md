# Agent Guardrails — Anti-Forgetting Enforcement (§11.4.109)

**Revision:** 1
**Last modified:** 2026-07-09T00:00:00Z
**Classification:** consumer instantiation of the universal §11.4.109 mandate. The canonical
preamble + checklist pattern lives in `constitution/docs/AGENT_GUARDRAILS.md`; this file is the
Helix OTA project-level instantiation (§11.4.35) and maps the guardrails to *this* project's
concrete surfaces.

Mandatory constraints MUST NOT depend on an agent remembering them. Two layers make the rules
mechanical (§11.4.109):

1. **`constitution/scripts/hooks/guard-forbidden-commands.sh`** — a Claude Code `PreToolUse`
   hook (inherited by reference, NEVER copied — a copy diverges silently). Wired in
   `.claude/settings.json`. It BLOCKS at the tool-call boundary, no matter what any agent
   remembers: `git push --force|-f|--force-with-lease|--no-verify|--no-gpg-sign` (§11.4.113
   absolute no-force-push), `sudo`/`su` (§11.4.161 rootless), raw host-direct emulator/adb, and
   host power-management (§12 Host-Session Safety). Escape hatch: a command carrying the literal
   `# guardrails:allow <reason>` marker is warned but allowed — EXCEPT host-power, which is never
   overridable.
2. **This document** — the preamble the orchestrator pastes into every subagent dispatch, and the
   checklist the orchestrator runs before every state-changing action.

---

## SUBAGENT CONSTITUTIONAL PREAMBLE

Paste this verbatim into every subagent dispatch (§11.4.20/§11.4.70 subagent-driven default):

> You operate under the Helix Constitution (`constitution/`). Non-negotiable, mechanically
> enforced (§11.4.109):
> - **No bluff (§11.4).** Every claimed result carries real captured physical evidence
>   (§11.4.5/§11.4.69/§11.4.107). Metadata-only / config-only / absence-of-error / grep-without-
>   runtime PASS are forbidden. If you cannot prove it, say `UNKNOWN:` / `PENDING_FORENSICS:`.
> - **No force-push, ever (§11.4.113).** No history rewrite, no `--no-verify`. Integrate onto
>   latest main and fast-forward.
> - **Rootless only (§11.4.161).** No `sudo`/`su`. Containers via the `vasic-digital/containers`
>   submodule (§11.4.76), Podman rootless.
> - **Target + host safety (§11.4.133 / §12).** Never brick the target device or destabilize the
>   host (no suspend/poweroff/logout; ≤60% host memory §12.6).
> - **Stay in scope.** Edit only the files named in your task. Do NOT run `git add/commit/push`
>   unless the orchestrator explicitly delegates it — the orchestrator serializes commits
>   (§11.4.84 working-tree quiescence).
> - **Systematic debugging first (§11.4.102).** No fix without a proven root cause.

---

## ORCHESTRATOR PRE-ACTION CHECKLIST

Run before every state-changing action (commit, push, tag, build, flash, deploy):

1. **Fetch-before-edit (§11.4.37):** `git fetch --all --prune` on parent + owned submodules ran
   this session; integrate any incoming before editing.
2. **Quiescence (§11.4.84):** before `git add`, grep the working tree for mutation residue
   (`MUTATED for paired`, `// always pass`, `_mutated_`); every staged file accounted for vs the
   declared scope; no in-flight paired-mutation gate live.
3. **Every change reviewed (§11.4.142):** an independent reviewer (subagent, not the author) has
   cleared the diff to a zero-finding GO (§11.4.134) before commit/build.
4. **Validate before commit (§11.4.26 step 3 / §11.4.40):** inheritance gate + pre-build sweep +
   regression + meta GREEN on the real tree.
5. **No force-push (§11.4.113):** pushes are fast-forward onto latest main; fan out to all four
   upstreams (§2.1).
6. **Multi-agent utilization (§11.4.183 / §11.4.103):** keep ≥3 non-contending background streams
   busy while the main stream is free; idle only when genuinely blocked (§11.4.94/§11.4.101).
7. **Manual-QA final confirmation (§11.4.185):** a release tag additionally requires the manual
   QA-team confirmation-of-done step.

---

## Helix OTA project-specific surfaces the guardrails protect

- **Containers (§11.4.76/§11.4.161/§11.4.173):** all container + build workloads go through
  `submodules/containers` (rootless Podman); no bare-host or `sudo`-escalated container runs.
- **Target-hardware safety (§11.4.133):** RK3588 / Orange Pi 5 Max flashing + A/B updates use the
  sanctioned tools + integrity-verified images only; no unverified voltage/clock/regulator writes.
- **Trust boundary (project CLAUDE.md):** the artifact-signature verification key comes ONLY from
  server config, never from the request.

Cross-reference: `constitution/docs/AGENT_GUARDRAILS.md` (canonical), `.claude/settings.json` (hook
wiring), `constitution/scripts/hooks/guard-forbidden-commands.sh` (the mechanism).
