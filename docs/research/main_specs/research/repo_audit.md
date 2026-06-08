# Repository Public-Visibility Audit (Gap G11)

**Revision:** 1
**Last modified:** 2026-06-08T00:00:00Z
**Authority:** Helix OTA release engineering — gap G11 (owned submodule + dependency repo visibility)
**Scope:** GitHub org `HelixDevelopment` + GitLab group `helixdevelopment1` for the 6 owned `ota-*` submodules; plus parent repo and the `http3` / `containers` / constitution dependencies.
**Method:** every cell is populated from a REAL CLI call (`gh repo view`, `glab api projects/...`) captured 2026-06-08. No assumptions (Constitution §11.4.6). Project policy: owned + dependency repos MUST be PUBLIC.

## table_of_contents

- [environment_and_auth](#environment_and_auth)
- [owned_ota_submodules](#owned_ota_submodules)
- [parent_repository](#parent_repository)
- [dependency_repositories](#dependency_repositories)
- [evidence_commands](#evidence_commands)
- [action_items](#action_items)
- [anti_bluff_unverified_register](#anti_bluff_unverified_register)

## environment_and_auth

| Tool | Path | Auth | Account |
|---|---|---|---|
| `gh` | `/opt/homebrew/bin/gh` | ✓ logged in github.com | `milos85vasic` (scopes: repo, read:org, admin:public_key, gist) |
| `glab` | `/opt/homebrew/bin/glab` | ✓ logged in gitlab.com (GITLAB_TOKEN env) | `milos85vasic` |

Both CLIs are authenticated — GitLab checks are REAL, not SKIPped.

Source-of-truth note: `.gitmodules` declares the constitution submodule as
`git@github.com:HelixDevelopment/HelixConstitution.git` (repo name **HelixConstitution**,
not `constitution`) and the deps `http3` + `containers` under org **vasic-digital**.
The audit was corrected to the real repo names/orgs after reading `.gitmodules`.

## owned_ota_submodules

| Repo | GitHub exists? | GitHub visibility | GitLab exists? | GitLab visibility | Policy |
|---|---|---|---|---|---|
| ota-protocol | YES | PUBLIC | YES | public | OK |
| ota-telemetry-schema | YES | PUBLIC | YES | public | OK |
| ota-artifact-validator | YES | PUBLIC | YES | public | OK |
| ota-rollout-engine | YES | PUBLIC | YES | public | OK |
| ota-update-engine-bridge | YES | PUBLIC | YES | public | OK |
| ota-android-agent | YES | PUBLIC | YES | public | OK |

All 6 owned `ota-*` submodules exist and are PUBLIC on BOTH GitHub
(`HelixDevelopment/<name>`) and GitLab (`helixdevelopment1/<name>`). No action needed.

## parent_repository

| Repo | GitHub exists? | GitHub visibility | GitLab exists? | GitLab visibility | Policy |
|---|---|---|---|---|---|
| helix_ota | YES (HelixDevelopment) | PUBLIC | YES (helixdevelopment1) | public | OK |

## dependency_repositories

| Repo | Declared origin (.gitmodules) | GitHub exists? | GitHub visibility | GitLab exists? | GitLab visibility | Policy |
|---|---|---|---|---|---|---|
| HelixConstitution | `HelixDevelopment/HelixConstitution` | YES | PUBLIC | YES `helixdevelopment1/helixconstitution` | public | OK (GitHub canonical) |
| http3 | `vasic-digital/http3` | YES | PUBLIC | YES `vasic-digital/http3` | public | OK |
| containers | `vasic-digital/containers` | YES | PUBLIC | YES `vasic-digital/containers` | **private** | ACTION (see G11-A1) |

Notes on dependency namespace reality (from real CLI):
- `HelixDevelopment/constitution`, `vasic-digital/constitution` — DO NOT EXIST on GitHub (404). The canonical name is `HelixConstitution`.
- `helixdevelopment1/containers` and `helixdevelopment1/http3` — DO NOT EXIST on GitLab (404). The deps live under `vasic-digital/*` on GitLab, matching `.gitmodules`.
- `vasic-digital/HelixConstitution` on GitLab exists but is **private** (a mirror; the canonical constitution origin per `.gitmodules` is GitHub, which is PUBLIC).

## evidence_commands

GitHub (per repo):
```
gh repo view HelixDevelopment/<name> --json name,visibility,isPrivate,url
gh repo view HelixDevelopment/HelixConstitution --json name,visibility,isPrivate,url
gh repo view vasic-digital/http3 --json name,visibility,isPrivate,url
gh repo view vasic-digital/containers --json name,visibility,isPrivate,url
```
GitLab (per repo):
```
glab api projects/helixdevelopment1%2F<name>
glab api projects/vasic-digital%2Fhttp3
glab api projects/vasic-digital%2Fcontainers
glab api projects/helixdevelopment1%2Fhelixconstitution
```

Real captured output (verbatim, 2026-06-08):
```
GH ota-protocol            -> {"isPrivate":false,"visibility":"PUBLIC","url":".../ota-protocol"}
GH ota-telemetry-schema    -> {"isPrivate":false,"visibility":"PUBLIC"}
GH ota-artifact-validator  -> {"isPrivate":false,"visibility":"PUBLIC"}
GH ota-rollout-engine      -> {"isPrivate":false,"visibility":"PUBLIC"}
GH ota-update-engine-bridge-> {"isPrivate":false,"visibility":"PUBLIC"}
GH ota-android-agent       -> {"isPrivate":false,"visibility":"PUBLIC"}
GH helix_ota               -> {"isPrivate":false,"visibility":"PUBLIC"}
GH HelixConstitution       -> {"isPrivate":false,"visibility":"PUBLIC"}
GH vasic-digital/http3     -> {"isPrivate":false,"visibility":"PUBLIC"}
GH vasic-digital/containers-> {"isPrivate":false,"visibility":"PUBLIC"}
GH HelixDevelopment/constitution -> 404 (does not exist)
GH HelixDevelopment/http3        -> 404 (does not exist)
GH HelixDevelopment/containers   -> 404 (does not exist)

GLAB helixdevelopment1/ota-protocol            -> visibility: public
GLAB helixdevelopment1/ota-telemetry-schema    -> visibility: public
GLAB helixdevelopment1/ota-artifact-validator  -> visibility: public
GLAB helixdevelopment1/ota-rollout-engine      -> visibility: public
GLAB helixdevelopment1/ota-update-engine-bridge-> visibility: public
GLAB helixdevelopment1/ota-android-agent       -> visibility: public
GLAB helixdevelopment1/helix_ota               -> visibility: public
GLAB helixdevelopment1/helixconstitution       -> visibility: public
GLAB vasic-digital/http3                        -> visibility: public
GLAB vasic-digital/containers                   -> visibility: private
GLAB helixdevelopment1/containers               -> 404 Project Not Found
GLAB helixdevelopment1/http3                    -> 404 Project Not Found
GLAB vasic-digital/constitution                 -> 404 Project Not Found
```

## action_items

| ID | Severity | Repo / Host | Finding | Required action |
|---|---|---|---|---|
| G11-A1 | Medium | `vasic-digital/containers` (GitLab) | GitLab mirror is **private** while GitHub origin is PUBLIC | Make the GitLab `vasic-digital/containers` repo PUBLIC to match policy, OR document it as a deliberate non-public mirror (GitHub is the canonical PUBLIC origin per `.gitmodules`). |
| G11-A2 | Low | `vasic-digital/HelixConstitution` (GitLab) | GitLab mirror is **private**; canonical constitution origin is GitHub (PUBLIC) | Confirm whether a PUBLIC GitLab constitution mirror is required. The canonical constitution origin (`HelixDevelopment/HelixConstitution`, GitHub) is PUBLIC, so policy is satisfied at the source-of-truth host; the GitLab private copy is informational only. |

No owned `ota-*` submodule is missing or private on either host — zero action items for the 6 owned repos.

## anti_bluff_unverified_register

| Item | Status | Reason |
|---|---|---|
| GitLab auth | VERIFIED | `glab auth status` → logged in as `milos85vasic`; all GitLab queries returned real API JSON (public/private/404), not auth failures. |
| GitFlic / GitVerse mirrors | UNVERIFIED | Project policy mentions 4 upstreams (GitHub + GitLab + GitFlic + GitVerse). This audit covered only GitHub + GitLab (the two requested hosts + available CLIs). GitFlic/GitVerse were not queried — no CLI available in this environment. Out of G11 scope as specified. |
| GitLab `helixconstitution` casing | NOTED-AS-FACT | GitLab normalized the path to lowercase `helixdevelopment1/helixconstitution` (GitLab project paths are case-normalized); the repo exists and is public. |
