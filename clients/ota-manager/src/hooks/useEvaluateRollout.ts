// Adapter shim: the deployment-detail "Rollout Control" panel drives a single
// rollout-percentage value, so map its { deploymentId, percentage } input onto
// the create/update-rollout endpoint (POST /deployments/:id/rollout).
//
// NOTE (§11.4.6): the raw /rollout/evaluate endpoint is a health-gate that needs
// success_rate/error_rate/post_boot_health_failed, which the percentage-only
// panel does not collect. The panel's action is "set the rollout percentage",
// which maps to the create-rollout request. Target groups are applied
// server-side from the deployment; the panel does not re-specify them.
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
        deployment_id: input.deploymentId,
        groups: [],
        rollout_percentage: input.percentage,
        staged: false,
      } satisfies CreateRolloutRequest),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollout(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.detail(input.deploymentId) });
    },
  });
}
