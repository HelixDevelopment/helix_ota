/*
 * Helix OTA — WorkManager poll scheduling: 15 min + configurable jitter.
 * Cadence/jitter values come from Config-KMP (runtime-tunable per fleet).
 * NOTE: WorkManager enforces a 15-min minimum periodic interval, which
 * matches the locked cadence (master §5 / D7). Constant value UNVERIFIED
 * against the target AndroidX version.
 */
package digital.vasic.helix.ota.agent.poll

import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import java.time.Duration
import kotlin.random.Random

/** Poll config sourced from Config-KMP (illustrative surface). */
data class PollConfig(
    val periodMinutes: Long = 15L,        // locked baseline cadence
    val jitterMaxMillis: Long = 5 * 60_000L,
    val requireUnmetered: Boolean = false,
    val requireCharging: Boolean = false,
)

object PollScheduler {

    private const val WORK_NAME = "helix-ota-poll"

    fun schedule(wm: WorkManager, cfg: PollConfig) {
        val constraints = Constraints.Builder()
            .setRequiredNetworkType(
                if (cfg.requireUnmetered) NetworkType.UNMETERED else NetworkType.CONNECTED
            )
            .setRequiresCharging(cfg.requireCharging)
            .build()

        // Jitter spreads fleet poll times so millions of devices don't stampede
        // the control plane at the same instant (scalability guarantee, master §1).
        val jitter = if (cfg.jitterMaxMillis > 0) {
            Random.nextLong(0, cfg.jitterMaxMillis)
        } else 0L

        val request = PeriodicWorkRequestBuilder<OtaPollWorker>(
            Duration.ofMinutes(cfg.periodMinutes)
        )
            .setConstraints(constraints)
            .setInitialDelay(Duration.ofMillis(jitter))
            .setBackoffCriteria(
                BackoffPolicy.EXPONENTIAL,
                Duration.ofSeconds(30),
            )
            .addTag(WORK_NAME)
            .build()

        wm.enqueueUniquePeriodicWork(
            WORK_NAME,
            // KEEP so a reschedule with new jitter does not duplicate work.
            ExistingPeriodicWorkPolicy.UPDATE,
            request,
        )
    }
}
