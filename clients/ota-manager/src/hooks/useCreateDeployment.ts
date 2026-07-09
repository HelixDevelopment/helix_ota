// Adapter shim: the create-deployment dialog collects a release *version* string
// and a single target group; map those onto the wire CreateDeploymentRequest.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deploymentKeys } from "./use-deployments";
import { apiPost } from "../lib/api-client";
import type { Deployment as WireDeployment, CreateDeploymentRequest } from "../types/api";

export interface CreateDeploymentInput {
  targetGroupId: string;
  releaseVersion: string;
  strategy: "rolling" | "blue-green" | "canary" | "full";
  percentage?: number;
}

function toWireStrategy(strategy: CreateDeploymentInput["strategy"]): CreateDeploymentRequest["strategy"] {
  switch (strategy) {
    case "canary":
      return "canary";
    case "blue-green":
      return "blue_green";
    default:
      // "rolling" and "full" both map to the wire "rolling" strategy.
      return "rolling";
  }
}

export function useCreateDeployment() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateDeploymentInput) =>
      apiPost<WireDeployment>("/deployments", {
        release_id: input.releaseVersion,
        group_ids: [input.targetGroupId],
        strategy: toWireStrategy(input.strategy),
        rollout_percentage: input.percentage,
        staged: input.strategy === "canary",
      } satisfies CreateDeploymentRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
    },
  });
}
