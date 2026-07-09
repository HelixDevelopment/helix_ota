// Adapter shim: the deployment-detail "Rollout Control" panel drives a single
// rollout-percentage value, so map its { deploymentId, percentage } input onto
// the create-rollout endpoint (POST /deployments/:id/rollout), whose real
// wire body is `{phases}` — server/internal/api/handlers_rollout.go:24-27
// (RolloutCreate struct); see CreateRolloutRequest's own comment in
// api-client.ts and docs/qa/20260710-client-request-body-audit/EVIDENCE.md
// (this shim previously built a fabricated `{deployment_id, groups,
// rollout_percentage, staged}` body that matched none of the real request
// fields and omitted the real required `phases` array).
//
// NOTE (§11.4.6): the raw /rollout/evaluate endpoint is a health-gate that needs
// success_rate/error_rate/post_boot_health_failed, which the percentage-only
// panel does not collect. The panel's action is "set the rollout percentage",
// which maps to the create-rollout request as a single phase. Target groups are
// applied server-side from the deployment; the panel does not re-specify them.
//
// KNOWN GAP (§11.4.6, discovered — not fixed — in the request-body
// wire-shape audit): the rollout-engine brick validates phase PLANS as
// strictly-increasing percentages ENDING AT 100 (handlers_rollout.go:80-81
// comment). This panel collects a single percentage with no phase-plan UI,
// so a submission below 100% builds a technically-correct WIRE SHAPE (now
// matching RolloutCreate exactly) that the engine may still reject as an
// invalid phase plan. Fixing that business-rule gap (a real phase-plan
// builder UI) is out of scope for this wire-shape audit.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deploymentKeys } from "./use-deployments";
import { apiPost } from "../lib/api-client";
import type { RolloutState, CreateRolloutRequest } from "../types/api";

export interface EvaluateRolloutInput {
  deploymentId: string;
  percentage: number;
}

export function useEvaluateRollout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: EvaluateRolloutInput) =>
      apiPost<RolloutState>(`/deployments/${input.deploymentId}/rollout`, {
        phases: [
          {
            percentage: input.percentage,
            success_threshold: 0.95,
            error_threshold: 0.05,
            duration_seconds: 0,
            auto_progress: false,
          },
        ],
      } satisfies CreateRolloutRequest),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollout(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.detail(input.deploymentId) });
    },
  });
}
