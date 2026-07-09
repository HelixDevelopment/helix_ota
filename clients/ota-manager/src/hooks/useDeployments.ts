// Adapter shim: maps the react-query deployment-list result onto the view shape
// the DeploymentsPage table consumes.
import { useDeployments as useDeploymentsQuery } from "./use-deployments";

export { useCreateRollout, useGetRollout, useEvaluateRollout, useRecall, useListRollbacks, deploymentKeys } from "./use-deployments";

export interface Deployment {
  id: string;
  targetGroupName?: string;
  releaseVersion?: string;
  strategy: string;
  status: string;
  deviceStats?: { succeeded: number; failed: number; pending: number };
  createdAt: string;
}

export function useDeployments(filters?: Record<string, string>) {
  const query = useDeploymentsQuery(filters);

  // d.group is the REAL single optional target-group field (Deployment has no
  // `group_ids` array on the wire — server/internal/api/wire.go:174-182).
  const data: Deployment[] = (query.data?.items ?? []).map((d) => ({
    id: d.deployment_id,
    releaseVersion: d.release_id,
    targetGroupName: d.group || undefined,
    strategy: d.strategy,
    status: d.status,
    createdAt: d.created_at,
  }));

  return { ...query, data };
}
