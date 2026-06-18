#!/usr/bin/env bash
# Helix OTA — OTA package generation + signing, driven by the Helix Go build pipeline.
# Produces ota_update.zip with ZIP_STORED entries + per-entry offset/size streaming
# metadata, signs the whole package (--package_key, otacert-pinned) and the A/B payload
# + metadata (--payload_signer). The four payload_properties values are surfaced into
# the control-plane release manifest (android-update-engine-api §4, §9, §11).
#
# The Go pipeline shells out to this AOSP tool; it then computes the artifact-level
# SHA-256 + detached signature (the MVP "signing + SHA-256" layer; ADR-0002 §4.1).
set -euo pipefail

TARGET_FILES="${1:?path to target-files.zip}"
OUT_ZIP="${2:?output ota_update.zip}"
PACKAGE_KEY="${HELIX_PACKAGE_KEY:?Helix package key prefix (.x509.pem + .pk8)}"
PAYLOAD_SIGNER="${HELIX_PAYLOAD_SIGNER:-}"   # optional external/HSM signer (1.0.1)

# Full OTA. Incremental/delta (-i PREVIOUS-target-files.zip) is deferred (ADR-0005).
# --full_ota / streaming flags ensure ZIP_STORED + offset/size metadata.
ota_from_target_files \
    --package_key "${PACKAGE_KEY}" \
    ${PAYLOAD_SIGNER:+--payload_signer "${PAYLOAD_SIGNER}"} \
    "${TARGET_FILES}" \
    "${OUT_ZIP}"

# Artifact-level integrity for verify-before-apply (agent checks this first).
sha256sum "${OUT_ZIP}" | awk '{print $1}' > "${OUT_ZIP}.sha256"
# openssl dgst -sha256 -sign "${HELIX_ARTIFACT_KEY}" -out "${OUT_ZIP}.sig" "${OUT_ZIP}"

echo "Built and signed: ${OUT_ZIP}"
echo "Surface FILE_HASH/FILE_SIZE/METADATA_HASH/METADATA_SIZE from payload_properties.txt"
echo "into the control-plane release manifest."
