// ──────────────────────────────────────────────────────────
// Artifact hooks — upload, metadata, delta register/lookup.
// ──────────────────────────────────────────────────────────

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type {
  Artifact,
  ArtifactUploadMetadata,
  DeltaRegisterRequest,
  DeltaView,
  DeltaFindParams,
} from '../types/api';
import { apiGet, apiPost, apiMultipartPost } from '../lib/api-client';

export const artifactKeys = {
  all: ['artifacts'] as const,
  details: () => [...artifactKeys.all, 'detail'] as const,
  detail: (id: string) => [...artifactKeys.details(), id] as const,
  deltas: () => [...artifactKeys.all, 'deltas'] as const,
  deltaFind: (params: DeltaFindParams) => [...artifactKeys.deltas(), params] as const,
};

// ── Get artifact metadata ──────────────────────

export function useArtifact(artifactId: string) {
  return useQuery({
    queryKey: artifactKeys.detail(artifactId),
    queryFn: () => apiGet<Artifact>(`/artifacts/${artifactId}`),
    enabled: !!artifactId,
  });
}

// ── Upload artifact (multipart) ────────────────

export function useUploadArtifact() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (vars: { file: Blob; metadata: ArtifactUploadMetadata; sha256?: Blob; signature?: Blob }) => {
      const formData = new FormData();
      formData.append('file', vars.file);
      formData.append('metadata', new Blob([JSON.stringify(vars.metadata)], { type: 'application/json' }));
      if (vars.sha256) formData.append('sha256', vars.sha256);
      if (vars.signature) formData.append('signature', vars.signature);
      return apiMultipartPost<Artifact>('/artifacts/upload', formData);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(artifactKeys.detail(data.artifact_id), data);
    },
  });
}

// ── Register delta ─────────────────────────────

export function useRegisterDelta() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (req: DeltaRegisterRequest) =>
      apiPost<DeltaView>('/deltas', req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: artifactKeys.deltas() });
    },
  });
}

// ── Find delta (query string lookup) ────────────

export function useDeltas(findParams?: DeltaFindParams) {
  return useQuery({
    queryKey: artifactKeys.deltaFind(findParams ?? { base: '', target: '' }),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (findParams) {
        params.base = findParams.base;
        params.target = findParams.target;
      }
      return apiGet<DeltaView>('/deltas', { params });
    },
    enabled: !!findParams && !!findParams.base && !!findParams.target,
  });
}
