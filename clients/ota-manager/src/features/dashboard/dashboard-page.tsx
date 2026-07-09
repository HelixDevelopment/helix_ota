import { useTelemetryOverview } from "@/hooks/useTelemetryOverview";
import { useAuditLog } from "@/hooks/useAuditLog";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useNavigate } from "@tanstack/react-router";
import {
  Monitor,
  Rocket,
  RefreshCw,
  AlertTriangle,
  Users,
  PackageOpen,
  ScrollText,
} from "lucide-react";

interface StatCardProps {
  title: string;
  value: number;
  icon: React.ReactNode;
  description: string;
  variant?: "default" | "destructive" | "warning";
}

function StatCard({ title, value, icon, description, variant = "default" }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <span
          className={
            variant === "destructive"
              ? "text-destructive"
              : variant === "warning"
                ? "text-amber-500"
                : "text-muted-foreground"
          }
        >
          {icon}
        </span>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value.toLocaleString()}</div>
        <p className="text-xs text-muted-foreground mt-1">{description}</p>
      </CardContent>
    </Card>
  );
}

function StatCardSkeleton() {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-5 w-5 rounded" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-8 w-16 mb-1" />
        <Skeleton className="h-3 w-32" />
      </CardContent>
    </Card>
  );
}

interface ActivityEvent {
  id: string;
  action: string;
  resource: string;
  resourceType: string;
  performedBy: string;
  timestamp: string;
  severity: "info" | "warning" | "error";
}

const severityVariant: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  info: "secondary",
  warning: "outline",
  error: "destructive",
};

function ActivityFeed({ events, loading }: { events: ActivityEvent[]; loading: boolean }) {
  if (loading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-start gap-3">
            <Skeleton className="h-2 w-2 mt-2 rounded-full shrink-0" />
            <div className="flex-1 space-y-1">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-3 w-1/2" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <ScrollText className="h-8 w-8 text-muted-foreground mb-2" />
        <p className="text-muted-foreground text-sm">No recent activity</p>
        <p className="text-muted-foreground text-xs mt-1">Events will appear here as they occur.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div key={event.id} className="flex items-start gap-3">
          <Badge
            variant={severityVariant[event.severity] ?? "secondary"}
            className="mt-0.5 h-5 w-5 rounded-full p-0 flex items-center justify-center shrink-0"
          >
            <span className="sr-only">{event.severity}</span>
            <span className="block h-1.5 w-1.5 rounded-full bg-current" />
          </Badge>
          <div className="flex-1 min-w-0">
            <p className="text-sm truncate">
              <span className="font-medium">{event.action}</span>
              {" on "}
              <span className="text-muted-foreground">{event.resource}</span>
            </p>
            <div className="flex items-center gap-2 mt-0.5">
              <span className="text-xs text-muted-foreground">
                {new Date(event.timestamp).toLocaleString()}
              </span>
              <span className="text-xs text-muted-foreground">by {event.performedBy}</span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export default function DashboardPage() {
  const navigate = useNavigate();
  const { data: overview, isLoading: statsLoading, isError: statsError, error: statsErrorObj, refetch: refetchStats } =
    useTelemetryOverview();
  const { data: auditEvents, isLoading: activityLoading } = useAuditLog({ limit: 10 });

  const activityEvents: ActivityEvent[] = (auditEvents ?? []).map((entry) => ({
    id: entry.id,
    action: entry.action,
    resource: entry.target,
    resourceType: "",
    performedBy: entry.actor,
    timestamp: entry.timestamp,
    severity: "info",
  }));

  // Loading skeleton state
  if (statsLoading) {
    return (
      <div className="space-y-6 p-6">
        <Skeleton className="h-8 w-48" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCardSkeleton />
          <StatCardSkeleton />
          <StatCardSkeleton />
          <StatCardSkeleton />
        </div>
        <div className="grid gap-6 lg:grid-cols-3">
          <Card className="lg:col-span-2">
            <CardHeader>
              <Skeleton className="h-5 w-32" />
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <Skeleton className="h-5 w-32" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-24 w-full" />
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }

  // Error state with retry button
  if (statsError) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 p-6 min-h-[50vh]">
        <AlertTriangle className="h-10 w-10 text-destructive" />
        <p className="text-destructive text-lg font-semibold">Failed to load dashboard data</p>
        <p className="text-muted-foreground text-sm max-w-md text-center">
          {(statsErrorObj as Error)?.message ?? "An unexpected error occurred while fetching telemetry data."}
        </p>
        <Button variant="outline" onClick={() => refetchStats()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-muted-foreground mt-1">
          Overview of your OTA infrastructure and recent activity.
        </p>
      </div>

      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Devices"
          value={overview?.totalDevices ?? 0}
          icon={<Monitor className="h-5 w-5" />}
          description="Registered devices across all groups"
        />
        <StatCard
          title="Active Deployments"
          value={overview?.activeDeployments ?? 0}
          icon={<Rocket className="h-5 w-5" />}
          description="Deployments currently in progress"
        />
        <StatCard
          title="Pending Updates"
          value={overview?.pendingUpdates ?? 0}
          icon={<RefreshCw className="h-5 w-5" />}
          description="Devices awaiting an update"
        />
        <StatCard
          title="Failed Devices"
          value={overview?.failedDevices ?? 0}
          icon={<AlertTriangle className="h-5 w-5" />}
          description="Devices with recent update failures"
          variant={overview && overview.failedDevices > 0 ? "destructive" : "default"}
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Activity feed */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle className="text-lg">Recent Activity</CardTitle>
          </CardHeader>
          <CardContent>
            <ActivityFeed events={activityEvents} loading={activityLoading} />
          </CardContent>
        </Card>

        {/* Quick actions */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Quick Actions</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Button className="w-full justify-start" variant="default" onClick={() => navigate({ to: "/releases" })}>
              <PackageOpen className="h-4 w-4 mr-2" />
              Create Release
            </Button>
            <Button className="w-full justify-start" variant="default" onClick={() => navigate({ to: "/deployments" })}>
              <Rocket className="h-4 w-4 mr-2" />
              Create Deployment
            </Button>
            <Button className="w-full justify-start" variant="default" onClick={() => navigate({ to: "/devices" })}>
              <Monitor className="h-4 w-4 mr-2" />
              Register Device
            </Button>
            <Button className="w-full justify-start" variant="outline" onClick={() => navigate({ to: "/groups" })}>
              <Users className="h-4 w-4 mr-2" />
              Manage Groups
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
