// Adapter shim: maps the react-query device-list result onto the view shape
// (`{ devices, loading, error, refresh }`) the DevicesPage consumes.
import { useDevices as useDevicesQuery } from "./use-devices";

export {
  useTelemetryOverview,
  useDeviceStatus,
  useRegisterDevice,
  useDeviceTelemetry,
  deviceKeys,
} from "./use-devices";

export type DeviceViewStatus = "online" | "offline" | "updating" | "error" | "unknown";

export interface DeviceView {
  id: string;
  hardwareId: string;
  os: string;
  version: string;
  status: DeviceViewStatus;
  lastSeen: string;
  targetModel?: string;
}

export interface DevicesFilter {
  search?: string;
  os?: string;
  status?: string;
  targetModel?: string;
}

const KNOWN_STATUSES: DeviceViewStatus[] = ["online", "offline", "updating", "error", "unknown"];

function toDeviceStatus(raw: string): DeviceViewStatus {
  return (KNOWN_STATUSES as string[]).includes(raw) ? (raw as DeviceViewStatus) : "unknown";
}

export function useDevices(filters?: DevicesFilter) {
  const query = useDevicesQuery({
    os: filters?.os || undefined,
    status: filters?.status || undefined,
    model: filters?.targetModel || undefined,
  });

  let devices: DeviceView[] = (query.data?.items ?? []).map((d) => ({
    id: d.device_id,
    hardwareId: d.id,
    os: d.board,
    version: "—",
    status: toDeviceStatus(d.status),
    lastSeen: d.created_at,
  }));

  const search = filters?.search?.trim().toLowerCase();
  if (search) {
    devices = devices.filter(
      (d) => d.id.toLowerCase().includes(search) || d.hardwareId.toLowerCase().includes(search),
    );
  }

  return {
    devices,
    loading: query.isLoading,
    error: query.error ? query.error.message : null,
    refresh: query.refetch,
  };
}
