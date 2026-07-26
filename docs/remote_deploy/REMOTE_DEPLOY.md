# HelixOTA Remote Deployment — Scaffold, Wiring & Operator Decisions (ATM-755)

**Revision:** 1
**Last modified:** 2026-07-14T00:00:00Z
**Status:** active — HOST-SIDE SCAFFOLD ONLY (no live deploy performed)
**Classification:** project-specific (§11.4.17) — consumer-layer remote-deploy
scaffolding for the HelixOTA control plane serving the Svord product line.
**Grounding (§11.4.6):** every claim below is either verified locally (parse
gates, `podman-compose config`, a real rootless `podman build`, redacted dry-runs)
or explicitly marked `UNCONFIRMED:` / `QD` (operator decision). Variable NAMES
only (§11.4.10) — no credential value appears in this doc or any script output.

---

## 1. What this is

The §11.4.18 companion doc for the HelixOTA remote-deployment + stack-lifecycle
scripts scaffolded under `scripts/remote_deploy/` + the consumer stack under
`deploy/svord/`. It deploys the HelixOTA control plane (Go API + PostgreSQL +
MinIO), the operator console/dashboards, and the marketing website to the remote
host (`hxota` @ `$ADDRESS_HXOTA:$SSH_PORT_HXOTA`), all over SSH + **rootless
podman compose** (§11.4.161), with HTTPS via the `lets_encrypt` submodule and
release-artifact upload via the `sftp` submodule.

**Nothing has been deployed to the live server.** This is a host-side scaffold,
validated locally; the real deploy is operator-gated (QD sequencing below).

## 2. Architecture

```
 operator host                                   remote deploy host (hxota)
 ─────────────                                    ──────────────────────────
 scripts/remote_deploy/*.sh  ── ssh ──▶  ~/hxota-stack/                (rootless
   deploy.sh (orchestrator)                 compose.svord.yml           podman)
   deploy_infrastructure.sh                 stack.env  (chmod 600)
   deploy_api.sh (x-compile ota-server)     nginx/hxota-proxy.conf
   deploy_dashboards.sh (build SPAs)        certs/<domain>/{fullchain,privkey}
   deploy_website.sh   (build website)      srv/{website,console,dashboard,acme}
   https_certs.sh  (lets_encrypt submodule) server/  (rsynced build context)
   publish_artifacts.sh (sftp submodule)         │
   {start,stop,restart,status,test}.sh           ▼
                                          ┌─ podman compose (rootless) ─┐
                                          │ postgres  minio  ota-server │
                                          │            proxy (nginx)    │
                                          └──────────────┬──────────────┘
                                             hxota.dev ──┤ console + /api/v1
                                             hxota.com ──┘ website
```

- **Decoupling (§11.4.28):** the stack + scripts are **project-agnostic**. No
  ATMOSphere/Mistiq literal is baked in; the connection creds live in a
  gitignored deploy env (config-injected, resolved — never a hardcoded path), and
  products are DATA (`deploy/svord/products/*.env`).
- **Rootless (§11.4.161):** no `privileged`, no host networking, no root user,
  fully-qualified images, bounded `mem_limit`. The systemd units are **user**
  units under `hxota` (no sudo/root).
- **Anti-bluff (§11.4.6/§11.4.13):** `--dry-run` prints every action WITHOUT
  connecting; the honest health probe (`hx_remote_health_confirm`) reports
  `PROXY_HEALTHY`/`API_READY` only on a real 200 — never a faked pass.

## 3. Script tree (all §11.4.18-documented, sh -n + bash -n clean §11.4.67)

| Script | Role |
|---|---|
| `lib/common.sh` | Shared, project-agnostic library: deploy-env resolution, credential-safe logging (§11.4.10), SSH/rsync/SFTP helpers with real DRY-RUN, rootless-compose helpers, multi-product resolution, remote stack.env writer (fail-closed on missing runtime secrets). |
| `test.sh` | Local validation harness (this scaffold's anti-bluff evidence): parse gates + `compose config` + deploy-env resolution + submodule availability. No live host. |
| `start.sh` / `stop.sh` / `restart.sh` / `status.sh` | Operate the stack (remote by default; `--local` for a local stack). Volumes persist across `stop`. |
| `deploy.sh` | **Main** orchestrator: preflight → infra → API → dashboards → website → certs → bring-up → LIVE health confirm → report. Deploys ALL + website. |
| `deploy_infrastructure.sh` | Prepare remote bundle, write `stack.env`, bring up postgres + minio. |
| `deploy_api.sh` | Cross-compile the static `ota-server`, rsync the build context, build + bring up the API container. |
| `deploy_dashboards.sh` | Build the ota-manager console + dashboard SPAs, stage into `srv/{console,dashboard}`, reload proxy. |
| `deploy_website.sh` | Build the Angular website, stage into `srv/website`, reload proxy. |
| `https_certs.sh` | `issue`/`renew`/`rotate` TLS certs via the `lets_encrypt` submodule; rsync certs to the remote; run `LE_RELOAD_CMD`. |
| `publish_artifacts.sh` | Upload the flashable image + OTA archives + hash files to the protected download area via the `sftp` submodule. |

Consumer stack: `deploy/svord/compose.svord.yml` (+ `nginx/hxota-proxy.conf`,
`products/*.env.example`, `systemd/*`, `srv/` staging, `certs/`).

## 4. Env contract (config injection, §11.4.28 / §11.4.10)

The gitignored deploy env (`scripts/testing/secrets/.hxota_deploy.env`, resolved
via `HXOTA_DEPLOY_ENV` → cwd default → `$HOME/.config/hxota/deploy.env`) carries:
connection (`ADDRESS_HXOTA`, `SSH_PORT_HXOTA`, `HXOTA_DEPLOY_USER`,
`HXOTA_DEPLOY_HOME`, `HXOTA_DEPLOY_PASSWORD`), bootstrap root (`HXOTA_ROOT_*`),
SFTP repo (`HXOTA_SFTP_REPO`), and the **runtime secrets** the stack needs
(`HXOTA_PG_PASSWORD`, `HXOTA_MINIO_USER/PASSWORD`, `HXOTA_TOKEN_SECRET`,
`HXOTA_ARTIFACT_PUBKEY`). Template: `scripts/remote_deploy/.hxota_deploy.env.example`
(placeholders only). **Only the runtime secrets are shipped to the remote
`stack.env`; the ssh/root passwords are NEVER shipped.**

## 5. Submodule-incorporation design (§11.4.28(C) — NOT wired to .gitmodules yet)

Per the task, `.gitmodules` pointers are **deferred** to the operator. The plan:

### 5.1 Containers submodule (already present)
`containers` is already a repo-root flat submodule (`helix-deps.yaml`,
`.gitmodules`). The scripts use the **same rootless-podman substrate** it wraps
(§11.4.161). A deeper integration path exists — the Go `containers/cmd/deploy-stack`
+ `pkg/{boot,compose,health,remote,remoteexec,distribution}` — which the shell
scripts here intentionally do NOT depend on (they follow the proven
`distribute_stack.sh` rootless-`podman-compose`-over-ssh pattern for portability
and zero-build operation). **Future option:** re-implement `deploy.sh` over
`containers/cmd/deploy-stack` for typed orchestration + health + leasing; tracked
as a follow-up, not required for this scaffold.

### 5.2 lets_encrypt submodule (LIVE upstream, NOT on disk)
- **Repo:** `git@github.com:vasic-digital/lets_encrypt.git`.
- **Canonical path (proposed, §11.4.28(C) flat-vs-grouped):**
  `submodules/lets_encrypt/` (grouped, alongside the other `ota-*` bricks) OR
  repo-root `lets_encrypt/` (flat). **QD** — pick one; recommend grouped.
- **Incorporation:** `incorporate-submodule git@github.com:vasic-digital/lets_encrypt.git`
  (§11.4.31/§11.4.36) OR `git submodule add … submodules/lets_encrypt`, then
  `install_upstreams` from its root (§11.4.36).
- **Wiring:** set `LETS_ENCRYPT_HOME=<path>` + `HXOTA_LE_ENTRYPOINT=<real CLI>`
  (`https_certs.sh` invokes it). **UNCONFIRMED (§11.4.6):** the exact CLI
  (`lets_encrypt.sh <action> --domain … --email … --out …`) is a placeholder —
  confirm against the submodule README before live use (§11.4.99). `helix-deps.yaml`
  gains a `lets_encrypt` dep row; **not added yet** (deferred).

### 5.3 sftp submodule (NOT on disk)
- **Repo:** `git@github.com:vasic-digital/sftp.git` (== `HXOTA_SFTP_REPO`).
- **Canonical path (proposed):** `submodules/sftp/` (grouped) — **QD**.
- **Wiring:** `HXOTA_SFTP_HOME=<path>` + `HXOTA_SFTP_ENTRYPOINT=<real CLI>`
  (`publish_artifacts.sh` invokes it, else falls back to plain `sftp` for the
  designed shape). **UNCONFIRMED (§11.4.6):** the exact CLI is a placeholder —
  confirm vs README. `helix-deps.yaml` gains an `sftp` dep row; **not added yet**.

All three follow §11.4.28(C): reached from the parent root, NO nested own-org
chains, decoupled + reusable, entrypoint config-injected (never a hardcoded reach).

## 6. Operator decisions (QD) blocking the real deploy

| ID | Decision | Detail / recommendation |
|---|---|---|
| **QD1** | **Root-password change / host bootstrap** | The deploy env carries `HXOTA_ROOT_PASSWORD` + `HXOTA_ROOT_PASSWORD_OLD`. Is the root pw to be rotated on first bootstrap, and who owns the one-time host prep (create the `hxota` user, install rootless podman + podman-compose, `loginctl enable-linger hxota`, set `net.ipv4.ip_unprivileged_port_start`)? The deploy pipeline itself NEVER uses root (§11.4.161). |
| **QD2** | **SSH key vs password auth** | Currently password creds are present. **Recommend SSH key auth** (`HXOTA_SSH_USE_PASSWORD=0` + `HXOTA_SSH_KEY`): passwordless, no `sshpass`, no secret on any process list. Password mode passes the secret via `SSHPASS` (never argv) but is second-best. Decide + provision the key. |
| **QD3** | **Release repo / artifact source + SFTP target** | Where do the flashable image + OTA archives come from (which build output / release repo), and what is the protected `HXOTA_ARTIFACT_DEST` download area the dashboards expose? Confirm `HXOTA_SFTP_REPO` + the sftp submodule CLI (§5.3). |
| **QD4** | **Svord product scope** | Products served: `atmosphere` + `mistiq_vader`. `atmosphere` target policy is known (`os_type=android`, `board=rk3588_t`); **the Mistiq/VADER board id + OS type are UNKNOWN** (placeholders in `products/mistiq_vader.env.example`) — the product owner must supply them (do NOT guess, §11.4.6). |
| **QD5** | **hxota.dev root behaviour** | "hxota.dev serves the console + as a regular site redirects to hxota.com" — default here serves the console SPA at `/`; a commented 302-redirect block in `nginx/hxota-proxy.conf` implements the redirect. Pick one. |
| **QD6** | **Runtime-secret provisioning** | Where do `HXOTA_PG_PASSWORD` / `HXOTA_TOKEN_SECRET` / `HXOTA_MINIO_*` live — added to the deploy env by the operator (recommended, `openssl rand -base64 48`) or generated-and-persisted on first deploy? The pipeline is **fail-closed** (§11.4.6): it refuses to bring up a stack with any mandatory runtime secret unset — never a silent weak default. |
| aux | **Rootless 80/443 binding** | Binding privileged ports rootless needs the host sysctl `net.ipv4.ip_unprivileged_port_start=80` (one-time host config, NOT sudo-at-runtime) OR a host-level proxy in front. Default ports are 8080/8443. |
| aux | **.gitmodules pointers** | Deferred — the operator adds the `lets_encrypt` + `sftp` submodule pointers (+ `helix-deps.yaml` rows) when ready (§5). |

## 7. Local validation evidence (anti-bluff, §11.4.6 — no live deploy)

Captured on 2026-07-14 (rootless, this host):

1. **Parse gates (§11.4.67):** `bash -n` + `sh -n` on all **13** scripts →
   **13/13 OK, 0 fail** (POSIX-syntactic; passes both shells).
2. **Compose validation:** `podman-compose -f deploy/svord/compose.svord.yml
   config` → **exit 0**, 4 services parsed (`minio`, `ota-server`, `postgres`,
   `proxy`).
3. **Real rootless container build:** `podman build -f server/Dockerfile
   -t hxota/ota-server:scaffold-smoke server/` (with a clearly-labelled STUB
   binary staged at `server/.docker-bin/ota-server`; the real cross-compile is
   `deploy_api.sh`'s job) → **BUILD_EXIT=0**, image `localhost/hxota/ota-server:
   scaffold-smoke` (12 MB), all 9 steps green. Stub + image cleaned up after.
4. **`test.sh` harness:** **PASS=30 FAIL=0 SKIP=2** (lets_encrypt + sftp honestly
   SKIP — designed, not yet live).
5. **SSH-command DRY-RUN:** `deploy.sh --dry-run` printed the full 6-stage
   pipeline WITHOUT connecting; every ssh/rsync target redacted to
   `<HXOTA_DEPLOY_USER>@<ADDRESS_HXOTA>:<SSH_PORT_HXOTA>` — **no host/IP, port, or
   password leaked (§11.4.10)**; passwords never printed anywhere. Component,
   lifecycle, and `publish_artifacts`/`https_certs` dry-runs likewise clean.

## 8. Honest boundaries (§11.4.6)

- **No live server was contacted.** All evidence is host-side (parse, compose
  config, local container build, redacted dry-runs). The real deploy + a live
  health-confirmed run remain operator-gated (QD sequencing).
- **lets_encrypt + sftp CLIs are UNCONFIRMED** until the submodules are
  incorporated and their READMEs read (§11.4.99). The wrappers config-inject the
  entrypoint and honestly SKIP until then — they never fabricate a cert or an
  upload.
- **The ota-server image build used a STUB binary** to prove image assembly; the
  real static binary is cross-compiled by `deploy_api.sh` on a host where the
  §11.4.28 sibling `replace` directives resolve (server/Dockerfile header).
- **Mistiq/VADER target policy is a placeholder** (QD4) — not invented.
