/*
 * Helix OTA — verify-before-apply (SHA-256 + signature) then read/verify the four
 * payload_properties values. Safety-critical (>=90% coverage; master §13).
 * Crypto primitives come from Security-KMP (no bespoke crypto in the agent).
 * MVP trust = signing + SHA-256 + AVB; TUF device-side deferred to 1.0.1 (ADR-0002).
 * NOTE: Security-KMP SHA-256 + detached-signature surface is UNVERIFIED.
 */
package digital.vasic.helix.ota.agent.verify

import digital.vasic.helix.ota.agent.download.LocalArtifact
import digital.vasic.helix.ota.protocol.UpdateAssignment

/** The four lines of payload_properties.txt (android-update-engine-api §4, §6). */
data class PayloadProperties(
    val fileHashB64: String,      // FILE_HASH    = base64(SHA-256) of payload.bin
    val fileSize: Long,           // FILE_SIZE    = bytes
    val metadataHashB64: String,  // METADATA_HASH= base64(SHA-256) of metadata prefix
    val metadataSize: Long,       // METADATA_SIZE= bytes
) {
    fun asHeaderArray(): Array<String> = arrayOf(
        "FILE_HASH=$fileHashB64",
        "FILE_SIZE=$fileSize",
        "METADATA_HASH=$metadataHashB64",
        "METADATA_SIZE=$metadataSize",
    )
}

data class VerifiedArtifact(
    val ok: Boolean,
    val artifact: LocalArtifact,
    val props: PayloadProperties,
    val payloadOffset: Long,   // offset of payload.bin entry within the outer zip
    val payloadSize: Long,     // size of payload.bin entry
)

/** Port over Security-KMP. */
interface Crypto {
    fun sha256(path: String): ByteArray
    /** Verifies a detached signature over [hash] with the Helix-pinned public key. */
    fun verifySignature(hash: ByteArray, signature: ByteArray): Boolean
    fun base64(bytes: ByteArray): String
}

/** Reads payload_properties.txt and the payload.bin zip entry offsets. */
interface ZipInspector {
    fun readPayloadProperties(zipPath: String): PayloadProperties
    fun payloadEntryRange(zipPath: String): Pair<Long, Long> // offset, size
    fun fileSize(zipPath: String): Long
}

class Verifier(
    private val crypto: Crypto,
    private val zip: ZipInspector,
) {
    fun verify(artifact: LocalArtifact, a: UpdateAssignment): VerifiedArtifact {
        val fail = VerifiedArtifact(false, artifact, EMPTY, 0, 0)

        // (1) Helix MVP trust: artifact SHA-256 + detached signature.
        val digest = crypto.sha256(artifact.path)
        if (crypto.base64(digest) != a.expectedSha256B64) return fail
        if (!crypto.verifySignature(digest, a.signature)) return fail

        // (2) payload header verification (independent of the engine's own check).
        val props = zip.readPayloadProperties(artifact.path)
        val (offset, size) = zip.payloadEntryRange(artifact.path)
        if (props.fileSize != size) return fail
        // Manifest carries the same four values from the build pipeline; cross-check.
        if (props.fileHashB64 != a.props.fileHashB64) return fail
        if (props.metadataHashB64 != a.props.metadataHashB64) return fail

        // Only a fully-verified artifact proceeds to update_engine.
        return VerifiedArtifact(true, artifact, props, offset, size)
    }

    private companion object {
        val EMPTY = PayloadProperties("", 0, "", 0)
    }
}
