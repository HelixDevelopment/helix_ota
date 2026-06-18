// ──────────────────────────────────────────────────────────
// Project hooks — multi-project abstraction layer.
// These hook into a future project-aware endpoint; the stub
// returns a single default project until the server supports it.
// ──────────────────────────────────────────────────────────

import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { Project } from '../types/api';
import { apiGet } from '../lib/api-client';

export const projectKeys = {
  all: ['projects'] as const,
  lists: () => [...projectKeys.all, 'list'] as const,
  list: () => [...projectKeys.lists()] as const,
  details: () => [...projectKeys.all, 'detail'] as const,
  detail: (id: string) => [...projectKeys.details(), id] as const,
};

// ── List projects ──────────────────────────────

export function useProjects() {
  return useQuery({
    queryKey: projectKeys.list(),
    queryFn: () => apiGet<Project[]>('/projects'),
    // Default stub when the endpoint does not exist yet: return a
    // single-project array. Remove `placeholderData` once the server
    // implements the endpoint.
    placeholderData: [
      {
        project_id: 'default',
        name: 'Default Project',
        description: 'Single-project mode — all resources live here.',
        os_types: ['android'],
        created_at: new Date().toISOString(),
      },
    ],
    // Retry once silently; on 404 the placeholderData kicks in.
    retry: (failureCount, error) => {
      if (error && 'status' in error && (error as { status: number }).status === 404) {
        return false;
      }
      return failureCount < 1;
    },
  });
}

// ── Get single project ─────────────────────────

export function useProject(projectId: string) {
  const queryClient = useQueryClient();

  return useQuery({
    queryKey: projectKeys.detail(projectId),
    queryFn: () => apiGet<Project>(`/projects/${projectId}`),
    enabled: !!projectId,
    // Fall back to the cached list entry when the detail endpoint is absent.
    placeholderData: () => {
      const projects = queryClient.getQueryData<Project[]>(projectKeys.list());
      return projects?.find((p) => p.project_id === projectId);
    },
  });
}
