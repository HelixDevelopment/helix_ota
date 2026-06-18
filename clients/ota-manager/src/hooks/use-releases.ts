// ──────────────────────────────────────────────────────────
// Release hooks — create, list, get by id.
// ──────────────────────────────────────────────────────────

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type {
  Release,
  ReleaseList,
  ReleaseFilter,
  CreateReleaseRequest,
} from '../types/api';
import { apiGet, apiPost } from '../lib/api-client';

export const releaseKeys = {
  all: ['releases'] as const,
  lists: () => [...releaseKeys.all, 'list'] as const,
  list: (filters?: ReleaseFilter) => [...releaseKeys.lists(), filters] as const,
  details: () => [...releaseKeys.all, 'detail'] as const,
  detail: (id: string) => [...releaseKeys.details(), id] as const,
};

// ── List releases ──────────────────────────────

export function useReleases(filters?: ReleaseFilter) {
  return useQuery({
    queryKey: releaseKeys.list(filters),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (filters?.os) params.os = filters.os;
      if (filters?.target_model) params['target_model'] = filters.target_model;
      if (filters?.status) params.status = filters.status;
      if (filters?.cursor) params.cursor = filters.cursor;
      if (filters?.limit !== undefined) params.limit = filters.limit;
      return apiGet<ReleaseList>('/releases', { params });
    },
  });
}

// ── Get single release ─────────────────────────

export function useRelease(releaseId: string) {
  return useQuery({
    queryKey: releaseKeys.detail(releaseId),
    queryFn: () => apiGet<Release>(`/releases/${releaseId}`),
    enabled: !!releaseId,
  });
}

// ── Create release ─────────────────────────────

export function useCreateRelease() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (req: CreateReleaseRequest) =>
      apiPost<Release>('/releases', req),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: releaseKeys.lists() });
      queryClient.setQueryData(releaseKeys.detail(data.release_id), data);
    },
  });
}
