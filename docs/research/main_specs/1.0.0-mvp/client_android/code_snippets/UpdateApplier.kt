/*
 * Helix OTA — apply coordinator: hands a VERIFIED local artifact to the
 * ota-update-engine-bridge. The bridge is the ONLY OS-apply path.
 * The agent never disables AVB/dm-verity, never writes slot flags,
 * never calls markBootSuccessful (android-avb-rollback §10).
 */
package digital.vasic.helix.ota.agent.apply

import digital.vasic.helix.ota.agent.telemetry.AgentState
import digital.vasic.helix.ota.agent.telemetry.TelemetryReporter
import digital.vasic.helix.ota.agent.verify.VerifiedArtifact
import digital.vasic.helix.ota.bridge.ApplyHandle
import digital.vasic.helix.ota.bridge.EngineStatus
import digital.vasic.helix.ota.bridge.UpdateEngineBridge

class UpdateApplier(
    private val bridge: UpdateEngineBridge,
    private val telemetry: TelemetryReporter,
) {
    suspend fun applyVerified(v: VerifiedArtifact) {
        telemetry.report(AgentState.APPLYING)

        // file:// (local) apply: bridge computes nothing about trust; it only
        // calls applyPayload(file://…, offset, size, props) on the verified file.
        val handle: ApplyHandle = bridge.applyVerifiedPackage(
            fileUrl = "file://${v.artifact.path}",
            offset = v.payloadOffset,
            size = v.payloadSize,
            props = v.props.asHeaderArray(),   // FILE_HASH/FILE_SIZE/METADATA_HASH/METADATA_SIZE
        )

        bridge.observeStatus().collect { status: EngineStatus ->
            when (status) {
                is EngineStatus.Progress ->
                    telemetry.reportProgress(status.code, status.percent)
                is EngineStatus.NeedReboot -> {
                    telemetry.report(AgentState.APPLIED_PENDING_REBOOT)
                    // Reboot to switch slots; update_verifier + AVB + framework
                    // markBootSuccessful take over. Merge completes post-boot.
                    bridge.rebootToNewSlot(handle)
                }
                is EngineStatus.Complete ->
                    if (status.errorCode == 0) telemetry.report(AgentState.APPLIED_PENDING_REBOOT)
                    else telemetry.reportEngineError(status.errorCode)
            }
        }
    }
}
