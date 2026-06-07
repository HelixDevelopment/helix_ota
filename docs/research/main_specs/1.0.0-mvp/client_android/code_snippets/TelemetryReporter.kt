/*
 * Helix OTA — telemetry reporter. Events use the ota-telemetry-schema (shared
 * server + agent; no transport/storage in the schema module). Posted over REST
 * /api/v1 (HTTP/3 -> HTTP/2). The control plane consumes these to drive staged
 * rollout halt/advance (ADR-0001). An update is "done" only after merge complete.
 */
package digital.vasic.helix.ota.agent.telemetry

import digital.vasic.helix.ota.telemetry.TelemetryEvent
import digital.vasic.helix.ota.telemetry.TelemetrySink

/** Agent lifecycle states mirroring the boot_control / Virtual A/B state machine. */
enum class AgentState {
    REGISTERED,
    POLLED, UPDATE_ASSIGNED,
    DOWNLOADING, DOWNLOADED,
    VERIFY_OK, VERIFY_FAILED,
    APPLYING, APPLIED_PENDING_REBOOT,
    BOOTED_NOT_YET_SUCCESSFUL, SUCCESSFUL, ROLLED_BACK,
    MERGE_SNAPSHOTTED, MERGING, MERGE_COMPLETE,
}

class TelemetryReporter(
    private val sink: TelemetrySink,
    private val deviceId: String,
) {
    fun report(state: AgentState) =
        sink.emit(TelemetryEvent.state(deviceId, state.name))

    fun reportProgress(engineStatusCode: Int, percent: Float) =
        sink.emit(TelemetryEvent.progress(deviceId, engineStatusCode, percent))

    fun reportError(state: AgentState, t: Throwable) =
        sink.emit(TelemetryEvent.error(deviceId, state.name, t.message ?: "unknown"))

    /** Map update_engine ErrorCodeConstants directly into telemetry codes. */
    fun reportEngineError(errorCode: Int) =
        sink.emit(TelemetryEvent.engineError(deviceId, errorCode))

    /** High-value failure signals that should drive automatic rollout halt. */
    fun reportBootHealth(
        unexpectedFallback: Boolean,
        verifiedBootDegraded: Boolean,
        verityEio: Boolean,
        mergeStatus: String,
    ) = sink.emit(
        TelemetryEvent.bootHealth(
            deviceId, unexpectedFallback, verifiedBootDegraded, verityEio, mergeStatus,
        )
    )
}
