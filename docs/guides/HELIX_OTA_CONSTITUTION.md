# Helix OTA Constitution

This constitution **extends** the Helix Universal Constitution at
`constitution/Constitution.md`. All clauses there apply unless
explicitly overridden below with an explicit `Override §X.Y` section.

## Project Articles

### §101. Native-A/B safety floor

The Android update path MUST preserve native A/B + AVB/dm-verity +
automatic rollback semantics. The control plane MAY orchestrate, gate,
and observe updates but MUST NOT bypass the device's slot-verification
and auto-rollback guarantees.

### §102. Signature trust boundary

The artifact-signature verification key is sourced EXCLUSIVELY from
server configuration. A request-supplied verification key is NEVER
trusted (it would defeat signature verification). See
`server/internal/api/handlers_artifact.go`.

### §103. Reuse-first, decouple-hard

Capabilities are built as independently reusable submodule bricks with
their own tests + docs, decoupled enough to be reused by future
projects. New bricks get PUBLIC repos under HelixDevelopment / vasic-digital.

### §104. Forward-OS roadmap preserved

Planning leaves explicit room for post-Android expansion (Linux, then
Windows, then other OSes and their flavors) under versioned
`1.X.X-<name>` directories; multi-standard support per OS is mandatory
where multiple standards exist.

---

## Overrides of Universal Constitution

(none — this project overrides no universal clause)

---

## Owned-submodule set

(per Universal §4 — submodules this project owns and tags)

```
submodules/ota-protocol
submodules/ota-telemetry-schema
submodules/ota-artifact-validator
submodules/ota-rollout-engine
submodules/ota-update-engine-bridge
submodules/ota-android-agent
```

`constitution/` (HelixConstitution) is consumed, not owned-for-edit here.
`containers/` is a shared vasic-digital brick consumed by this project.

---

## Project-specific remotes

| Repo | Remotes |
|---|---|
| Main (`helix_ota`) | github, gitlab, gitflic, gitverse (origin fans out to all four) |
| Owned `ota-*` submodules | github + gitlab (PUBLIC), GitFlic/GitVerse mirrors per §4 |

---
