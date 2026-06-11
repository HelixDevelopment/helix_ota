# RK3588 / Orange Pi 5 Max — A/B-virt Emulator — Status Summary

**Revision:** 3
**Last modified:** 2026-06-11T13:15:00Z
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
- **The heart of the A/B update now works and is proven.** Using a real
  bootloader (U-Boot 2024.01) on the Mac, the system can **switch from system
  half A to system half B** — we ran it and the device genuinely came up on the
  other half (confirmed from inside the running device), and we repeated it
  twice with identical results.
- **Automatic roll-back works too.** When we mark the new half as "bad" (too
  many failed boot attempts), the bootloader automatically **falls back to the
  known-good half** — we ran it and the device recovered to the good half. A
  second control run proved the roll-back only happens when a half is actually
  bad, not every time.
- The container-based test (control plane talking to a fake device) also works
  end-to-end.
- Our automated quality bank for this project passes its checks (12 of 12),
  including the check that the boot recording above is real and on file.

**What is still pending (NOT done yet — honestly):**

- **The dm-verity tamper-protection layer (RAUC) is NOT proven yet.** The script
  for it is written but has not been run — it needs a signed update bundle and a
  small reconciliation between the RAUC slot scheme and the bootloader slot
  scheme we just proved. Until we run it and capture the result, we do not claim
  it works.
- **The full real-Android A/B update test** needs a **Linux machine** (it
  cannot run on this Mac — the Mac lacks the required virtualization device).
  It is ready to run the moment your incoming Linux host is available; for now
  it is correctly **skipped**, not faked.
- Testing on the **real RK3588 / Orange Pi 5 Max board** is pending — no board
  on the bench yet.

**Bottom line:** boots on the Mac = proven; the real A/B slot switch and
automatic roll-back = **proven** (with recordings on file). Still pending:
dm-verity (RAUC), the Linux-host real-Android A/B test, and the real board.
**No release version has been tagged** — that needs the full ladder green,
including the Linux-host and real-board tiers, so tagging now would be
overclaiming. We do not pretend anything works that has not been captured as
evidence.

---

## Page 2 — For software engineers

**Fidelity ladder.** T0 podman control-plane+emulator (SHIPPED, protocol-only)
→ T1 A/B-virt base image on QEMU `virt` + HVF (**GREEN**: live-userspace boot +
real U-Boot 2024.01 A/B slot switch (E3) + corrupt-slot auto-rollback (E5)
PROVEN; RAUC dm-verity (E4) AUTHORED, not yet run) → T2 Cuttlefish real Android
`update_engine` A/B + AVB/dm-verity + auto-rollback incl. PWU-CF-2 corrupt-slot
rollback (SKIP/UNVERIFIED on this host, Linux+KVM-gated) → T3 real RK3588
hardware (PENDING).

**Proven (captured evidence):**

- **PWU-AB-1 base image + real `u-boot.bin`** — `tests/emulator/ab_virt/out/images/Image`
  (MD5 `9f3670cd7dba7bdeeebe2c6d791e929a`) +
  `tests/emulator/ab_virt/out/images/rootfs.ext2`
  (MD5 `a056760e88eea575977be13e38cfe430`); builder
  `tests/emulator/ab_virt/build_image.sh`. The U-Boot+RAUC build COMPLETED — the
  E3/E5 verdict files record `U-Boot 2024.01 (Jun 11 2026 - 08:21:42 +0000)`
  driving the boot.
- **PWU-AB-1 foundation boot** — `docs/qa/20260611T061626Z-ab-virt-boot/console.log`
  (196 lines): kernel on Apple CPU MIDR `0x610f` (`physical CPU … [0x610f0000]`),
  Buildroot `buildroot login: root`, post-login `uname -a → Linux buildroot
  6.1.44 … aarch64`, sentinel `HELIX_USERSPACE_LIVE_OK` (line 182), clean
  `poweroff`. §11.4.107 liveness battery (full transcript + post-login sentinel,
  not a single frame). Driver `tests/emulator/ab_virt/boot_smoke.sh`.
- **PWU-AB-1 A/B slot switch (E3)** — `docs/qa/20260611T094958Z-ab-slot-switch/`:
  `verdict.txt` (Run A rc=0 → `HELIX_SLOTID=A` `/dev/vda2`; Run B rc=0 →
  `HELIX_SLOTID=B` `/dev/vda3` — the slot SWITCHED), `consoleA.log` /
  `consoleB.log` transcripts, `determinism_n3.txt` (**3/3 identical PASS**,
  §11.4.50). Each run asserts the post-login `slot_id` sentinel + `findmnt`
  root-device + kernel `helix_slot=` + the negative "did NOT boot the other
  slot" check (real switch, not a no-op). Drivers
  `tests/emulator/ab_virt/ab_slot_switch.sh` + `uboot_ab/boot.cmd` (commit
  `18ed84a`).
- **PWU-AB-3 corrupt-slot auto-rollback (E5)** — `docs/qa/20260611T095918Z-ab-rollback/`:
  `verdict.txt` (Run ROLLBACK rc=0: head=bad slot B, `bootcount=2 > bootlimit=1`
  → altbootcmd swap → booted slot A `/dev/vda2`; Run CONTROL rc=0: head B, no
  rollback → B `/dev/vda3`), `consoleROLLBACK.log` shows
  `A/B: bootcount=2 > bootlimit=1 -> rolling back (altbootcmd swap)` →
  `booting slot A` → guest `HELIX_SLOTID=A`. The CONTROL run is the negative
  proof the rollback fires only on a bad slot. Drivers
  `tests/emulator/ab_virt/ab_rollback.sh` + `uboot_ab/boot.cmd` (commit
  `42be557`).
- **T0 round-trip** — `tests/emulator/tier1_container_e2e.sh` (register →
  update-check → telemetry asserted from captured podman logs under
  `docs/qa/<run-id>/`).
- **HelixQA bank static gate (E9)** —
  `bash tools/helixqa/run_bank.sh --dry-run --bank tools/helixqa/banks/helix_ota.yaml`
  → 12 passed / 0 failed / 0 skipped. `HOTA-AB-VIRT-BOOT` declares evidence
  `docs/qa/20260611T061626Z-ab-virt-boot/console.log` (verified present). The
  runner FAILs any challenge whose declared evidence is missing — this gates
  that the boot evidence exists + is wired into the bank (it does NOT itself
  exercise A/B apply).

**Pending / not proven (§11.4.6):**

- **RAUC dm-verity slot integrity (E4)** = PENDING_FORENSICS. Driver
  `tests/emulator/ab_virt/ab_rauc_verity.sh` AUTHORED + parse-clean but **NOT
  run** — gated on a signed `.raucb` bundle + reconciling the RAUC slot-class
  scheme with the proven `boot.cmd` `BOOT_ORDER` env scheme. No dm-verity
  evidence yet.
- **T2 Cuttlefish (E7)** = SKIP / UNVERIFIED-pending-Linux-host. Driver
  `tests/emulator/tier2_cuttlefish_ab.sh` (PWU-CF-2 corrupt-slot auto-rollback
  section authored, mirrors ab_virt PWU-AB-3); host gate `/dev/kvm` absent on
  this Apple-Silicon macOS host (verified). Exit-3 honest topology SKIP per
  §11.4.3/§11.4.112. Script header marks the exact OTA-apply + rollback
  invocation `UNCONFIRMED:` (Virtual-A/B vs legacy A/B; `update_device.py` vs
  `cvd`) — runtime-detected, never guessed. Ready for the operator's incoming
  Linux + nested-KVM host.
- **T3 hardware (E8)** = PENDING_FORENSICS. No board.
- **No release tag** — §11.4.40 needs the full ladder GREEN (T2 + T3 still
  SKIP/PENDING); tagging now would be a bluff.

**Provenance:** `dd43738` (research + dev-host A/B-virt build infra) → `d5374d0`
(PWU-AB-1 foundation GREEN, live-userspace boot) → `4278aa9` (U-Boot + RAUC + GPT
tooling) → `0f120a6` (parallel streams: HelixQA bank wiring + these Status docs +
Cuttlefish PWU-CF-2 rollback) → `6581ab4` (A/B disk assembler + U-Boot bootcount
slot-select, AUTHORED) → `0d27438` (build host-tools `libssl-dev` fix) →
`18ed84a` (**PWU-AB-1 REAL A/B slot switch PROVEN**, E3) → `afcad1d` (determinism
soak + PWU-AB-2 `ab_rauc_verity.sh` / PWU-AB-3 authored) → `42be557` (**PWU-AB-3
REAL corrupt-slot AUTO-ROLLBACK PROVEN**, E5). HEAD `42be557`.

**§-refs:** §11.4.45 (Status doc), §11.4.56 (two-audience summary), §11.4.5
(captured-evidence table), §11.4.6 (no-guessing), §11.4.107 (liveness),
§11.4.3 / §11.4.112 (topology-gated SKIP), §11.4.44 (revision header).
