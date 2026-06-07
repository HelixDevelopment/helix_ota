/*
 * Helix OTA — download the full ota_update.zip to /data/ota_package BEFORE apply
 * (verify-before-apply / local file:// path; ADR-0002 §4.1, android-update-engine-api §7).
 * Bytes are persisted via Storage-KMP; transport uses Range + identity content-encoding
 * (ADR-0004 §106). The agent owns retry/resume on the local path.
 * NOTE: Storage-KMP range-get surface is UNVERIFIED.
 */
package digital.vasic.helix.ota.agent.download

import digital.vasic.helix.ota.protocol.UpdateAssignment

/** A fully-downloaded, on-disk artifact (not yet verified). */
data class LocalArtifact(
    val path: String,            // e.g. /data/ota_package/ota_update.zip
    val sizeBytes: Long,
)

/** Port over Storage-KMP for resumable, range-based blob download. */
interface BlobStore {
    /** Free space on the partition backing /data/ota_package. */
    fun freeBytes(targetDir: String): Long
    /** Resumable download; uses HTTP Range to continue a partial file. */
    suspend fun downloadResumable(url: String, targetPath: String, expectedSize: Long): LocalArtifact
    fun delete(path: String)
}

class Downloader(
    private val store: BlobStore,
    private val otaDir: String = "/data/ota_package",
) {
    suspend fun downloadToLocal(a: UpdateAssignment): LocalArtifact {
        // Gate on free /data: low space lowers Virtual A/B success (COW may spill
        // to /data). android15-virtual-ab §8. Reported as telemetry by the caller.
        val free = store.freeBytes(otaDir)
        require(free > a.artifactSizeBytes) {
            "insufficient free /data: have=$free need=${a.artifactSizeBytes}"
        }
        val target = "$otaDir/ota_update.zip"
        return store.downloadResumable(a.artifactUrl, target, a.artifactSizeBytes)
    }

    fun delete(artifact: LocalArtifact) = store.delete(artifact.path)
}
