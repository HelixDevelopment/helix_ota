// Adapter shim: maps the react-query release-list result onto the view shape
// the ReleasesPage table consumes.
import { useReleases as useReleasesQuery } from "./use-releases";

export { useCreateRelease, useRelease, releaseKeys } from "./use-releases";

export interface Release {
  id: string;
  version: string;
  os: string;
  targetModel: string;
  status: string;
  createdAt: string;
}

export interface ReleasesFilter {
  os?: string;
  targetModel?: string;
  status?: string;
}

export function useReleases(filters?: ReleasesFilter) {
  const query = useReleasesQuery({
    os: filters?.os || undefined,
    target_model: filters?.targetModel || undefined,
    status: filters?.status || undefined,
  });

  // r.release_id/r.os/r.target_model are the REAL Release fields —
  // server/internal/api/wire.go:147-156 (§11.4.6/§11.4.108: previously
  // `r.id`/`r.firmware_version`/`r.target_board`, none of which exist on the
  // wire).
  const data: Release[] = (query.data?.items ?? []).map((r) => ({
    id: r.release_id,
    version: r.version,
    os: r.os,
    targetModel: r.target_model,
    status: r.status,
    createdAt: r.created_at,
  }));

  return { ...query, data };
}
