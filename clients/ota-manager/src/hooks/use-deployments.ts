// ──────────────────────────────────────────────────────────
// Deployment hooks — CRUD + rollout + recall + rollbacks.
// ──────────────────────────────────────────────────────────

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type {
  Deployment,
  DeploymentList,
  DeploymentStatus,
  CreateDeploymentRequest,
  CreateRolloutRequest,
  RolloutDecision,
  RolloutState,
  RecallRequest,
  RollbackList,
} from '../types/api';
import { apiGet, apiPost } from '../lib/api-client';

export const deploymentKeys = {
  all: ['deployments'] as const,
  lists: () => [...deploymentKeys.all, 'list'] as const,
  list: (filters?: Record<string, unknown>) => [...deploymentKeys.lists(), filters] as const,
  details: () => [...deploymentKeys.all, 'detail'] as const,
  detail: (id: string) => [...deploymentKeys.details(), id] as const,
  rollout: (id: string) => [...deploymentKeys.all, 'rollout', id] as const,
  rollbacks: (id: string) => [...deploymentKeys.all, 'rollbacks', id] as const,
};

// ── List deployments ───────────────────────────

export function useDeployments(filters?: Record<string, string>) {
  return useQuery({
    queryKey: deploymentKeys.list(filters),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (filters?.status) params.status = filters.status;
      if (filters?.cursor) params.cursor = filters.cursor;
      if (filters?.limit) params.limit = Number(filters.limit);
      return apiGet<DeploymentList>('/deployments', { params });
    },
  });
}

// ── Get single deployment ──────────────────────

export function useDeployment(deploymentId: string) {
  return useQuery({
    queryKey: deploymentKeys.detail(deploymentId),
    queryFn: () => apiGet<DeploymentStatus>(`/deployments/${deploymentId}`),
    enabled: !!deploymentId,
    refetchInterval: 15_000, // progress updates every 15 s
  });
}

// ── Create deployment ──────────────────────────

export function useCreateDeployment() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (req: CreateDeploymentRequest) =>
      apiPost<Deployment>('/deployments', req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
      queryClient.setQueryData(deploymentKeys.detail(data.deployment_id), data);
    },
  });
}

// ── Create rollout ─────────────────────────────

export function useCreateRollout() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ deploymentId, req }: { deploymentId: string; req: CreateRolloutRequest }) =>
      apiPost<RolloutState>(`/deployments/${deploymentId}/rollout`, req),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollout(vars.deploymentId) });
    },
  });
}

// ── Get rollout state ──────────────────────────

export function useGetRollout(deploymentId: string) {
  return useQuery({
    queryKey: deploymentKeys.rollout(deploymentId),
    queryFn: () => apiGet<RolloutState>(`/deployments/${deploymentId}/rollout`),
    enabled: !!deploymentId,
  });
}

// ── Evaluate rollout ───────────────────────────

export function useEvaluateRollout() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { deploymentId: string; success_rate: number; error_rate: number; post_boot_health_failed: boolean }) =>
      apiPost<RolloutDecision>(`/deployments/${vars.deploymentId}/rollout/evaluate`, {
        success_rate: vars.success_rate,
        error_rate: vars.error_rate,
        post_boot_health_failed: vars.post_boot_health_failed,
      }),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollout(vars.deploymentId) });
    },
  });
}

// ── Recall deployment ──────────────────────────

export function useRecall() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { deploymentId: string; req: RecallRequest }) =>
      apiPost<RollbackList[`items`][number]>(`/deployments/${vars.deploymentId}/recall`, vars.req),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: deploymentKeys.detail(vars.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.rollbacks(vars.deploymentId) });
      queryClient.invalidateQueries({ queryKey: deploymentKeys.lists() });
    },
  });
}

// ── List rollbacks ─────────────────────────────

export function useListRollbacks(deploymentId: string) {
  return useQuery({
    queryKey: deploymentKeys.rollbacks(deploymentId),
    queryFn: () => apiGet<RollbackList>(`/deployments/${deploymentId}/rollbacks`),
    enabled: !!deploymentId,
  });
}
