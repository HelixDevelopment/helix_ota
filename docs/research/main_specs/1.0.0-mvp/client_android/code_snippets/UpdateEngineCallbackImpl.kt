/*
 * Helix OTA — UpdateEngineCallback bridge (androidMain, inside ota-update-engine-bridge).
 * Wraps the @SystemApi android.os.UpdateEngine. Requires system-UID / privileged-app.
 * Status & error constants verified in android-update-engine-api §8.
 *
 * Pseudo-Kotlin: android.os.UpdateEngine / UpdateEngineCallback are @SystemApi and only
 * resolve when built in the AOSP tree (platform_apis: true; see Android.bp).
 */
package digital.vasic.helix.ota.bridge

import android.os.UpdateEngine
import android.os.UpdateEngineCallback
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow

/** UpdateStatusConstants (verified values, android-update-engine-api §8). */
object Status {
    const val IDLE = 0
    const val CHECKING_FOR_UPDATE = 1
    const val UPDATE_AVAILABLE = 2
    const val DOWNLOADING = 3
    const val VERIFYING = 4
    const val FINALIZING = 5
    const val UPDATED_NEED_REBOOT = 6
    const val REPORTING_ERROR_EVENT = 7
    const val ATTEMPTING_ROLLBACK = 8
    const val DISABLED = 9
    // const val CLEANUP_PREVIOUS_UPDATE = 12  // UNVERIFIED for Android 15
}

/** ErrorCodeConstants (verified where literal-sourced, android-update-engine-api §8). */
object ErrorCode {
    const val SUCCESS = 0
    const val ERROR = 1
    const val DOWNLOAD_TRANSFER_ERROR = 9
    const val PAYLOAD_HASH_MISMATCH_ERROR = 10
    const val PAYLOAD_SIZE_MISMATCH_ERROR = 11
    const val DOWNLOAD_PAYLOAD_VERIFICATION_ERROR = 12
    const val PAYLOAD_TIMESTAMP_ERROR = 51          // anti-rollback: target older than current
    const val UPDATED_BUT_NOT_ACTIVE = 52
    const val NOT_ENOUGH_SPACE = 60                 // Virtual A/B COW space
    const val DEVICE_CORRUPTED = 61
}

sealed interface EngineStatus {
    data class Progress(val code: Int, val percent: Float) : EngineStatus
    data object NeedReboot : EngineStatus
    data class Complete(val errorCode: Int) : EngineStatus
}

/** Real bridge over the @SystemApi UpdateEngine. */
class AndroidUpdateEngineBridge : UpdateEngineBridge {

    private val engine = UpdateEngine()

    override fun applyVerifiedPackage(
        fileUrl: String, offset: Long, size: Long, props: Array<String>,
    ): ApplyHandle {
        // The local file is already verified by the agent (Security-KMP).
        engine.applyPayload(fileUrl, offset, size, props)
        return ApplyHandle(fileUrl)
    }

    override fun observeStatus(): Flow<EngineStatus> = callbackFlow {
        val cb = object : UpdateEngineCallback() {
            override fun onStatusUpdate(status: Int, percent: Float) {
                if (status == Status.UPDATED_NEED_REBOOT) trySend(EngineStatus.NeedReboot)
                else trySend(EngineStatus.Progress(status, percent))
            }
            override fun onPayloadApplicationComplete(errorCode: Int) {
                trySend(EngineStatus.Complete(errorCode))
            }
        }
        engine.bind(cb)
        // unbind() is a real @SystemApi method on android.os.UpdateEngine;
        // call it directly to detach the callback when the flow is cancelled.
        awaitClose { engine.unbind() }
    }

    // Read-only observers for telemetry (never mutate slot state).
    override fun currentSlot(): String = readProp("ro.boot.slot_suffix")
    override fun verifiedBootState(): String = readProp("ro.boot.verifiedbootstate")
}
