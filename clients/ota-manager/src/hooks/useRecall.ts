// Adapter shim: the recall dialog collects a target release id + a reason
// string; map onto the wire RecallRequest ({ to_release_id, reason? }) —
// server/internal/api/handlers_recall.go:17-20 (RecallRequest struct); see
// RecallRequest's own comment in api-client.ts and
// docs/qa/20260710-client-request-body-audit/EVIDENCE.md (this shim
// previously built a fabricated `{reason, force}` body — the server has no
// `force` field and the real required `to_release_id` was entirely absent,
// so the recall endpoint rejected EVERY request this client sent).
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { deploymentKeys } from "./use-deployments";
import { apiPost } from "../lib/api-client";
import type { RollbackList, RecallRequest } from "../types/api";

export interface RecallInput {
  deploymentId: string;
  toReleaseId: string;
  reason?: string;
}

export function useRecall() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RecallInput) =>
      apiPost<RollbackList["items"][number]>(`/deployments/${input.deploymentId}/recall`, {
        to_release_id: input.toReleaseId,
        reason: input.reason || undefined,
      } satisfies RecallRequest),
    onSuccess: (_data, input) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.detail(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollbacks(input.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
    },
  });
}
