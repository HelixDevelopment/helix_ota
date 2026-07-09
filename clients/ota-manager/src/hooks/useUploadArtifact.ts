// Adapter shim: exposes the real upload-artifact mutation plus an artifact
// option list for the create-release picker.
//
// KNOWN GAP (§11.4.6): the server exposes no list-artifacts endpoint, so the
// picker has no source and the option list is empty until one is added. The
// picker renders its "No artifacts available" empty state honestly. See
// EVIDENCE.md.
import { useUploadArtifact as useUploadArtifactMutation } from "./use-artifacts";

export interface ArtifactOption {
  id: string;
  name: string;
}

export function useUploadArtifact() {
  const mutation = useUploadArtifactMutation();
  const data: ArtifactOption[] = [];
  return { ...mutation, data };
}
