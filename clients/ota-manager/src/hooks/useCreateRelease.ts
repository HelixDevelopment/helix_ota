// Adapter shim: the create-release wizard collects an artifact selection plus
// version/os/target-model. Map those onto the wire CreateReleaseRequest.
//
// KNOWN GAP (§11.4.6): the server exposes no list-artifacts endpoint (only
// POST /artifacts/upload + GET /artifacts/:id), so the wizard cannot resolve a
// selected artifact into its file_url/file_hash, and there is no project picker
// yet. Those wire fields are therefore submitted empty until an artifact-list
// (and project-context) source lands — the create-release SUBMISSION is not yet
// end-to-end functional and is reported as deferred in EVIDENCE.md. The wizard
// itself is wired and renders; only its final submit is incomplete.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { releaseKeys } from "./use-releases";
import { apiPost } from "../lib/api-client";
import type { Release, CreateReleaseRequest } from "../types/api";

export interface CreateReleaseInput {
  artifactId: string;
  version: string;
  os: string;
  targetModel: string;
  projectId?: string;
  fileUrl?: string;
  fileHash?: string;
  changelog?: string;
}

export function useCreateRelease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateReleaseInput) =>
      apiPost<Release>("/releases", {
        project_id: input.projectId ?? "",
        version: input.version,
        file_url: input.fileUrl ?? "",
        file_hash: input.fileHash ?? "",
        changelog: input.changelog ?? "",
        target_board: input.targetModel,
        firmware_version: input.os,
      } satisfies CreateReleaseRequest),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: releaseKeys.lists() });
    },
  });
}
