// Adapter shim: the create-group dialog collects only name + description,
// matching the wire CreateGroupRequest exactly — server/internal/api/
// handlers_group.go:42-45 (GroupCreate struct): `{name, description?}` ONLY.
// (§11.4.6/§11.4.115/§11.4.108 — request-body wire-shape audit,
// docs/qa/20260710-client-request-body-audit/EVIDENCE.md: this shim
// previously sent additional `device_ids: []` / `labels: {}` fields the
// server has no concept of on group creation — Go's JSON unmarshal silently
// ignored them, so this was a silent-data-loss bug rather than a rejected
// request. Group membership is a separate endpoint, see useAddGroupMember.)
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { groupKeys } from "./use-groups";
import { apiPost } from "../lib/api-client";
import type { Group as WireGroup, CreateGroupRequest } from "../types/api";

export interface CreateGroupInput {
  name: string;
  description?: string;
}

export function useCreateGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateGroupInput) =>
      apiPost<WireGroup>("/groups", {
        name: input.name,
        description: input.description,
      } satisfies CreateGroupRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.lists() });
    },
  });
}
