export interface User {
  id: string;
  email: string;
  display_name: string;
  avatar_url: string | null;
  roles: string[];
  permissions: string[];
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  slug: string;
  description: string;
  board: string;
  created_at: string;
  device_count: number;
  release_count: number;
}

export interface Device {
  id: string;
  device_id: string;
  board: string;
  firmware_version: string;
  hardware_revision: string;
  serial_number: string;
  status: DeviceConnectionStatus;
  last_seen: string | null;
  ip_address: string | null;
  groups: string[];
  labels: Record<string, string>;
  created_at: string;
}

export type DeviceConnectionStatus =
  | "online"
  | "offline"
  | "unknown"
  | "updating";

export interface Release {
  id: string;
  project_id: string;
  version: string;
  file_url: string;
  file_hash: string;
  file_size_bytes: number;
  changelog: string;
  target_board: string;
  firmware_version: string;
  status: ReleaseStatus;
  created_at: string;
  created_by: string;
}

export type ReleaseStatus =
  | "draft"
  | "published"
  | "deploying"
  | "deployed"
  | "rolled_back"
  | "failed";

export interface Deployment {
  id: string;
  release_id: string;
  project_id: string;
  group_ids: string[];
  strategy: DeploymentStrategy;
  rollout_percentage: number;
  staged: boolean;
  status: DeploymentStatus;
  created_at: string;
  completed_at: string | null;
  created_by: string;
}

export type DeploymentStrategy = "rolling" | "canary" | "blue_green";

export type DeploymentStatus =
  | "pending"
  | "in_progress"
  | "completed"
  | "failed"
  | "rolled_back"
  | "paused";

export interface GroupDetails extends Group {
  devices: Device[];
  active_deployment: Deployment | null;
}

export interface Group {
  id: string;
  project_id: string;
  name: string;
  description: string;
  device_count: number;
  labels: Record<string, string>;
  created_at: string;
}
