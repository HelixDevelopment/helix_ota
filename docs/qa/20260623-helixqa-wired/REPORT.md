# HelixQA Go orchestrator — dependency submodules wired + build PROVEN (2026-06-23)

**Revision:** 1
**Last modified:** 2026-06-23T16:30:00Z
**Operator directive:** add the HelixQA submodule dependencies into the proper location per the constitution (§11.4.28(C)/§11.4.31/§11.4.36) so the Go orchestrator builds. Verdict: **DONE — real build proof.**

## What was added (7 dependency submodules at `submodules/<name>`, §11.4.28(C))

| submodule path | repo | HEAD | content |
|---|---|---|---|
| `submodules/containers` | vasic-digital/containers | e635ad8 | ✓ |
| `submodules/doc_processor` | vasic-digital/DocProcessor | b8c71de | ✓ |
| `submodules/llm_orchestrator` | vasic-digital/LLMOrchestrator | a484f7d | ✓ |
| `submodules/llm_provider` | vasic-digital/LLMProvider | 8a4ca59 | ✓ |
| `submodules/llms_verifier` | vasic-digital/LLMsVerifier | 09bcaec | ✓ (shallow — see note) |
| `submodules/security` | vasic-digital/security | b69c7d9 | ✓ |
| `submodules/vision_engine` | vasic-digital/VisionEngine | 0bf75ee | ✓ |

These match helixqa's `go.mod` `replace` paths exactly (`../<name>` from `submodules/helixqa` → `submodules/<name>`).

## Path-mismatch bridge (§11.4.29 inconsistency)

`submodules/llm_orchestrator/go.mod` references `../LLMProvider` (CamelCase) while helixqa references `../llm_provider` (snake_case) — the same repo at two conventions. Bridged with a consumer-side relative symlink `submodules/LLMProvider → llm_provider` (parent-level, NOT injected into any submodule per §11.4.28(B); not a go.work, to avoid affecting the ota-* builds). The `llmsverifier` module lives at the subpath `submodules/llms_verifier/llm-verifier/` (matching helixqa's `../llms_verifier/llm-verifier` replace).

## Build proof (rock-solid, §11.4.6 — real exit codes)

```
cd submodules/helixqa
go build ./...                     → BUILD_EXIT=0   (clean)
go build -o helixqa ./cmd/helixqa  → CMD_EXIT=0     (binary 29,940,418 bytes — links)
go vet ./pkg/testbank ./pkg/orchestrator → clean
```

The HelixQA Go orchestrator (`cmd/helixqa`: `run|list|report|autonomous|http|...`) now compiles + links against the OTA project's submodule layout. Per the earlier WIRING_PLAN, the OTA bank format already matches; the remaining step to drive it vs the live OTA system is a thin `DeviceExec` host-exec adapter for the `dispatches_to` challenges (separate follow-up).

## Honest notes (§11.4.6)

- **LLMsVerifier clone:** the full GitHub clone fails repeatedly with `fetch-pack` (a deep-history object-transfer error, not LFS, not empty-repo — the repo is 388 KB, pushed today). A `--depth 1` shallow clone succeeds and provides the working tree (1316 files) the build needs. It is registered as a submodule; a future `git submodule absorbgitdirs` + full-history re-fetch (once the upstream pack issue is resolved) is a tracked follow-up.
- **install_upstreams (§11.4.36):** run per added submodule where an `upstreams/` recipe dir is present.
- This wires the orchestrator to BUILD; running a full HelixQA autonomous session vs the live OTA system is the next step (needs the DeviceExec adapter + thinker).
