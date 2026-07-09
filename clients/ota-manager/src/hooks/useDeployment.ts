// Adapter shim: maps the react-query single-deployment result (DeploymentStatus)
// onto the view shape the DeploymentDetailPage consumes. Accepts either a bare id
// or a { deploymentId } object plus optional options (the underlying query already
// polls); fields the wire contract does not carry (target group name, per-device
// stats, rollback history) are left absent and rendered as honest placeholders by
// the page.
import { useDeployment as useDeploymentQuery } from "./use-deployments";

export interface Deployment {
  id: string;
  strategy: string;
  status: string;
  createdAt: string;
  createdBy: string;
  rolloutPercentage: number;
  releaseVersion: string;
  targetGroupName?: string;
}

export function useDeployment(
  arg: string | { deploymentId: string },
  _options?: { refetchInterval?: number },
) {
  const deploymentId = typeof arg === "string" ? arg : arg.deploymentId;
  const query = useDeploymentQuery(deploymentId);

  const data: Deployment | undefined = query.data
    ? {
        id: query.data.deployment_id,
        strategy: query.data.strategy,
        status: query.data.status,
        createdAt: query.data.created_at,
        createdBy: query.data.created_by,
        rolloutPercentage: query.data.rollout_percentage,
        releaseVersion: query.data.release_id,
        targetGroupName: query.data.group_ids[0],
      }
    : undefined;

  return { ...query, data };
}
