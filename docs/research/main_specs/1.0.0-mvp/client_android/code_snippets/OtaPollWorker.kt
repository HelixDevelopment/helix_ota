/*
 * Helix OTA — WorkManager worker that runs one poll->download->verify->apply cycle.
 * Transient failures use WorkManager backoff (Result.retry); the agent never busy-loops.
 */
package digital.vasic.helix.ota.agent.poll

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import digital.vasic.helix.ota.agent.apply.UpdateApplier
import digital.vasic.helix.ota.agent.download.Downloader
import digital.vasic.helix.ota.agent.telemetry.TelemetryReporter
import digital.vasic.helix.ota.agent.telemetry.AgentState
import digital.vasic.helix.ota.agent.verify.Verifier
import digital.vasic.helix.ota.protocol.UpdateAssignment

class OtaPollWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {

    // In production these are injected; shown inline for the snippet.
    private val client = AgentGraph.controlPlaneClient(applicationContext)
    private val downloader: Downloader = AgentGraph.downloader(applicationContext)
    private val verifier: Verifier = AgentGraph.verifier(applicationContext)
    private val applier: UpdateApplier = AgentGraph.updateApplier(applicationContext)
    private val telemetry: TelemetryReporter = AgentGraph.telemetry(applicationContext)

    override suspend fun doWork(): Result {
        telemetry.report(AgentState.POLLED)

        // GET /api/v1/devices/{id}/update  -> 200 manifest | 204 no update
        val assignment: UpdateAssignment = when (val r = client.pollForUpdate()) {
            is PollResponse.NoUpdate -> return Result.success()
            is PollResponse.Update -> r.assignment
            is PollResponse.TransientError -> return Result.retry()
        }
        telemetry.report(AgentState.UPDATE_ASSIGNED)

        // 1) download to /data/ota_package (Range, identity content-encoding)
        telemetry.report(AgentState.DOWNLOADING)
        val artifact = try {
            downloader.downloadToLocal(assignment)
        } catch (t: Throwable) {
            telemetry.reportError(AgentState.DOWNLOADING, t)
            return Result.retry()
        }
        telemetry.report(AgentState.DOWNLOADED)

        // 2) verify-before-apply: SHA-256 + signature + the four payload props
        val verified = verifier.verify(artifact, assignment)
        if (!verified.ok) {
            telemetry.report(AgentState.VERIFY_FAILED)
            downloader.delete(artifact)
            return Result.failure() // poisoned/wrong artifact never reaches update_engine
        }
        telemetry.report(AgentState.VERIFY_OK)

        // 3) hand the VERIFIED local file to update_engine via the bridge
        return try {
            applier.applyVerified(verified)   // emits APPLYING.. APPLIED_PENDING_REBOOT
            Result.success()
        } catch (t: Throwable) {
            telemetry.reportError(AgentState.APPLYING, t)
            Result.retry()
        }
    }
}
