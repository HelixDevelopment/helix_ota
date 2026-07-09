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

  const data: Deployment[] = (query.data?.items ?? []).map((d) => ({
    id: d.deployment_id,
    releaseVersion: d.release_id,
    targetGroupName: d.group_ids[0],
    strategy: d.strategy,
    status: d.status,
    createdAt: d.created_at,
  }));

  return { ...query, data };
}
