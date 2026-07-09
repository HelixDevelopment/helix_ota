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

  const data: Release[] = (query.data?.items ?? []).map((r) => ({
    id: r.id,
    version: r.version,
    os: r.firmware_version,
    targetModel: r.target_board,
    status: r.status,
    createdAt: r.created_at,
  }));

  return { ...query, data };
}
