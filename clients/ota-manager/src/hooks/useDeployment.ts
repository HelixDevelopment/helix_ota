// Adapter shim: maps the react-query single-deployment result (DeploymentStatus)
// onto the view shape the DeploymentDetailPage consumes. Accepts either a bare id
// or a { deploymentId } object plus optional options (the underlying query already
// polls); fields the wire contract does not carry (target group name, per-device
// stats, rollback history) are left absent and rendered as honest placeholders by
// the page.
import { useDeployment as useDeploymentQuery } from "./use-deployments";

// createdBy/rolloutPercentage are OPTIONAL: the real DeploymentStatus
// (server/internal/api/wire.go:202-205 = Deployment + progress) carries
// neither `created_by` nor a `rollout_percentage` — those concepts do not
// exist on Deployment itself (a staged rollout's percentage lives on the
// separate RolloutState resource). Previously typed as required with values
// sourced from fields that never existed on the wire (§11.4.6/§11.4.108).
export interface Deployment {
  id: string;
  strategy: string;
  status: string;
  createdAt: string;
  createdBy?: string;
  rolloutPercentage?: number;
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
        releaseVersion: query.data.release_id,
        targetGroupName: query.data.group || undefined,
      }
    : undefined;

  return { ...query, data };
}
