# RK3588 / Orange Pi 5 Max — A/B-virt Emulator — Status Summary

**Revision:** 1
**Last modified:** 2026-06-11T12:10:00Z
**Companion of:** [`Status.md`](Status.md) (§11.4.56 two-audience parity).

---

## Page 1 — For the operator (plain language)

We are building a way to test the RK3588 / Orange Pi 5 Max over-the-air (OTA)
update flow **without needing the actual board on the desk yet** — by emulating
the device on this Mac.

**What works today:**

- The emulated device **boots up and runs**. We built a small aarch64 Linux
  image and it boots all the way to a working login on this Mac (using Apple's
  built-in virtualization). We have the full boot recording as proof.
- The container-based test (control plane talking to a fake device) also works
  end-to-end.

**What is still pending (NOT done yet — honestly):**

- **The real "switch to the new system half and roll back if it fails" part is
  NOT proven yet.** That is the heart of the A/B update (pick slot A or B,
  verify it, fall back if the new one is bad). The pieces for it are being built
  into the image right now, but there is no proof it works yet, so we are not
  claiming it does.
- **The full real-Android A/B update test** needs a **Linux machine** (it
  cannot run on this Mac — the Mac lacks the required virtualization device).
  It is ready to run the moment your incoming Linux host is available; for now
  it is correctly **skipped**, not faked.
- Testing on the **real RK3588 / Orange Pi 5 Max board** is pending — no board
  on the bench yet.

**Bottom line:** boots on the Mac = proven. Real A/B slot switch + the
Linux-host Android A/B test = pending / waiting on the right environment. We do
not pretend anything works that has not been captured as evidence.

---

## Page 2 — For software engineers

**Fidelity ladder.** T0 podman control-plane+emulator (SHIPPED, protocol-only)
→ T1 A/B-virt base image on QEMU `virt` + HVF (FOUNDATION GREEN: live-userspace
boot; A/B-apply layers in progress) → T2 Cuttlefish real Android `update_engine`
A/B + AVB/dm-verity + auto-rollback (SKIP on this host, Linux+KVM-gated) → T3
real RK3588 hardware (PENDING).

**Proven (captured evidence):**

- **PWU-AB-1 base image** — `tests/emulator/ab_virt/out/images/Image`
  (MD5 `9f3670cd7dba7bdeeebe2c6d791e929a`) +
  `tests/emulator/ab_virt/out/images/rootfs.ext2`
  (MD5 `a056760e88eea575977be13e38cfe430`); builder
  `tests/emulator/ab_virt/build_image.sh`. (A U-Boot+RAUC rebuild is in flight
  and will replace these.)
- **PWU-AB-1 foundation boot** — `docs/qa/20260611T061626Z-ab-virt-boot/console.log`
  (196 lines): kernel on Apple CPU MIDR `0x610f` (`physical CPU … [0x610f0000]`),
  Buildroot `buildroot login: root`, post-login `uname -a → Linux buildroot
  6.1.44 … aarch64`, sentinel `HELIX_USERSPACE_LIVE_OK` (line 182), clean
  `poweroff`. §11.4.107 liveness battery (full transcript + post-login sentinel,
  not a single frame). Driver `tests/emulator/ab_virt/boot_smoke.sh`.
- **T0 round-trip** — `tests/emulator/tier1_container_e2e.sh` (register →
  update-check → telemetry asserted from captured podman logs under
  `docs/qa/<run-id>/`).

**Pending / not proven (§11.4.6):**

- **A/B slot switch + dm-verity + auto-rollback (E3–E5)** = PENDING_FORENSICS.
  No captured slot-switch or rollback evidence. U-Boot qemu_arm64 bootcount +
  RAUC dm-verity slot layers (commit `4278aa9`) are being built into the image;
  not yet exercised.
- **T2 Cuttlefish (E7)** = SKIP. Driver `tests/emulator/tier2_cuttlefish_ab.sh`;
  host gate `/dev/kvm` absent on this Apple-Silicon macOS host (verified). Exit-3
  honest topology SKIP per §11.4.3/§11.4.112. Script header marks the exact
  OTA-apply invocation `UNCONFIRMED:` (Virtual-A/B vs legacy A/B; `update_device.py`
  vs `cvd`) — runtime-detected, never guessed. Ready for the operator's incoming
  Linux + nested-KVM host.
- **T3 hardware (E8)** = PENDING_FORENSICS. No board.

**Provenance:** `dd43738` (research + dev-host A/B-virt build infra) → `d5374d0`
(PWU-AB-1 foundation GREEN, live-userspace boot) → `4278aa9` (U-Boot + RAUC + GPT
tooling, PWU-AB-1 full in progress). HEAD `4278aa9`.

**§-refs:** §11.4.45 (Status doc), §11.4.56 (two-audience summary), §11.4.5
(captured-evidence table), §11.4.6 (no-guessing), §11.4.107 (liveness),
§11.4.3 / §11.4.112 (topology-gated SKIP), §11.4.44 (revision header).
