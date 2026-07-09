// ──────────────────────────────────────────────────────────
// Group hooks — CRUD + membership management.
// ──────────────────────────────────────────────────────────

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type {
  Group,
  GroupList,
  CreateGroupRequest,
  UpdateGroupRequest,
  AddGroupMembersRequest,
  AddGroupMembersResult,
  GroupMembers,
} from '../types/api';
import { apiGet, apiPost, apiPatch, apiDelete } from '../lib/api-client';

export const groupKeys = {
  all: ['groups'] as const,
  lists: () => [...groupKeys.all, 'list'] as const,
  list: (filters?: Record<string, string>) => [...groupKeys.lists(), filters] as const,
  details: () => [...groupKeys.all, 'detail'] as const,
  detail: (id: string) => [...groupKeys.details(), id] as const,
  members: (id: string) => [...groupKeys.all, 'members', id] as const,
};

// ── List groups ────────────────────────────────

export function useGroups() {
  return useQuery({
    queryKey: groupKeys.list(),
    queryFn: () => apiGet<GroupList>('/groups'),
  });
}

// ── Get single group ───────────────────────────

export function useGroup(groupId: string) {
  return useQuery({
    queryKey: groupKeys.detail(groupId),
    queryFn: () => apiGet<Group>(`/groups/${groupId}`),
    enabled: !!groupId,
  });
}

// ── Create group ───────────────────────────────

export function useCreateGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (req: CreateGroupRequest) =>
      apiPost<Group>('/groups', req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.lists() });
      queryClient.setQueryData(groupKeys.detail(data.id), data);
    },
  });
}

// ── Update group ───────────────────────────────

export function useUpdateGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { groupId: string; req: UpdateGroupRequest }) =>
      apiPatch<Group>(`/groups/${vars.groupId}`, vars.req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.lists() });
      queryClient.setQueryData(groupKeys.detail(data.id), data);
    },
  });
}

// ── Delete group ───────────────────────────────

export function useDeleteGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (groupId: string) =>
      apiDelete<void>(`/groups/${groupId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: groupKeys.lists() });
    },
  });
}

// ── List group members ─────────────────────────

export function useGroupMembers(groupId: string) {
  return useQuery({
    queryKey: groupKeys.members(groupId),
    queryFn: () => apiGet<GroupMembers>(`/groups/${groupId}/members`),
    enabled: !!groupId,
  });
}

// ── Add group members ──────────────────────────

export function useAddGroupMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { groupId: string; req: AddGroupMembersRequest }) =>
      apiPost<AddGroupMembersResult>(`/groups/${vars.groupId}/members`, vars.req),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.members(vars.groupId) });
      queryClient.invalidateQueries({ queryKey: groupKeys.detail(vars.groupId) });
    },
  });
}

// ── Remove group member ────────────────────────

export function useRemoveGroupMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { groupId: string; deviceId: string }) =>
      apiDelete<void>(`/groups/${vars.groupId}/members/${vars.deviceId}`),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: groupKeys.members(vars.groupId) });
      queryClient.invalidateQueries({ queryKey: groupKeys.detail(vars.groupId) });
    },
  });
}
