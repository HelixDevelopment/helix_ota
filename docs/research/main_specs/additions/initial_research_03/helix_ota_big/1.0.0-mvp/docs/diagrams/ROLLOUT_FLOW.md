# Helix OTA — Rollout Flow

## Overview

This state diagram illustrates the **complete lifecycle of a rollout campaign** in Helix OTA. A rollout progresses through graduated stages (5% → 10% → 30% → 50% → 100%) with built-in controls for pausing, resuming, halting, and rolling back. Each stage gates advancement on success-rate thresholds before exposing more devices to the update.

---

## Diagram

```mermaid
stateDiagram-v2
    direction TB

    [*] --> Draft : Campaign created<br/>by operator

    state Draft {
        [*] --> Configuring
        Configuring --> ArtifactSelected : Select artifact
        Configuring --> DeviceTargeting : Define device filter
        ArtifactSelected --> Ready : All fields valid
        DeviceTargeting : filter by hw_revision,<br/>oem_version range,<br/>device group, region
        DeviceTargeting --> Ready : All fields valid
    }

    Draft --> Active : Operator clicks<br/>"Start Rollout"

    state Active {
        state "Stage: 5%" as S5
        state "Stage: 10%" as S10
        state "Stage: 30%" as S30
        state "Stage: 50%" as S50
        state "Stage: 100%" as S100

        [*] --> S5 : Rollout activated
        S5 --> S10 : Success rate ≥ 95%<br/>+ min 50 devices<br/>+ 30min elapsed
        S10 --> S30 : Success rate ≥ 95%<br/>+ min 100 devices<br/>+ 30min elapsed
        S30 --> S50 : Success rate ≥ 97%<br/>+ min 500 devices<br/>+ 1h elapsed
        S50 --> S100 : Success rate ≥ 98%<br/>+ min 1000 devices<br/>+ 2h elapsed
        S100 --> FullyDeployed : All targeted<br/>devices updated

        state "Threshold Check" as TC {
            [*] --> Evaluate
            Evaluate --> Pass : success_rate ≥ threshold<br/>AND min_devices met<br/>AND time_gate passed
            Evaluate --> Fail : success_rate < threshold<br/>OR critical errors
            Pass --> [*]
            Fail --> [*]
        }

        S5 --> TC : Evaluate<br/>stage gate
        S10 --> TC : Evaluate<br/>stage gate
        S30 --> TC : Evaluate<br/>stage gate
        S50 --> TC : Evaluate<br/>stage gate
    }

    Active --> Paused : Operator clicks<br/>"Pause"<br/>OR auto-pause on<br/>critical failure rate

    Paused --> Active : Operator clicks<br/>"Resume"

    Active --> Halted : Operator clicks<br/>"Halt"<br/>OR auto-halt on<br/>catastrophic failure

    Paused --> Halted : Operator clicks<br/>"Halt"

    Active --> Rollback : Operator triggers<br/>"Rollback"<br/>(force previous build)

    Paused --> Rollback : Operator triggers<br/>"Rollback"

    FullyDeployed --> Completed : All devices<br/>reported success<br/>(or timeout)

    Rollback --> Completed : Rollback<br/>confirmed

    Halted --> [*] : Terminal state<br/>(no further action)

    Completed --> [*] : Terminal state<br/>(archived)

    note right of S5
        5% of targeted devices
        receive the update.
        Canary stage — closest
        monitoring required.
    end note

    note right of S100
        100% rollout — all
        remaining targeted
        devices receive update.
    end note

    note right of Paused
        No new devices are
        selected. In-flight
        updates continue to
        completion.
    end note

    note right of Halted
        All updates stopped.
        In-flight installs
        are allowed to finish.
        Operator must create
        new campaign to retry.
    end note

    note right of Rollback
        Sends rollback command
        to devices that received
        the new build. Devices
        with A/B partitions
        automatically revert
        to previous slot.
    end note
```

## Stage Gate Criteria

| Stage | Success Rate Threshold | Min Devices | Min Time Elapsed | Description |
|---|---|---|---|---|
| **5%** → 10% | ≥ 95% | 50 | 30 min | Canary — early detection of systemic issues |
| **10% → 30%** | ≥ 95% | 100 | 30 min | Expanded canary — broader device diversity |
| **30% → 50%** | ≥ 97% | 500 | 1 hour | Growing confidence — significant fleet coverage |
| **50% → 100%** | ≥ 98% | 1000 | 2 hours | Final push — remaining devices |

## Auto-Pause / Auto-Halt Conditions

| Condition | Action | Trigger |
|---|---|---|
| **Success rate < 90%** at any stage | Auto-pause | Immediate threshold breach |
| **Any CRITICAL severity report** | Auto-pause | Single critical error (boot loop, bricked device) |
| **Success rate < 70%** | Auto-halt | Severe failure — stop all new deployments |
| **> 5 devices boot-looping** | Auto-halt | Catastrophic — immediate full stop |

## Rollback Mechanism

1. **A/B Partition Fallback**: Devices with uncommitted new slots automatically boot back to the old slot
2. **Explicit Rollback Command**: Server sends `POST /api/v1/devices/{id}/rollback` → device marks old slot as active
3. **Forced Rollback**: For already-committed updates, a separate rollback campaign can be created targeting affected devices with the previous artifact version

## State Transition Table

| From | To | Trigger | Side Effects |
|---|---|---|---|
| Draft | Active | Operator starts rollout | Scheduler begins selecting devices |
| Active (5%) | Active (10%) | Gate criteria met | Next 5% of devices selected |
| Active | Paused | Operator / auto-pause | No new device selections |
| Paused | Active | Operator resumes | Scheduler resumes selections |
| Active/Paused | Halted | Operator / auto-halt | Campaign frozen, in-flight continues |
| Active/Paused | Rollback | Operator triggers | Rollback command sent to updated devices |
| Active (100%) | Completed | All devices reported | Campaign archived |
| Rollback | Completed | Rollback confirmed | Campaign archived |
