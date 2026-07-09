// Adapter shim: the create-deployment dialog collects a release *version* string
// and a single target group; map those onto the wire CreateDeploymentRequest
// (server/internal/api/wire.go:167-171 DeploymentCreate: `{release_id,
// strategy, group?}` — a single optional group name, never a `group_ids`
// array, and no `rollout_percentage`/`staged` field on this request; see
// CreateDeploymentRequest's own comment in api-client.ts and
// docs/qa/20260710-client-request-body-audit/EVIDENCE.md).
//
// KNOWN GAP (§11.4.6, discovered — not fixed — in the request-body
// wire-shape audit): `handleCreateDeployment`
// (server/internal/api/handlers_deployment.go:39-42) accepts ONLY the
// literal strategy `"all-targets"` for this MVP; every value this dialog's
// picker offers (rolling/blue_green/canary) is a 400 VALIDATION_FAILED. That
// is a business-rule/value-domain mismatch, not a JSON structural drift —
// the wire SHAPE built below is now correct; the strategy-picker redesign
// needed to make deployment creation succeed end-to-end is a separate,
// out-of-scope work item.
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

function toWireStrategy(strategy: CreateDeploymentInput["strategy"]): string {
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
        strategy: toWireStrategy(input.strategy),
        group: input.targetGroupId || undefined,
      } satisfies CreateDeploymentRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
    },
  });
}
