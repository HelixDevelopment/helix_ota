#!/usr/bin/env bash
# =============================================================================
# gen_dev_keys.sh — RK3588 A/B-virt emulator: RAUC dev signing key + cert (PWU-AB-2)
# -----------------------------------------------------------------------------
# Purpose:
#   Generate a THROWAWAY self-signed development RSA key + X.509 cert used to
#   sign the RAUC dm-verity bundle (PWU-AB-2) and to seed the in-guest RAUC
#   keyring (system.conf [keyring] path -> the cert). Dev-only emulator trust —
#   a production build uses a real PKI (§11.4.10). This is the exact openssl
#   recipe from the RAUC examples doc, parameterised for this project.
#
# Outputs (into the GITIGNORED out/ tree — /tests/emulator/ab_virt/out/ is in
# .gitignore, so the key NEVER lands in git, §11.4.10/§11.4.30):
#   out/rauc-keys/dev.key.pem    PRIVATE key  (chmod 600 — NEVER committed)
#   out/rauc-keys/dev.cert.pem   PUBLIC cert  (the bundle cert + guest keyring)
#   parent dir chmod 700.
#
# §11.4.10 (credentials): the private key is a generated, gitignored secret —
#   this script CREATES it at build time; it is NOT stored in the repo, NOT
#   printed, NOT logged. Idempotent: refuses to overwrite an existing key unless
#   --force (so a re-run does not silently rotate a key a built bundle was signed
#   with). The cert subject/days are dev-only.
#
# Usage:
#   tests/emulator/ab_virt/rauc/gen_dev_keys.sh           # generate if absent
#   tests/emulator/ab_virt/rauc/gen_dev_keys.sh --force   # regenerate (rotate)
#   Env: HELIX_RAUC_KEY_DIR  (default <repo>/tests/emulator/ab_virt/out/rauc-keys)
#        HELIX_RAUC_KEY_CN    (default helix-ota-ab-virt-dev)
#        HELIX_RAUC_KEY_DAYS  (default 365)
#
# Dependencies: openssl.
# Cross-refs: PWU_AB_2_RAUC_VERITY.md §4.1 ; system.conf [keyring] (consumes the
#   cert) ; manifest.raucm.in (the bundle this key signs).
# §11.4.6 STATUS: authored helper, NOT yet run. No key exists in-tree.
#
# Sources verified 2026-06-11:
#   https://rauc.readthedocs.io/en/latest/examples.html  (openssl req -x509
#     -newkey rsa:4096 -nodes self-signed dev cert; --cert/--key to rauc bundle)
# =============================================================================
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
KEY_DIR="${HELIX_RAUC_KEY_DIR:-${REPO_ROOT}/tests/emulator/ab_virt/out/rauc-keys}"
KEY_CN="${HELIX_RAUC_KEY_CN:-helix-ota-ab-virt-dev}"
KEY_DAYS="${HELIX_RAUC_KEY_DAYS:-365}"
KEY="${KEY_DIR}/dev.key.pem"
CERT="${KEY_DIR}/dev.cert.pem"

FORCE=0
if [ "${1:-}" = "--force" ]; then FORCE=1; fi

log() { printf '[gen_dev_keys] %s\n' "$*"; }

command -v openssl >/dev/null 2>&1 || { log "ABORT: openssl not found"; exit 3; }

# §11.4.10 idempotence: do NOT silently rotate a key an existing bundle trusts.
if [ -s "$KEY" ] && [ -s "$CERT" ] && [ "$FORCE" -ne 1 ]; then
  log "dev key + cert already present at ${KEY_DIR} (use --force to rotate); leaving untouched"
  exit 0
fi

# Guard: the key dir MUST be inside the gitignored out/ tree. Refuse to write a
# secret anywhere git could track it (§11.4.10/§11.4.30 belt-and-suspenders).
case "$KEY_DIR" in
  */tests/emulator/ab_virt/out/*) : ;;
  *)
    log "ABORT: refusing to write a private key outside the gitignored out/ tree:"
    log "       ${KEY_DIR}"
    log "       (set HELIX_RAUC_KEY_DIR under tests/emulator/ab_virt/out/ — §11.4.10)"
    exit 2
    ;;
esac

mkdir -p "$KEY_DIR"
chmod 700 "$KEY_DIR" 2>/dev/null || true

log "generating throwaway self-signed dev key+cert (CN=${KEY_CN}, ${KEY_DAYS}d) in ${KEY_DIR}"
# The RAUC examples-doc recipe (self-signed dev cert). -nodes = unencrypted key
# (a build-time throwaway; never committed).
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout "$KEY" \
  -out    "$CERT" \
  -subj   "/O=Helix OTA dev/CN=${KEY_CN}" \
  -days   "$KEY_DAYS" >/dev/null 2>&1 \
  || { log "ABORT: openssl key/cert generation failed"; exit 1; }

chmod 600 "$KEY"  2>/dev/null || true
chmod 644 "$CERT" 2>/dev/null || true

# §11.4.6: declare success ONLY if both artifacts exist non-empty. Do NOT print
# or cat the key (§11.4.10 — never log a secret).
if [ -s "$KEY" ] && [ -s "$CERT" ]; then
  log "OK — dev key (chmod 600, NOT committed) + cert generated:"
  log "  key : ${KEY}"
  log "  cert: ${CERT}"
  log "  cert fingerprint:"
  openssl x509 -in "$CERT" -noout -fingerprint -sha256 2>/dev/null | sed 's/^/    /' || true
  exit 0
fi
log "FAILED — key/cert missing after openssl (NOT stamping success, §11.4.6)"
exit 1
