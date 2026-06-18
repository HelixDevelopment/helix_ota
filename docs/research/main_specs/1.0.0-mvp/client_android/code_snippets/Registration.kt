/*
 * Helix OTA — device registration (ota-android-agent, commonMain-facing).
 * Reuses Auth-KMP for the device token and ota-protocol for wire types.
 * REST primary (/api/v1), HTTP/3 (QUIC) -> HTTP/2 fallback per ADR-0004.
 * NOTE: illustrative; Auth-KMP / ota-protocol public APIs are UNVERIFIED.
 */
package digital.vasic.helix.ota.agent.registration

import digital.vasic.helix.ota.protocol.DeviceRegistration
import digital.vasic.helix.ota.protocol.RegistrationResult

/** Thin port over Auth-KMP token issue/verify. Implemented in androidMain. */
interface DeviceAuth {
    /** Returns a (possibly hardware-bound) device token. */
    suspend fun obtainDeviceToken(hardwareId: String): String
}

/** Thin port over the transport layer (http3 submodule, REST /api/v1). */
interface ControlPlaneClient {
    suspend fun register(token: String, body: DeviceRegistration): RegistrationResult
}

/** Reads stable device facts (androidMain provides the real impl). */
interface DeviceFacts {
    fun hardwareId(): String
    fun buildFingerprint(): String          // ro.build.fingerprint of the running slot
    fun currentSlot(): String               // from boot_control, telemetry only
    fun verifiedBootState(): String         // ro.boot.verifiedbootstate, telemetry only
    fun model(): String
    fun soc(): String                       // e.g. "RK3588"
    fun agentVersion(): String
}

class Registrar(
    private val auth: DeviceAuth,
    private val client: ControlPlaneClient,
    private val facts: DeviceFacts,
) {
    suspend fun registerIfNeeded(): RegistrationResult {
        val hwId = facts.hardwareId()
        val token = auth.obtainDeviceToken(hwId)
        val body = DeviceRegistration(
            hardwareId = hwId,
            currentBuildFingerprint = facts.buildFingerprint(),
            currentSlot = facts.currentSlot(),
            verifiedBootState = facts.verifiedBootState(),
            model = facts.model(),
            soc = facts.soc(),
            agentVersion = facts.agentVersion(),
        )
        // POST /api/v1/devices  (path owned by ota-protocol)
        return client.register(token, body)
    }
}
