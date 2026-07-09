import type { ColumnDef } from "@tanstack/react-table";
import { useParams } from "@tanstack/react-router";
import { useDeviceStatus } from "@/hooks/useDeviceStatus";
import { useDeviceTelemetry } from "@/hooks/useDeviceTelemetry";
import { DataTable } from "@/components/data-table/data-table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type DeviceStatus = "online" | "offline" | "updating" | "error" | "unknown";

interface DeviceInfo {
  id: string;
  hardwareId: string;
  version: string;
  status: DeviceStatus;
  lastSeen: string;
  // Fields below are not carried by the REAL device-status wire contract —
  // server/internal/api/wire.go:69-78 (DeviceStatus struct) returns
  // device_id/hardware_id/current_version/target_version/last_seen/
  // update_state/active_slot/health only (§11.4.6/§11.4.108: the previous
  // comment here described a FABRICATED contract —
  // device_id/online/current_release_id/last_seen/ip_address — that the
  // server never actually sent). `currentSlot` and `healthStatus` ARE real
  // (sourced from `active_slot` / `health.ok` below); the rest remain
  // optional and render as honest placeholders until a richer device-detail
  // endpoint exists.
  os?: string;
  buildId?: string;
  firstSeen?: string;
  currentSlot?: "a" | "b";
  bootSlot?: "a" | "b";
  availableUpdate?: string;
  healthStatus?: "healthy" | "degraded" | "critical";
  targetModel?: string;
  imei?: string;
  serial?: string;
}

interface TelemetryEntry {
  timestamp: string;
  cpu: number;
  memory: number;
  temperature: number;
  signalStrength?: number;
  diskUsage?: number;
}

interface UpdateEvent {
  version: string;
  status: "success" | "failure" | "rolled-back" | "in-progress";
  timestamp: string;
  duration: string;
}

interface GroupInfo {
  id: string;
  name: string;
  rolloutPercent: number;
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const statusBadgeVariant: Record<DeviceStatus, "default" | "secondary" | "outline" | "destructive"> =
  {
    online: "default",
    offline: "secondary",
    updating: "outline",
    error: "destructive",
    unknown: "secondary",
  };

const statusLabel: Record<DeviceStatus, string> = {
  online: "Online",
  offline: "Offline",
  updating: "Updating",
  error: "Error",
  unknown: "Unknown",
};

function formatDateTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat("en-US", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function formatPercent(value: number): string {
  return `${Math.round(value)}%`;
}

/* ------------------------------------------------------------------ */
/*  Telemetry columns                                                  */
/* ------------------------------------------------------------------ */

const telemetryColumns: ColumnDef<TelemetryEntry>[] = [
  {
    accessorKey: "timestamp",
    header: "Time",
    cell: ({ row }) => formatDateTime(row.getValue("timestamp")),
  },
  {
    accessorKey: "cpu",
    header: "CPU %",
    cell: ({ row }) => formatPercent(row.getValue("cpu")),
  },
  {
    accessorKey: "memory",
    header: "Memory %",
    cell: ({ row }) => formatPercent(row.getValue("memory")),
  },
  {
    accessorKey: "temperature",
    header: "Temp (°C)",
    cell: ({ row }) => `${(row.getValue("temperature") as number).toFixed(1)}`,
  },
  {
    accessorKey: "signalStrength",
    header: "Signal",
    cell: ({ row }) => {
      const val = row.getValue<number | undefined>("signalStrength");
      return val !== undefined ? formatPercent(val) : "—";
    },
  },
  {
    accessorKey: "diskUsage",
    header: "Disk %",
    cell: ({ row }) => {
      const val = row.getValue<number | undefined>("diskUsage");
      return val !== undefined ? formatPercent(val) : "—";
    },
  },
];

/* ------------------------------------------------------------------ */
/*  Update history columns                                             */
/* ------------------------------------------------------------------ */

const updateHistoryColumns: ColumnDef<UpdateEvent>[] = [
  {
    accessorKey: "version",
    header: "Version",
    cell: ({ row }) => (
      <span className="font-mono text-xs">{row.getValue("version")}</span>
    ),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => {
      const status = row.getValue<UpdateEvent["status"]>("status");
      const variant: Record<string, "default" | "secondary" | "outline" | "destructive"> = {
        success: "default",
        failure: "destructive",
        "rolled-back": "outline",
        "in-progress": "secondary",
      };
      return (
        <Badge variant={variant[status] ?? "secondary"}>
          {status === "in-progress" ? "In Progress" : status.charAt(0).toUpperCase() + status.slice(1)}
        </Badge>
      );
    },
  },
  {
    accessorKey: "timestamp",
    header: "Date",
    cell: ({ row }) => formatDateTime(row.getValue("timestamp")),
  },
  {
    accessorKey: "duration",
    header: "Duration",
  },
];

/* ------------------------------------------------------------------ */
/*  Loading skeleton                                                   */
/* ------------------------------------------------------------------ */

function LoadingSkeleton() {
  return (
    <div className="space-y-6 p-6">
      <Skeleton className="h-8 w-72" />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-40 w-full" />
        ))}
      </div>
      <Skeleton className="h-64 w-full" />
      <Skeleton className="h-48 w-full" />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Error state                                                        */
/* ------------------------------------------------------------------ */

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="mb-4 text-5xl text-destructive">❌</div>
      <h3 className="mb-2 text-lg font-semibold">Failed to Load Device</h3>
      <p className="mb-6 max-w-md text-sm text-muted-foreground">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function DeviceInfoCard({ device }: { device: DeviceInfo }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Device Information</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
        <div>
          <span className="text-muted-foreground">Device ID</span>
          <p className="font-mono text-xs">{device.id}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Hardware ID</span>
          <p className="font-mono text-xs">{device.hardwareId}</p>
        </div>
        <div>
          <span className="text-muted-foreground">OS</span>
          <p>{device.os ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Target Model</span>
          <p>{device.targetModel ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Serial</span>
          <p className="font-mono text-xs">{device.serial ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">IMEI</span>
          <p className="font-mono text-xs">{device.imei ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Status</span>
          <div>
            <Badge variant={statusBadgeVariant[device.status]}>
              {statusLabel[device.status]}
            </Badge>
          </div>
        </div>
        <div>
          <span className="text-muted-foreground">First Seen</span>
          <p>{device.firstSeen ? formatDateTime(device.firstSeen) : "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Last Seen</span>
          <p>{formatDateTime(device.lastSeen)}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Health</span>
          <Badge
            variant={
              device.healthStatus === "healthy"
                ? "default"
                : device.healthStatus === "degraded"
                  ? "outline"
                  : device.healthStatus === "critical"
                    ? "destructive"
                    : "secondary"
            }
          >
            {device.healthStatus ?? "Unknown"}
          </Badge>
        </div>
      </CardContent>
    </Card>
  );
}

function VersionCard({ device }: { device: DeviceInfo }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Version &amp; Update</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div>
          <span className="text-muted-foreground">Current Version</span>
          <p className="font-mono text-xs">{device.version}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Build ID</span>
          <p className="font-mono text-xs">{device.buildId ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Available Update</span>
          <p className="font-mono text-xs">{device.availableUpdate ?? "Up to date"}</p>
        </div>
      </CardContent>
    </Card>
  );
}

function SlotCard({ device }: { device: DeviceInfo }) {
  const slots: Array<{ name: string; active: boolean; boot: boolean }> = [
    {
      name: "Slot A",
      active: device.currentSlot === "a",
      boot: device.bootSlot === "a",
    },
    {
      name: "Slot B",
      active: device.currentSlot === "b",
      boot: device.bootSlot === "b",
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Slot (A/B)</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {slots.map((slot) => (
          <div
            key={slot.name}
            className={`rounded border px-4 py-3 text-sm ${
              slot.active
                ? "border-primary bg-primary/10"
                : "border-muted-foreground/20"
            }`}
          >
            <div className="flex items-center justify-between">
              <span className="font-medium">{slot.name}</span>
              {slot.active && (
                <Badge variant="default" className="text-[10px]">
                  Active
                </Badge>
              )}
            </div>
            {slot.boot && (
              <p className="mt-1 text-xs text-muted-foreground">
                Booted from this slot
              </p>
            )}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function TelemetryCard({
  telemetry,
  loading,
}: {
  telemetry: TelemetryEntry[];
  loading: boolean;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Telemetry History</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : telemetry.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            No telemetry data available for this device.
          </p>
        ) : (
          <DataTable columns={telemetryColumns} data={telemetry} />
        )}
      </CardContent>
    </Card>
  );
}

function UpdateHistoryCard({
  updates,
  loading,
}: {
  updates: UpdateEvent[];
  loading: boolean;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Update History</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : updates.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            No update history recorded.
          </p>
        ) : (
          <DataTable columns={updateHistoryColumns} data={updates} />
        )}
      </CardContent>
    </Card>
  );
}

function GroupsCard({ groups }: { groups: GroupInfo[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Group Membership</CardTitle>
      </CardHeader>
      <CardContent>
        {groups.length === 0 ? (
          <p className="py-4 text-sm text-muted-foreground">
            This device is not assigned to any group.
          </p>
        ) : (
          <ul className="space-y-3">
            {groups.map((group) => (
              <li
                key={group.id}
                className="flex items-center justify-between rounded border px-4 py-3 text-sm"
              >
                <div>
                  <p className="font-medium">{group.name}</p>
                  <p className="font-mono text-xs text-muted-foreground">{group.id}</p>
                </div>
                <Badge variant="outline">
                  Rollout: {group.rolloutPercent}%
                </Badge>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------------ */
/*  Page component                                                     */
/* ------------------------------------------------------------------ */

export default function DeviceDetailPage() {
  const { deviceId } = useParams({ from: "/layout/devices/$deviceId" });

  const statusQuery = useDeviceStatus(deviceId);
  const telemetryQuery = useDeviceTelemetry(deviceId);

  // Map the REAL device-status wire result (server/internal/api/wire.go:69-78)
  // onto the rich view model. Fields the real contract does not carry are
  // left undefined and shown as honest placeholders.
  const device: DeviceInfo | null = statusQuery.data
    ? {
        id: statusQuery.data.device_id,
        hardwareId: statusQuery.data.hardware_id,
        version: statusQuery.data.current_version || "—",
        status: !statusQuery.data.health.ok
          ? "error"
          : statusQuery.data.update_state === "download_started" ||
              statusQuery.data.update_state === "installing" ||
              statusQuery.data.update_state === "installed" ||
              statusQuery.data.update_state === "verifying"
            ? "updating"
            : statusQuery.data.update_state === "failure"
              ? "error"
              : "online",
        lastSeen: statusQuery.data.last_seen ?? "",
        currentSlot:
          statusQuery.data.active_slot === "a" || statusQuery.data.active_slot === "b"
            ? statusQuery.data.active_slot
            : undefined,
        healthStatus: statusQuery.data.health.ok ? "healthy" : "critical",
      }
    : null;

  // The wire contract exposes no per-device update history, group membership, or
  // typed telemetry series (telemetry items are opaque records), so these render
  // their empty states until richer endpoints exist (see EVIDENCE.md).
  const updates: UpdateEvent[] = [];
  const groups: GroupInfo[] = [];
  const telemetry: TelemetryEntry[] = [];

  const statusLoading = statusQuery.isLoading;
  const telemetryLoading = telemetryQuery.isLoading;
  const loading = statusLoading || telemetryLoading;
  const error = statusQuery.error?.message ?? telemetryQuery.error?.message ?? null;

  const handleRetry = () => {
    statusQuery.refetch();
    telemetryQuery.refetch();
  };

  /* ---- loading ---- */
  if (loading && !device) {
    return <LoadingSkeleton />;
  }

  /* ---- error ---- */
  if (error && !device) {
    return (
      <div className="p-6">
        <h1 className="mb-6 text-2xl font-bold tracking-tight">Device</h1>
        <ErrorState message={error} onRetry={handleRetry} />
      </div>
    );
  }

  /* ---- not found ---- */
  if (!device) {
    return (
      <div className="p-6">
        <h1 className="mb-6 text-2xl font-bold tracking-tight">Device</h1>
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <h3 className="mb-2 text-lg font-semibold">Device Not Found</h3>
          <p className="max-w-sm text-sm text-muted-foreground">
            The device <span className="font-mono">{deviceId}</span> could not be
            found. It may have been removed or the ID is incorrect.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      {/* Header */}
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{device.id}</h1>
          <p className="text-sm text-muted-foreground">
            <span className="font-mono">{device.hardwareId}</span> &middot;{" "}
            {device.os ?? "—"} &middot; {device.targetModel ?? "Generic"}
          </p>
        </div>

        <div className="flex gap-3">
          <Button variant="outline">Check Update</Button>
          <Button variant="secondary">Recall</Button>
        </div>
      </div>

      {/* Top cards */}
      <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-3">
        <DeviceInfoCard device={device} />
        <VersionCard device={device} />
        <SlotCard device={device} />
      </div>

      {/* Telemetry */}
      <div className="mb-6">
        <TelemetryCard
          telemetry={telemetry}
          loading={telemetryLoading}
        />
      </div>

      {/* Bottom row: update history + groups */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <UpdateHistoryCard updates={updates} loading={statusLoading} />
        <GroupsCard groups={groups} />
      </div>
    </div>
  );
}
