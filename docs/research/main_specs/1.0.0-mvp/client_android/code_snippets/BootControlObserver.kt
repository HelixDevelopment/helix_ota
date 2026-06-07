/*
 * Helix OTA — read-only observation of the AVB / boot_control / Virtual A/B state
 * for telemetry. The agent OBSERVES but never DRIVES this stack outside update_engine
 * (android-avb-rollback §10). Detecting unexpected fallback / non-GREEN verifiedbootstate
 * / verity EIO / stuck MERGING are high-value rollout-halt signals.
 */
package digital.vasic.helix.ota.bridge

/** Virtual A/B MergeStatus (android15-virtual-ab §9). */
enum class MergeStatus { NONE, UNKNOWN, SNAPSHOTTED, MERGING, CANCELLED }

data class BootHealth(
    val currentSlot: String,
    val commandedSlot: String?,         // slot the agent told the device to boot
    val verifiedBootState: String,      // GREEN / YELLOW / ORANGE / RED
    val verityMode: String,             // ro.boot.veritymode
    val slotSuccessful: Boolean,
    val mergeStatus: MergeStatus,
) {
    /** Unexpected fallback: booted a slot other than the one we commanded. */
    val unexpectedFallback: Boolean
        get() = commandedSlot != null && currentSlot != commandedSlot

    /** Production fleets should be GREEN (or YELLOW for a custom-key fleet). */
    val verifiedBootDegraded: Boolean
        get() = verifiedBootState != "green" && verifiedBootState != "yellow"

    /** INVARIANT: never trigger a slot revert once merge has started. */
    val revertForbidden: Boolean
        get() = mergeStatus == MergeStatus.MERGING
}

interface BootControlObserver {
    fun read(commandedSlot: String?): BootHealth
}
