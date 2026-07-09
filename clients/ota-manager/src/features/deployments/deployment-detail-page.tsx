import { useState, useMemo } from "react";
import { useParams, useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { useToast } from "@/hooks/use-toast";
import { useDeployment, type Deployment } from "@/hooks/useDeployment";
import { useEvaluateRollout } from "@/hooks/useEvaluateRollout";
import { useRecall } from "@/hooks/useRecall";

/* ------------------------------------------------------------------ */
/*  Types & constants                                                  */
/* ------------------------------------------------------------------ */

type DeployStatus = "active" | "paused" | "completed" | "failed" | "recalled";

const STATUS_CONFIG: Record<
  DeployStatus,
  { label: string; variant: "default" | "secondary" | "outline" | "destructive"; dot: string }
> = {
  active: { label: "Active", variant: "default", dot: "bg-green-500" },
  paused: { label: "Paused", variant: "secondary", dot: "bg-yellow-500" },
  completed: { label: "Completed", variant: "outline", dot: "bg-blue-500" },
  failed: { label: "Failed", variant: "destructive", dot: "bg-red-500" },
  recalled: { label: "Recalled", variant: "secondary", dot: "bg-gray-400" },
};

/* ------------------------------------------------------------------ */
/*  Rollback history types                                             */
/* ------------------------------------------------------------------ */

interface RollbackEvent {
  id: string;
  timestamp: string;
  fromVersion: string;
  toVersion: string;
  reason: string;
  triggeredBy: string;
}

/* ------------------------------------------------------------------ */
/*  Rollout percentage form schema                                     */
/* ------------------------------------------------------------------ */

const rolloutSchema = z.object({
  percentage: z.coerce.number().min(0, "Minimum is 0%").max(100, "Maximum is 100%"),
});

type RolloutValues = z.infer<typeof rolloutSchema>;

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function StatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase() as DeployStatus;
  const config = STATUS_CONFIG[key];
  if (!config) return <Badge>{status}</Badge>;
  return (
    <Badge variant={config.variant} className="flex items-center gap-1.5">
      <span className={`inline-block h-2 w-2 rounded-full ${config.dot}`} />
      {config.label}
    </Badge>
  );
}

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
/*  Loading skeleton                                                   */
/* ------------------------------------------------------------------ */

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-64" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-32 w-full" />
        ))}
      </div>
      <Skeleton className="h-48 w-full" />
      <Skeleton className="h-32 w-full" />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Error state                                                        */
/* ------------------------------------------------------------------ */

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="mb-4 text-5xl text-destructive">!</div>
      <h3 className="mb-2 text-lg font-semibold">Failed to Load Deployment</h3>
      <p className="mb-6 max-w-md text-sm text-muted-foreground">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Deployment Info Card                                               */
/* ------------------------------------------------------------------ */

interface DeploymentInfoCardProps {
  deployment: Deployment;
}

function DeploymentInfoCard({ deployment }: DeploymentInfoCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Deployment Info</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div>
          <span className="text-muted-foreground">Target Group</span>
          <p className="font-medium">{(deployment as { targetGroupName?: string }).targetGroupName ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Release Version</span>
          <p className="font-mono font-medium">{(deployment as { releaseVersion?: string }).releaseVersion ?? "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Strategy</span>
          <p className="font-medium capitalize">{deployment.strategy}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Status</span>
          <div className="mt-0.5">
            <StatusBadge status={deployment.status} />
          </div>
        </div>
        <div>
          <span className="text-muted-foreground">Created</span>
          <p>{deployment.createdAt ? formatDateTime(deployment.createdAt) : "—"}</p>
        </div>
        <div>
          <span className="text-muted-foreground">Created By</span>
          <p>{(deployment as { createdBy?: string }).createdBy ?? "—"}</p>
        </div>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------------ */
/*  Device Status Breakdown                                            */
/* ------------------------------------------------------------------ */

interface DeviceStatusBreakdownProps {
  stats: { succeeded: number; failed: number; pending: number; total: number };
}

function DeviceStatusBreakdown({ stats }: DeviceStatusBreakdownProps) {
  const succeededPct = stats.total > 0 ? Math.round((stats.succeeded / stats.total) * 100) : 0;
  const failedPct = stats.total > 0 ? Math.round((stats.failed / stats.total) * 100) : 0;
  const pendingPct = stats.total > 0 ? Math.round((stats.pending / stats.total) * 100) : 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Device Status</CardTitle>
        <CardDescription>
          {stats.total} device{stats.total !== 1 ? "s" : ""} in this deployment target
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex h-3 overflow-hidden rounded-full bg-muted">
          {stats.succeeded > 0 && (
            <div
              className="bg-green-500 transition-all"
              style={{ width: `${succeededPct}%` }}
              title={`${stats.succeeded} succeeded`}
            />
          )}
          {stats.failed > 0 && (
            <div
              className="bg-red-500 transition-all"
              style={{ width: `${failedPct}%` }}
              title={`${stats.failed} failed`}
            />
          )}
          {stats.pending > 0 && (
            <div
              className="bg-yellow-500 transition-all"
              style={{ width: `${pendingPct}%` }}
              title={`${stats.pending} pending`}
            />
          )}
        </div>

        <div className="grid grid-cols-3 gap-4 text-center text-sm">
          <div className="space-y-1">
            <p className="text-2xl font-bold text-green-600">{stats.succeeded}</p>
            <p className="text-muted-foreground text-xs">Succeeded</p>
          </div>
          <div className="space-y-1">
            <p className="text-2xl font-bold text-red-600">{stats.failed}</p>
            <p className="text-muted-foreground text-xs">Failed</p>
          </div>
          <div className="space-y-1">
            <p className="text-2xl font-bold text-yellow-600">{stats.pending}</p>
            <p className="text-muted-foreground text-xs">Pending</p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------------ */
/*  Rollout Control Panel                                              */
/* ------------------------------------------------------------------ */

interface RolloutControlPanelProps {
  deploymentId: string;
  currentPercentage: number;
  currentStatus: string;
}

function RolloutControlPanel({ deploymentId, currentPercentage, currentStatus }: RolloutControlPanelProps) {
  const { toast } = useToast();
  const evaluateRolloutMutation = useEvaluateRollout();

  const form = useForm<RolloutValues>({
    resolver: zodResolver(rolloutSchema),
    defaultValues: { percentage: currentPercentage },
  });

  const watchedPercentage = form.watch("percentage");

  const isTerminal = currentStatus === "completed" || currentStatus === "failed" || currentStatus === "recalled";

  const handleEvaluate = async (values: RolloutValues) => {
    try {
      await evaluateRolloutMutation.mutateAsync({
        deploymentId,
        percentage: values.percentage,
      });
      toast({
        title: "Rollout evaluated",
        description: `Rollout percentage updated to ${values.percentage}%.`,
      });
    } catch {
      toast({ title: "Failed to evaluate rollout", variant: "destructive" });
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Rollout Control</CardTitle>
        <CardDescription>
          Adjust the rollout percentage and evaluate progress.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Current rollout status */}
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Current rollout status</span>
          <StatusBadge status={currentStatus} />
        </div>

        {/* Percentage slider + input */}
        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleEvaluate)} className="space-y-4">
            <FormField
              control={form.control}
              name="percentage"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Rollout Percentage</FormLabel>
                  <div className="flex items-center gap-4">
                    <div className="flex-1 space-y-2">
                      <Progress value={watchedPercentage} className="h-3" />
                      <div className="flex justify-between text-xs text-muted-foreground">
                        <span>0%</span>
                        <span>{watchedPercentage ?? 0}%</span>
                        <span>100%</span>
                      </div>
                    </div>
                    <FormControl>
                      <Input
                        type="number"
                        min={0}
                        max={100}
                        className="w-20"
                        {...field}
                      />
                    </FormControl>
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <Button
              type="submit"
              disabled={evaluateRolloutMutation.isPending || isTerminal}
              className="w-full"
            >
              {evaluateRolloutMutation.isPending ? "Evaluating…" : "Evaluate Rollout"}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------------ */
/*  Recall Button                                                      */
/* ------------------------------------------------------------------ */

interface RecallSectionProps {
  deploymentId: string;
  currentStatus: string;
  recallReason: string;
  onRecallReasonChange: (reason: string) => void;
}

function RecallSection({ deploymentId, currentStatus, recallReason, onRecallReasonChange }: RecallSectionProps) {
  const { toast } = useToast();
  const recallMutation = useRecall();
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);

  const isTerminal = currentStatus === "completed" || currentStatus === "failed" || currentStatus === "recalled";

  const handleRecall = async () => {
    try {
      await recallMutation.mutateAsync({ deploymentId, reason: recallReason });
      toast({
        title: "Deployment recalled",
        description: "The deployment has been recalled successfully.",
      });
      setConfirmDialogOpen(false);
    } catch {
      toast({ title: "Failed to recall deployment", variant: "destructive" });
    }
  };

  if (currentStatus === "recalled") {
    return null;
  }

  return (
    <>
      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-lg text-destructive">Recall Deployment</CardTitle>
          <CardDescription>
            Immediately halt this deployment and roll back affected devices.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="recall-reason" className="text-sm font-medium">
              Reason for recall
            </label>
            <Input
              id="recall-reason"
              placeholder="e.g. Critical bug discovered in release"
              value={recallReason}
              onChange={(e) => onRecallReasonChange(e.target.value)}
              disabled={isTerminal}
            />
          </div>
          <Button
            variant="destructive"
            onClick={() => setConfirmDialogOpen(true)}
            disabled={isTerminal || recallMutation.isPending || !recallReason.trim()}
            className="w-full"
          >
            {recallMutation.isPending ? "Recalling…" : "Recall Deployment"}
          </Button>
        </CardContent>
      </Card>

      <AlertDialog open={confirmDialogOpen} onOpenChange={setConfirmDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Recall Deployment?</AlertDialogTitle>
            <AlertDialogDescription>
              This will immediately halt the deployment and roll back affected devices.
              This action cannot be undone.
              {recallReason && (
                <>
                  <br />
                  <br />
                  <strong>Reason:</strong> {recallReason}
                </>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleRecall} className="bg-destructive text-destructive-foreground">
              Confirm Recall
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

/* ------------------------------------------------------------------ */
/*  Rollback History List                                              */
/* ------------------------------------------------------------------ */

interface RollbackHistoryProps {
  events: RollbackEvent[];
}

function RollbackHistory({ events }: RollbackHistoryProps) {
  if (!events || events.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Rollback History</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">No rollbacks have occurred for this deployment.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Rollback History</CardTitle>
        <CardDescription>
          {events.length} rollback{events.length !== 1 ? "s" : ""} recorded
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {events.map((event) => (
          <div
            key={event.id}
            className="flex items-start justify-between rounded-md border p-3 text-sm"
          >
            <div className="space-y-1">
              <p className="font-medium">
                {event.fromVersion} → {event.toVersion}
              </p>
              <p className="text-muted-foreground text-xs">{event.reason}</p>
              <p className="text-muted-foreground text-xs">
                Triggered by: {event.triggeredBy}
              </p>
            </div>
            <span className="text-muted-foreground text-xs whitespace-nowrap ml-4">
              {formatDateTime(event.timestamp)}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

/* ------------------------------------------------------------------ */
/*  Page component                                                     */
/* ------------------------------------------------------------------ */

export default function DeploymentDetailPage() {
  const { deploymentId } = useParams({ from: "/layout/deployments/$deploymentId" });
  const navigate = useNavigate();

  const [recallReason, setRecallReason] = useState("");

  const {
    data: deployment,
    isLoading,
    isError,
    error,
    refetch,
  } = useDeployment(
    { deploymentId: deploymentId ?? "" },
    { refetchInterval: 15_000 },
  );

  /* ---- derived data ---- */
  const deviceStats = useMemo(() => {
    if (!deployment) return { succeeded: 0, failed: 0, pending: 0, total: 0 };
    const stats = (deployment as { deviceStats?: { succeeded: number; failed: number; pending: number } }).deviceStats;
    if (!stats) return { succeeded: 0, failed: 0, pending: 0, total: 0 };
    return {
      ...stats,
      total: stats.succeeded + stats.failed + stats.pending,
    };
  }, [deployment]);

  const rollbackEvents = useMemo<RollbackEvent[]>(() => {
    if (!deployment) return [];
    return (deployment as { rollbackHistory?: RollbackEvent[] }).rollbackHistory ?? [];
  }, [deployment]);

  const currentPercentage = useMemo(() => {
    if (!deployment) return 0;
    return (deployment as { rolloutPercentage?: number }).rolloutPercentage ?? 0;
  }, [deployment]);

  /* ---- states ---- */
  if (isLoading) {
    return (
      <div className="p-6">
        <LoadingSkeleton />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-6">
        <ErrorState
          message={(error as Error)?.message ?? "An unexpected error occurred."}
          onRetry={() => refetch()}
        />
      </div>
    );
  }

  if (!deployment) {
    return (
      <div className="p-6">
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <h3 className="mb-2 text-lg font-semibold">Deployment Not Found</h3>
          <p className="mb-6 max-w-sm text-sm text-muted-foreground">
            The deployment you are looking for does not exist or has been removed.
          </p>
          <Button variant="outline" onClick={() => navigate({ to: "/deployments" })}>
            Back to Deployments
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" onClick={() => navigate({ to: "/deployments" })}>
            &larr; Back
          </Button>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              Deployment {(deployment as { releaseVersion?: string }).releaseVersion ?? ""}
            </h1>
            <p className="text-muted-foreground text-sm">
              Target: {(deployment as { targetGroupName?: string }).targetGroupName ?? "—"}
            </p>
          </div>
        </div>
        <StatusBadge status={deployment.status} />
      </div>

      {/* Info cards grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          <DeploymentInfoCard deployment={deployment} />
          <DeviceStatusBreakdown stats={deviceStats} />
          <RollbackHistory events={rollbackEvents} />
        </div>

        <div className="space-y-6">
          <RolloutControlPanel
            deploymentId={deployment.id}
            currentPercentage={currentPercentage}
            currentStatus={deployment.status}
          />
          <RecallSection
            deploymentId={deployment.id}
            currentStatus={deployment.status}
            recallReason={recallReason}
            onRecallReasonChange={setRecallReason}
          />
        </div>
      </div>
    </div>
  );
}
