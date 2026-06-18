# CodeGraph Integration — Helix OTA

| Field | Value |
|---|---|
| **Revision:** | 1 |
| **Last modified:** | 2026-06-19T00:00:00Z |
| **Status:** | active |

## Overview

This project uses [CodeGraph](https://github.com/colbymchenry/codegraph)
(`@colbymchenry/codegraph`) for local semantic code intelligence — a
SQLite-backed knowledge graph that agents query via MCP. Per §11.4.78
(CodeGraph code-intelligence mandate), CodeGraph is the mandatory code-
intelligence tool for every project under this Constitution.

## Installation

CodeGraph is installed globally via npm at `~/.local/bin/codegraph`:

```bash
npm install -g @colbymchenry/codegraph
```

Current version: `1.0.1` (`codegraph --version`).

The binary must be on `PATH` — Claude Code, Cursor, and other AGENT
runtimes resolve it directly (no hardcoded host path per §11.4.78).

## Configuration

Config lives at `.codegraph/config.json` at the project root.

### Own-org submodules — INCLUDED (§11.4.79)

| Submodule | Org |
|---|---|
| `submodules/challenges` | vasic-digital |
| `submodules/helixqa` | HelixDevelopment |
| `submodules/ota-android-agent` | HelixDevelopment |
| `submodules/ota-artifact-validator` | HelixDevelopment |
| `submodules/ota-protocol` | HelixDevelopment |
| `submodules/ota-rollout-engine` | HelixDevelopment |
| `submodules/ota-telemetry-schema` | HelixDevelopment |
| `submodules/ota-update-engine-bridge` | HelixDevelopment |
| `containers` | vasic-digital |
| `constitution` | HelixDevelopment |

All own-org submodules are INCLUDED in the index so agents resolve
cross-references across the entire project (§11.4.79).

### Third-party submodules — EXCLUDED

| Submodule | Reason |
|---|---|
| `submodules/http3` | Third-party, not owned |

### Excluded paths — rationale per §11.4.10

| Pattern | Rationale |
|---|---|
| `.env`, `.env.*` | Credentials per §11.4.10 |
| `scripts/testing/secrets/**` | Credentials per §11.4.10 |
| `secrets/**` | Credentials per §11.4.10 |
| `*.pem`, `*.key` | Credentials per §11.4.10 |
| `*.log` | Runtime logs |
| `qa-results/**` | Raw captured evidence (gitignored per §11.4.30) |
| `docs/qa/stress-chaos-runs/**` | Stress/chaos scratch output |
| `_exports/**`, `docs/**/_exports/**` | Generated document exports (regenerable) |
| `.git-backups/**` | Data-safety backups (§9.2) |
| `.codegraph/cache/**`, `*.db`, `*.db-wal`, etc. | CodeGraph transient data |
| `out/`, `build/`, `target/`, `bin/` | Build artefacts |
| `node_modules/` | Third-party dependencies |
| `__pycache__/`, `*.pyc` | Python cache |

## Re-indexing on submodule updates

After `git submodule update --remote --merge`, re-index:

```bash
scripts/codegraph_setup.sh
```

Or manually:

```bash
codegraph sync .
codegraph status .
```

A full rebuild (cleaner, picks up engine improvements):

```bash
codegraph index -f .
```

## MCP server wiring

CodeGraph exposes an MCP server at `codegraph serve --mcp`. The
following agent configs wire it:

### Claude Code

In `.claude/mcp.json` (project-scoped) or `~/.claude.json` (global):

```json
{
  "mcpServers": {
    "codegraph": {
      "type": "stdio",
      "command": "codegraph",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Or use the installer:

```bash
codegraph install --target claude --location local
```

### Other agents

```bash
codegraph install --target cursor
codegraph install --target opencode
```

## Anti-bluff verification

Run `scripts/codegraph_validate.sh` to verify the index resolves a
symbol from an own-org submodule. The validation:

1. Queries CodeGraph for `Validate` in `submodules/ota-artifact-validator`
2. Asserts the query returns ≥ 1 result from that submodule
3. Reports PASS/FAIL

A FAIL means the index is stale, configured incorrectly, or an
own-org submodule was accidentally excluded.

## Regular updates (§11.4.80)

`scripts/codegraph_setup.sh` handles npm updates, re-indexing, and
validation. Run weekly or after major submodule changes.
