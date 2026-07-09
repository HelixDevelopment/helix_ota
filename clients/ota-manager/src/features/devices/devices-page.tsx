import { useState, useCallback } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { useDevices } from "@/hooks/useDevices";
import { DataTable } from "@/components/data-table/data-table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from "@/components/ui/sheet";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type DeviceStatus = "online" | "offline" | "updating" | "error" | "unknown";

interface Device {
  id: string;
  hardwareId: string;
  os: string;
  version: string;
  status: DeviceStatus;
  lastSeen: string;
  targetModel?: string;
  currentSlot?: "a" | "b";
  availableUpdate?: string;
  healthStatus?: "healthy" | "degraded" | "critical";
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

/* ------------------------------------------------------------------ */
/*  Columns                                                            */
/* ------------------------------------------------------------------ */

const columns: ColumnDef<Device>[] = [
  {
    accessorKey: "id",
    header: "Device ID",
    cell: ({ row }) => (
      <span className="font-mono text-xs">{row.getValue("id")}</span>
    ),
  },
  {
    accessorKey: "hardwareId",
    header: "Hardware ID",
    cell: ({ row }) => (
      <span className="font-mono text-xs">{row.getValue("hardwareId")}</span>
    ),
  },
  {
    accessorKey: "os",
    header: "OS",
  },
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
      const status = row.getValue<DeviceStatus>("status");
      return (
        <Badge variant={statusBadgeVariant[status] ?? "secondary"}>
          {statusLabel[status]}
        </Badge>
      );
    },
  },
  {
    accessorKey: "lastSeen",
    header: "Last Seen",
    cell: ({ row }) => formatDateTime(row.getValue("lastSeen")),
  },
  {
    id: "actions",
    header: "Actions",
    cell: () => (
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline">
          Check Update
        </Button>
        <Button size="sm" variant="outline">
          Send Telemetry
        </Button>
      </div>
    ),
  },
];

/* ------------------------------------------------------------------ */
/*  Empty state                                                        */
/* ------------------------------------------------------------------ */

function EmptyState({ onRefresh }: { onRefresh: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="mb-4 text-6xl text-muted-foreground">📡</div>
      <h3 className="mb-2 text-lg font-semibold">No Devices Registered</h3>
      <p className="mb-6 max-w-sm text-sm text-muted-foreground">
        No devices have connected to the OTA server yet. Once a device checks in
        it will appear here.
      </p>
      <Button variant="outline" onClick={onRefresh}>
        Refresh
      </Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Loading skeleton                                                   */
/* ------------------------------------------------------------------ */

function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-10 w-40" />
        <Skeleton className="h-10 w-40" />
      </div>
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-12 w-full" />
      ))}
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
      <h3 className="mb-2 text-lg font-semibold">Failed to Load Devices</h3>
      <p className="mb-6 max-w-md text-sm text-muted-foreground">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Device detail sheet                                                */
/* ------------------------------------------------------------------ */

interface DeviceDetailSheetProps {
  device: Device | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function DeviceDetailSheet({ device, open, onOpenChange }: DeviceDetailSheetProps) {
  if (!device) return null;

  const telemetryPoints = 12;
  const miniChart = Array.from({ length: telemetryPoints }, () =>
    Math.floor(Math.random() * 60 + 20),
  );
  const maxVal = Math.max(...miniChart, 1);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader className="mb-6">
          <SheetTitle>Device Details</SheetTitle>
          <SheetDescription>
            Full information and controls for <span className="font-mono">{device.id}</span>
          </SheetDescription>
        </SheetHeader>

        {/* Device Info Card */}
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-sm">Device Info</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <span className="text-muted-foreground">Hardware ID</span>
              <p className="font-mono text-xs">{device.hardwareId}</p>
            </div>
            <div>
              <span className="text-muted-foreground">OS</span>
              <p>{device.os}</p>
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
              <span className="text-muted-foreground">Last Seen</span>
              <p>{formatDateTime(device.lastSeen)}</p>
            </div>
          </CardContent>
        </Card>

        {/* Version + Update Card */}
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-sm">Version &amp; Update</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <span className="text-muted-foreground">Current Version</span>
              <p className="font-mono text-xs">{device.version}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Available Update</span>
              <p className="font-mono text-xs">
                {device.availableUpdate ?? "Up to date"}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Slot Info */}
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-sm">Slot (A/B)</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4 text-sm">
              <div
                className={`rounded border px-3 py-1 font-mono text-xs ${
                  device.currentSlot === "a"
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-muted-foreground/30 text-muted-foreground"
                }`}
              >
                Slot A
              </div>
              <div
                className={`rounded border px-3 py-1 font-mono text-xs ${
                  device.currentSlot === "b"
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-muted-foreground/30 text-muted-foreground"
                }`}
              >
                Slot B
              </div>
              {device.currentSlot && (
                <span className="text-xs text-muted-foreground">
                  Active: Slot {device.currentSlot.toUpperCase()}
                </span>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Telemetry History Mini-Chart */}
        <Card className="mb-4">
          <CardHeader>
            <CardTitle className="text-sm">Telemetry History (Last {telemetryPoints}h)</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-end gap-[3px]">
              {miniChart.map((value, i) => (
                <div
                  key={i}
                  className="w-full rounded-t bg-primary/40"
                  style={{
                    height: `${Math.max((value / maxVal) * 64, 4)}px`,
                  }}
                  title={`${value}%`}
                />
              ))}
            </div>
            <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
              <span>-{telemetryPoints}h</span>
              <span>Now</span>
            </div>
          </CardContent>
        </Card>

        {/* Health Status */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="text-sm">Health</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge
              variant={
                device.healthStatus === "healthy"
                  ? "default"
                  : device.healthStatus === "degraded"
                    ? "outline"
                    : "destructive"
              }
            >
              {device.healthStatus ?? "Unknown"}
            </Badge>
          </CardContent>
        </Card>

        {/* Action Buttons */}
        <div className="flex gap-3">
          <Button className="flex-1">Check Update</Button>
          <Button className="flex-1" variant="secondary">
            Send Telemetry
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}

/* ------------------------------------------------------------------ */
/*  Filter Bar                                                         */
/* ------------------------------------------------------------------ */

interface FilterBarProps {
  search: string;
  onSearchChange: (value: string) => void;
  osFilter: string;
  onOsFilterChange: (value: string) => void;
  statusFilter: string;
  onStatusFilterChange: (value: string) => void;
  modelFilter: string;
  onModelFilterChange: (value: string) => void;
}

function FilterBar({
  search,
  onSearchChange,
  osFilter,
  onOsFilterChange,
  statusFilter,
  onStatusFilterChange,
  modelFilter,
  onModelFilterChange,
}: FilterBarProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input
        placeholder="Search by Device ID…"
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        className="w-64"
      />
      <Input
        placeholder="OS Type"
        value={osFilter}
        onChange={(e) => onOsFilterChange(e.target.value)}
        className="w-40"
      />
      <Input
        placeholder="Status"
        value={statusFilter}
        onChange={(e) => onStatusFilterChange(e.target.value)}
        className="w-40"
      />
      <Input
        placeholder="Target Model"
        value={modelFilter}
        onChange={(e) => onModelFilterChange(e.target.value)}
        className="w-40"
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Page component                                                     */
/* ------------------------------------------------------------------ */

export default function DevicesPage() {
  const [search, setSearch] = useState("");
  const [osFilter, setOsFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [modelFilter, setModelFilter] = useState("");
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  const { devices, loading, error, refresh } = useDevices({
    search,
    os: osFilter,
    status: statusFilter,
    targetModel: modelFilter,
  });

  const handleRowClick = useCallback((device: Device) => {
    setSelectedDevice(device);
    setSheetOpen(true);
  }, []);

  /* ---- states ---- */
  if (loading) {
    return (
      <div className="p-6">
        <h1 className="mb-6 text-2xl font-bold tracking-tight">Devices</h1>
        <LoadingSkeleton />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <h1 className="mb-6 text-2xl font-bold tracking-tight">Devices</h1>
        <ErrorState message={error} onRetry={refresh} />
      </div>
    );
  }

  const isEmpty = !devices || devices.length === 0;

  return (
    <div className="p-6">
      <h1 className="mb-6 text-2xl font-bold tracking-tight">Devices</h1>

      {isEmpty ? (
        <EmptyState onRefresh={refresh} />
      ) : (
        <>
          <div className="mb-4">
            <FilterBar
              search={search}
              onSearchChange={setSearch}
              osFilter={osFilter}
              onOsFilterChange={setOsFilter}
              statusFilter={statusFilter}
              onStatusFilterChange={setStatusFilter}
              modelFilter={modelFilter}
              onModelFilterChange={setModelFilter}
            />
          </div>

          <DataTable
            columns={columns}
            data={devices}
            onRowClick={handleRowClick}
          />
        </>
      )}

      <DeviceDetailSheet
        device={selectedDevice}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
      />
    </div>
  );
}
