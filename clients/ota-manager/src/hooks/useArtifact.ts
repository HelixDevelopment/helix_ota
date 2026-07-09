// Adapter shim: allow the create-release wizard to call useArtifact() with no
// argument before an artifact is selected (empty id => query disabled).
import { useArtifact as useArtifactQuery } from "./use-artifacts";

export function useArtifact(artifactId?: string) {
  return useArtifactQuery(artifactId ?? "");
}
