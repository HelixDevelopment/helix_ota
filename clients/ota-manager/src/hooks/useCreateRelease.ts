// Adapter shim: the create-release wizard collects an artifact selection plus
// version/os/target-model. Map those onto the wire CreateReleaseRequest
// (server/internal/api/wire.go:137-144 ReleaseCreate: `{artifact_id,
// version, os, target_model, notes?, min_current_version?}`; see
// CreateReleaseRequest's own comment in api-client.ts and
// docs/qa/20260710-client-request-body-audit/EVIDENCE.md — this shim
// previously built a fabricated `{project_id, file_url, file_hash,
// changelog, target_board, firmware_version}` body that matched none of the
// real request fields and omitted the real required `artifact_id`).
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { releaseKeys } from "./use-releases";
import { apiPost } from "../lib/api-client";
import type { Release, CreateReleaseRequest } from "../types/api";

export interface CreateReleaseInput {
  artifactId: string;
  version: string;
  os: string;
  targetModel: string;
  notes?: string;
  minCurrentVersion?: string;
}

export function useCreateRelease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateReleaseInput) =>
      apiPost<Release>("/releases", {
        artifact_id: input.artifactId,
        version: input.version,
        os: input.os,
        target_model: input.targetModel,
        notes: input.notes,
        min_current_version: input.minCurrentVersion,
      } satisfies CreateReleaseRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: releaseKeys.lists() });
    },
  });
}
