# Helix OTA — Cuttlefish Rootful-Privileged Container: §11.4.161 Documented Exception

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-22T00:00:00Z |
| Status | documented-exception recorded — narrowest-scope rootful-privileged Cuttlefish launch; image build + artifact fetch remain rootless |
| Status summary | The §11.4.161 (rootless-container-runtime mandate) **documented exception** for the Cuttlefish (`cvd`) virtual-Android Tier-2 OTA validation path. Rootless Podman is **NOT viable** for Cuttlefish: the official `google/android-cuttlefish` container README mandates `--privileged` (kvm + vsock + network), and a rootless container **cannot** create the `cvd-ebr` bridge / `cvd-wtap`/`cvd-mtap` tap devices nor write the `/proc/sys/net` knobs Cuttlefish needs (these require `CAP_NET_ADMIN` in a root network namespace; rootless uses `slirp4netns`/`pasta`). This is classified per §11.4.112 as a **host-gated documented exception, NOT a structural impossibility** — Cuttlefish IS possible, it merely needs container privilege. Every load-bearing claim is sourced (see `## Sources verified 2026-06-22`); uncertain claims are marked `UNCONFIRMED:` per §11.4.6. |
| Authority | Helix OTA control-plane / device-integration team |
| Related | `docs/design/CUTTLEFISH_TIER2.md` (the Tier-2 bring-up recipe this exception authorises); `docs/design/EMULATED_DEVICE_TESTING.md` (Tier-1/2/3 overview); `containers/` submodule (`vasic-digital/containers`, §11.4.76 / §11.4.161) — proposed `pkg/cuttlefish` extension |

## 1. Purpose + scope of this exception

§11.4.161 (rootless container runtime mandate) requires every containerized workload
to run under **Podman in rootless mode**; Docker-rootful, `sudo`, or any escalation to
root for container management is **FORBIDDEN unless the target platform has no rootless
option AND that constraint is documented per §11.4.112.**

This document IS that documented constraint, for exactly one workload: the **launch of
the Cuttlefish (`cvd`) virtual Android device inside a container** for Tier-2 OTA
validation (real `update_engine` + AVB/dm-verity + auto-rollback — the path
`docs/design/CUTTLEFISH_TIER2.md` details). It establishes as cited FACT that rootless
Podman cannot host Cuttlefish, records the minimal-privilege rootful recipe actually
used, the residual risk + reversibility, and the strict scope-narrowing (only the
`launch_cvd` run is privileged; the container-image build and the AOSP artifact fetch
stay rootless).

## 2. Classification (§11.4.112): documented exception, NOT structural-impossibility

Per §11.4.112 the correct classification is a **host-gated / documented exception**, NOT
`structurally-impossible` won't-fix:

- **Cuttlefish IS possible** — it boots and applies real OTAs on a Linux + KVM host. The
  only requirement that conflicts with §11.4.161 is **container privilege** (`--privileged`
  + the device passthrough + host networking). This is a *capability requirement*, not a
  platform/protocol impossibility.
- Contrast with a true §11.4.112 structural-impossibility case (e.g. relocating a
  `FLAG_SECURE` secure surface to a second display, which the platform blanks by design):
  there the goal cannot be achieved on the platform at all. Here the goal IS achievable —
  it simply requires the privilege the rootless mandate normally forbids, so it is recorded
  as a **§11.4.161 documented exception** rather than a won't-fix.
- A future change MAY narrow or remove the exception (see §6) — the classification is
  durable but not eternal; a reopen needs NEW evidence rootless Cuttlefish became viable
  (§11.4.7 / §11.4.34), never a re-derivation of the same constraint.

## 3. Why rootless Podman cannot host Cuttlefish (cited FACT)

The official Cuttlefish container path is privileged BY DESIGN:

- The `google/android-cuttlefish` `container/README.md` documents the canonical run as a
  **`--privileged`** container with KVM, vsock, and host networking — Cuttlefish inside a
  container is run with `--privileged` and the host's `/dev/kvm`, `/dev/vhost-vsock`,
  `/dev/vhost-net`, `/dev/vsock`, and `/dev/net/tun` passed through [SRC-CF-CONTAINER].
- **Networking needs a root network namespace.** Cuttlefish creates a bridge (`cvd-ebr`)
  and per-instance tap devices (`cvd-wtap-NN` / `cvd-mtap-NN`) and writes `/proc/sys/net/*`
  knobs to wire guest networking; these operations require **`CAP_NET_ADMIN` in a root
  (initial) network namespace** [SRC-CF-GET-STARTED] [SRC-CF-ONPREM]. A **rootless**
  container has no such namespace — rootless Podman provides userspace networking via
  `slirp4netns`/`pasta`, which **cannot** create host bridges/tap devices nor write the
  `/proc/sys/net` knobs [SRC-PODMAN-PRIV] [SRC-ROOTLESS-NET] [SRC-NONPRIV-POD]. The
  bridge/tap creation therefore fails under rootless, and Cuttlefish guest networking does
  not come up.
- **Device + KVM access.** `/dev/kvm` + the `vhost_vsock`/`vhost_net` device nodes must be
  accessible to the guest VMM; the documented path grants this via `--privileged` device
  passthrough rather than ad-hoc per-device rootless mapping [SRC-CF-CONTAINER].

Net FACT: the project's deep research (§11.4.150, accessed 2026-06-22) found **no**
rootless Podman recipe that brings up Cuttlefish networking — the upstream-mandated
`--privileged` path is the only documented, working one. Per §11.4.161 this is the
"no rootless option" condition, and this document is the required §11.4.112 record.

## 4. The minimal-privilege rootful recipe actually used (FACT)

Only the **launch step** is privileged; everything narrower is preferred where it works.

```bash
# Rootful Podman, narrowest device + network grant that Cuttlefish requires.
# (run as the privileged launch step ONLY — see §5 scope narrowing)
podman run --rm \
  --privileged \
  --network host \
  --device /dev/kvm \
  --device /dev/vhost-vsock \
  --device /dev/vhost-net \
  --device /dev/vsock \
  --device /dev/net/tun \
  <cuttlefish-image> \
  /bin/bash -lc 'HOME=$PWD ./bin/launch_cvd --daemon && ./bin/adb devices'
```

Host preconditions (FACT):

- **Kernel modules** `vhost_vsock` + `vhost_net` loaded on the host (or built-in). On the
  current candidate host `nezha`, `lsmod` does **not** list them, which is consistent with
  them being **built-in** to the host's Linux 6.12 kernel — `UNCONFIRMED:` whether built-in
  vs not-yet-loaded; verify at bring-up with `modinfo vhost_vsock` / checking
  `/sys/module/vhost_vsock` before the first privileged launch (do not guess).
- **Host groups** `kvm`, `cvdnetwork`, `render` for the launching user [SRC-CF-GET-STARTED].
- **Device nodes.** On `nezha` (FACT, host probe 2026-06-22): `/dev/kvm`,
  `/dev/vhost-vsock`, `/dev/vhost-net`, `/dev/net/tun` are present and world-rw;
  **`/dev/vsock` is ABSENT**. The `--device /dev/vsock` line above is therefore conditional
  — include it only if the node exists at launch time; `UNCONFIRMED:` whether Cuttlefish on
  this kernel needs the `/dev/vsock` client node in addition to `/dev/vhost-vsock`, verify
  on first bring-up rather than assume.
- **Guest target.** Tier-2 fetches the public-CI target **`aosp_cf_x86_64_only_phone-userdebug`**
  from `ci.android.com` — **no credentials required** [SRC-CF-GET-STARTED]. The
  `update_engine` README confirms "Cuttlefish works as well" for the real A/B
  `update_engine` apply [SRC-UE-SEARCH].

## 5. Scope narrowing — only the launch is privileged

The exception is the **narrowest** possible. The Tier-2 pipeline has three phases; only
the third is privileged:

1. **Container-image build — ROOTLESS.** Building the Cuttlefish container image (Debian
   host packages per `docs/design/CUTTLEFISH_TIER2.md` §4.1) is an ordinary rootless
   Podman build — no privilege, no exception.
2. **AOSP artifact fetch — ROOTLESS, no credentials.** Downloading the device-image zip +
   `cvd-host_package.tar.gz` for `aosp_cf_x86_64_only_phone-userdebug` from the public
   `ci.android.com` is a plain rootless network fetch — no privilege, no secrets
   [SRC-CF-GET-STARTED].
3. **`launch_cvd` run — ROOTFUL `--privileged` (THIS exception).** Only the actual boot of
   the virtual device (which creates the `cvd-ebr` bridge + tap devices and opens
   `/dev/kvm` + vhost nodes) requires the privileged rootful container of §4.

Everything that CAN be rootless STAYS rootless; the privileged surface is confined to the
single `launch_cvd`/`stop_cvd` lifecycle. This is the §11.4.161 "documented + narrowest"
posture.

## 6. Residual risk + reversibility

- **Residual risk.** A `--privileged --network host` container has broad host access for
  the duration of the launch (device nodes + host network namespace). Mitigations: it runs
  **only** the `launch_cvd`/`stop_cvd` lifecycle on a dedicated Linux test host (`nezha`),
  not on the operator's primary workstation; it carries no project secrets (§11.4.10 — the
  AOSP fetch is credential-free); and the privileged window is the launch only (§5), torn
  down with `stop_cvd` + container removal.
- **Reversibility (FACT).** The exception leaves **no persistent host mutation beyond
  reversible config**: the Cuttlefish host packages are `apt`-installed (removable via
  `apt remove cuttlefish-base cuttlefish-user`), the only host-state changes are the loaded
  kernel modules (`vhost_vsock`/`vhost_net`, transient) and the user's group membership
  (`kvm`/`cvdnetwork`/`render`, reversible via `gpasswd -d`). No bootloader, partition, or
  firmware change occurs on the host — the A/B slot-switch + corrupt-slot-rollback happen
  **inside the virtual device**, never on the host (composes §11.4.133 target-safety: the
  "target" is the Cuttlefish guest, not the host).
- **Containers-submodule routing (§11.4.76 / §11.4.161).** The privileged launch MUST be
  driven through the `vasic-digital/containers` submodule's lifecycle primitives — the
  proposed `pkg/cuttlefish` extension wrapping `launch_cvd`/`stop_cvd` + `adb devices`
  readiness — NOT ad-hoc `podman run` outside `pkg/boot`/`pkg/compose`/`pkg/health`. The
  `--privileged` flag is requested through the submodule's run options, keeping the
  exception inside the sanctioned orchestration layer; a missing capability is extended
  upstream per §11.4.74, never worked around in-project.

## 7. Honest boundary (§11.4.6)

- This document authorises the **container privilege** Cuttlefish needs; it does NOT claim
  Tier-2 has run. The Tier-2 bring-up + apply recipe and its still-open `UNCONFIRMED:`
  items (exact Virtual-A/B-vs-legacy on the chosen image; the exact corrupt-slot mechanism)
  live in `docs/design/CUTTLEFISH_TIER2.md` and remain open there.
- The exception is **operator-gated for first use**: the first privileged `launch_cvd`
  requires operator authorisation to run a rootful `--privileged` container on the test
  host (high-blast-radius capability grant, §11.4.101 / §11.4.21). Until then the path is
  `designed + authorised-in-doc, pending operator privileged-launch authorization` — not a
  green, not `Operator-blocked` beyond that one authorisation.

## Sources verified 2026-06-22

- [SRC-CF-CONTAINER] `google/android-cuttlefish` — `container/README.md`, GitHub —
  https://github.com/google/android-cuttlefish (Cuttlefish-in-a-container run uses
  `--privileged` with host `/dev/kvm`, `/dev/vhost-vsock`, `/dev/vhost-net`, `/dev/vsock`,
  `/dev/net/tun` passthrough + host networking) — accessed 2026-06-22.
- [SRC-CF-GET-STARTED] Get started — Cuttlefish, Android Open Source Project —
  https://source.android.com/docs/devices/cuttlefish/get-started (host KVM + group
  membership `kvm`/`cvdnetwork`/`render`; `ci.android.com` image fetch; public
  `aosp_cf_x86_64_only_phone-userdebug` target, no credentials) — accessed 2026-06-22.
- [SRC-CF-ONPREM] On-premises Cuttlefish, AOSP —
  https://source.android.com/docs/devices/cuttlefish/on-premises (host networking /
  `cvd-ebr` bridge + tap-device requirements for multi-instance Cuttlefish networking) —
  accessed 2026-06-22.
- [SRC-PODMAN-PRIV] Podman `run --privileged` docs — privileged containers grant the
  device + capability access that rootless containers lack; rootless cannot create host
  bridges/tap devices — accessed 2026-06-22.
- [SRC-ROOTLESS-NET] Rootless container networking (oneuptime / Podman rootless networking
  writeup) — rootless uses `slirp4netns`/`pasta` userspace networking and cannot perform
  `CAP_NET_ADMIN` host-namespace operations (bridge/tap creation, `/proc/sys/net` writes) —
  accessed 2026-06-22.
- [SRC-NONPRIV-POD] Non-privileged Cuttlefish-pod community writeup (Medium) — documents
  why a non-privileged pod cannot create the Cuttlefish bridge/tap network and the
  privileged path it required instead — accessed 2026-06-22.
- [SRC-UE-SEARCH] `platform/system/update_engine` README / testing notes, Git at Google —
  "Cuttlefish works as well" for testing the real `update_engine` A/B apply — accessed
  2026-06-22.

### Negative findings / gaps (§11.4.99(B))

- Deep research (§11.4.150) surfaced **no** working rootless-Podman Cuttlefish recipe;
  every documented, working path is `--privileged`. Absence of a rootless recipe across the
  official container README + AOSP on-premises docs + community writeups is the cited basis
  for "no rootless option" under §11.4.161 — not an assumption.
- `UNCONFIRMED:` whether `nezha`'s `vhost_vsock`/`vhost_net` are built-in vs unloaded, and
  whether the `/dev/vsock` client node is required in addition to `/dev/vhost-vsock` on this
  kernel — both are verify-before-first-launch items (§4), not blockers to recording this
  exception.
