// Adapter shim: the create-group dialog collects only name + description; a new
// group starts with no devices/labels, so fill the remaining wire fields.
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
        description: input.description ?? "",
        device_ids: [],
        labels: {},
      } satisfies CreateGroupRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.lists() });
    },
  });
}
