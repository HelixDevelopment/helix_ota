# Signed-Pipeline vs Live — FRESH Re-validation (2026-07-09)

**Revision:** 1
**Last modified:** 2026-07-09T17:15:00Z

Fresh, single-owner (§11.4.119) re-run of the signed OTA pipeline against a
**freshly booted REAL system** (real `ota-server` + real PostgreSQL, rootless
podman). Supersedes the 2026-06-23 UNTRACKED/stale 15/15 evidence per §11.4.108
(no stale evidence — everything below was re-run today from a clean boot).

Host: `the-factory` (user `milos`), podman 5.7.1 rootless, Go 1.26.4, x86_64.
Boot target: `milos@localhost` (the harness's default `thinker.local` was
unreachable — DNS `Name or service not known`; the run was pointed at the local
rootless-podman host over ssh-to-localhost, which the harness supports).

## 1. Boot the real system + /readyz + admin login  — PASS

Command:
```
TARGET=milos@localhost REMOTE_USER=milos bash tests/lib/boot_real_system.sh --up
```
Result (`boot_standalone.log`): cross-compiled a static `linux/amd64` ota-server
(22,618,274 bytes), `podman-compose build` + ordered up (postgres-first, waited
for `pg_isready`, then ota-server), `/readyz -> 200 {"status":"ready"}`.
`BASE_URL=http://127.0.0.1:18080`.

Independent capture (`boot_readyz_login.txt`), against the live loopback:
- `GET /readyz`  -> **HTTP 200** `{"status":"ready"}`
- `GET /healthz` -> **HTTP 200** `{"status":"ok"}`
- `POST /api/v1/auth/login` (admin@helix.system) -> **HTTP 200**, real
  DB-backed `access_token` issued (redacted), `roles:["admin","operator","viewer"]`.
- Containers: `helix-ota-system_postgres_1 Up (healthy) :55480->5432`,
  `helix-ota-system_ota-server_1 Up (healthy) :18080->8080`.

Standalone stack then torn down (`bash boot_real_system.sh --down`) — orphan
check returned `NONE` (§11.4.14).

## 2. tests/e2e/pipeline_signed_live.sh — FRESH 15/15 PASS

Command:
```
TARGET=milos@localhost REMOTE_USER=milos \
EVIDENCE_DIR=docs/qa/20260709-signed-pipeline-revalidate \
bash tests/e2e/pipeline_signed_live.sh
```
Self-booted the real system in CALLER-PUBKEY MODE (live server trusts the
caller's ephemeral ed25519 pubkey; real Postgres), `/readyz -> 200`
(`run.log`), drove the full pipeline, self-tore-down. **PASS=15 FAIL=0 SKIP=0,
exit=0** (`SUMMARY.txt`). Per-step captured evidence (real HTTP responses,
tokens redacted):

| Step | Assertion | HTTP | Evidence file |
|---|---|---|---|
| 01 | live admin login, access_token | 200 | step01_login.txt |
| 02 | anti-bluff: bogus signature REJECTED (SIGNATURE_INVALID) | 422 | step02_antibluff_bogus_sig_reject.txt |
| 03 | upload BASE v1.0.0 (caller-signed) verified=true | 201 | step03_upload_base_signed.txt |
| 04 | upload TARGET v1.1.0 (caller-signed) verified=true | 201 | step04_upload_target_signed.txt |
| 05 | GET target artifact persisted (id echoes) | 200 | step05_get_target_artifact.txt |
| 06 | create BASE release | 201 | step06_release_base.txt |
| 07 | create TARGET release | 201 | step07_release_target.txt |
| 08 | register device on 1.0.0 (device_token issued) | 201 | step08_register_device.txt |
| 09 | deploy TARGET release | 201 | step09_deploy_target.txt |
| 10 | rollout create (echoes deployment_id) | 201 | step10_rollout_create.txt |
| 11 | rollout get | 200 | step11_rollout_get.txt |
| 12 | rollout evaluate -> action=advance | 200 | step12_rollout_evaluate.txt |
| 13 | register base->target delta | 201 | step13_register_delta.txt |
| 14 | **PAYOFF**: device on 1.0.0 polls /client/update, RECEIVES signed 1.1.0 (version, release, sha256 echoes signed TARGET, signature present, delta base_version=1.0.0 sha matches) | 200 | step14_device_poll_receives_signed_update.txt |
| 15 | control: device already on 1.1.0 gets no update | 204 | step15_device_on_target_poll.txt |

Step-14 captured body (real): `version:"1.1.0"`, `sha256:91ae83856bdd…`,
`signature:"OxSx/DrERKBN…VBg=="`, `delta.base_version:"1.0.0"`,
`delta.sha256:7dfb18b3336a…` — the device receives the correct SIGNED update
through the full real-DB pipeline.

Post-run orphan check: `NONE_CLEAN` (§11.4.14 quiescence — no leftover containers).

## 3. Challenges bank dry-run — PASS (20/0/0)

Command:
```
bash tools/helixqa/run_bank.sh --dry-run
```
Result (`challenges_dryrun.log`): **20 passed / 0 failed / 0 skipped
(mode=dry), RESULT: PASS**, exit=0. Honest boundary (§11.4.6): `--dry-run` is a
STATIC coverage audit (bank structure + declared evidence_artifact existence) —
it touches nothing live and is NOT a live challenge execution. A live full-bank
run requires `HELIX_ADMIN_PASSWORD` and each challenge self-boots/tears-down.

## 4. Teardown / cleanup — PASS

Both the standalone boot and the pipeline's self-teardown removed their
project-scoped stacks; post-run `podman ps -a | grep helix-ota-system` returned
none. No source files modified (`git status`: only this new evidence dir added).

## SKIP-with-reason

None. The default remote host `thinker.local` was unreachable, but the run was
NOT skipped — it was pointed at the local rootless-podman host, booted, and
proven live. No fabricated PASS; no metadata-only PASS.

## Evidence files (this run)

- boot_standalone.log, boot_standalone.stdout — standalone boot
- boot_readyz_login.txt — independent /readyz + /healthz + admin login capture
- run.log — pipeline_signed_live full run log (self-boot readyz)
- step01..step15 *.txt — per-step captured HTTP responses (redacted)
- SUMMARY.txt — PASS=15 FAIL=0 SKIP=0 exit=0
- pipeline_console.log — full console of the pipeline run
- challenges_dryrun.log — challenges bank dry-run (20/0/0 PASS)
