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

const IN_FLIGHT_UPDATE_STATES = ["download_started", "installing", "installed", "verifying"];

// deriveViewStatus maps the REAL DeviceListItem fields (`health_ok` +
// `update_state`) onto the view's connectivity/update-progress vocabulary.
// (§11.4.6/§11.4.108: the previous `toDeviceStatus` matched the raw `status`
// field against the view's own enum literals — but `status` never existed on
// the real wire (see api-client.ts DeviceListItem); every device rendered
// "unknown" at runtime. `health_ok` + `update_state` are the real,
// server-sent signals this mapping is derived from.)
function deriveViewStatus(healthOk: boolean, updateState: string): DeviceViewStatus {
  if (!healthOk) return "error";
  if (IN_FLIGHT_UPDATE_STATES.includes(updateState)) return "updating";
  if (updateState === "failure") return "error";
  return "online";
}

export function useDevices(filters?: DevicesFilter) {
  const query = useDevicesQuery({
    os: filters?.os || undefined,
    status: filters?.status || undefined,
    model: filters?.targetModel || undefined,
  });

  let devices: DeviceView[] = (query.data?.items ?? []).map((d) => ({
    id: d.device_id,
    hardwareId: d.hardware_id,
    os: d.os,
    version: d.current_version || "—",
    status: deriveViewStatus(d.health_ok, d.update_state),
    lastSeen: d.last_seen ?? "",
    // targetModel here is the device's hardware `model` (matching the
    // `targetModel` filter above, which maps to the server's `model` query
    // param) — the real DeviceListItem field, never populated before this fix.
    targetModel: d.model || undefined,
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
