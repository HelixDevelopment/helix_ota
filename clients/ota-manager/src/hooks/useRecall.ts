// Adapter shim: the recall dialog collects a reason string; map it onto the wire
// RecallRequest ({ reason, force }).
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deploymentKeys } from "./use-deployments";
import { apiPost } from "../lib/api-client";
import type { RollbackList, RecallRequest } from "../types/api";

export interface RecallInput {
  deploymentId: string;
  reason: string;
  force?: boolean;
}

export function useRecall() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RecallInput) =>
      apiPost<RollbackList["items"][number]>(`/deployments/${input.deploymentId}/recall`, {
        reason: input.reason,
        force: input.force ?? false,
      } satisfies RecallRequest),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.detail(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollbacks(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
    },
  });
}
