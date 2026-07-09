import { useState, useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { DataTable } from "@/components/data-table/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";
import { useDeployments, type Deployment } from "@/hooks/useDeployments";
import { useCreateDeployment } from "@/hooks/useCreateDeployment";

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

type DeploymentStrategy = "rolling" | "blue-green" | "canary" | "full";

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
/*  Create deployment schema                                           */
/* ------------------------------------------------------------------ */

const createDeploymentSchema = z.object({
  targetGroupId: z.string().min(1, "Target group is required"),
  releaseVersion: z.string().min(1, "Release version is required"),
  strategy: z.enum(["rolling", "blue-green", "canary", "full"], {
    required_error: "Strategy is required",
  }),
  percentage: z.coerce.number().min(1).max(100).optional(),
});

type CreateDeploymentValues = z.infer<typeof createDeploymentSchema>;

/* ------------------------------------------------------------------ */
/*  Status badge helper                                                */
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

/* ------------------------------------------------------------------ */
/*  Strategy badge helper                                              */
/* ------------------------------------------------------------------ */

const STRATEGY_LABEL: Record<DeploymentStrategy, string> = {
  rolling: "Rolling",
  "blue-green": "Blue-Green",
  canary: "Canary",
  full: "Full",
};

function StrategyBadge({ strategy }: { strategy: string }) {
  const key = strategy as DeploymentStrategy;
  return <Badge variant="secondary">{STRATEGY_LABEL[key] ?? strategy}</Badge>;
}

/* ------------------------------------------------------------------ */
/*  Columns                                                            */
/* ------------------------------------------------------------------ */

const columns: ColumnDef<Deployment>[] = [
  {
    accessorKey: "targetGroupName",
    header: "Target",
    cell: ({ row }) => (
      <span className="font-medium">{row.getValue("targetGroupName")}</span>
    ),
  },
  {
    accessorKey: "releaseVersion",
    header: "Version",
    cell: ({ row }) => (
      <span className="font-mono text-sm">{row.getValue("releaseVersion")}</span>
    ),
  },
  {
    accessorKey: "strategy",
    header: "Strategy",
    cell: ({ row }) => <StrategyBadge strategy={row.getValue("strategy")} />,
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge status={row.getValue("status")} />,
  },
  {
    accessorKey: "deviceStats",
    header: "Devices",
    cell: ({ row }) => {
      const stats = row.getValue<{ succeeded: number; failed: number; pending: number } | undefined>(
        "deviceStats",
      );
      if (!stats) return <span className="text-muted-foreground text-sm">—</span>;
      const total = stats.succeeded + stats.failed + stats.pending;
      return <span className="text-sm">{total} devices</span>;
    },
  },
  {
    accessorKey: "createdAt",
    header: "Created",
    cell: ({ row }) => {
      const val = row.getValue("createdAt");
      if (!val) return <span className="text-muted-foreground text-sm">—</span>;
      return (
        <span className="text-muted-foreground text-sm">
          {new Date(val as string).toLocaleDateString()}
        </span>
      );
    },
  },
];

/* ------------------------------------------------------------------ */
/*  Loading skeleton                                                   */
/* ------------------------------------------------------------------ */

function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <Skeleton className="h-10 w-48" />
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
      <div className="mb-4 text-5xl text-destructive">!</div>
      <h3 className="mb-2 text-lg font-semibold">Failed to Load Deployments</h3>
      <p className="mb-6 max-w-md text-sm text-muted-foreground">{message}</p>
      <Button onClick={onRetry}>Retry</Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Empty state                                                        */
/* ------------------------------------------------------------------ */

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <h3 className="mb-2 text-lg font-semibold">No Deployments Yet</h3>
      <p className="mb-6 max-w-sm text-sm text-muted-foreground">
        Create a deployment to roll out a release to your device groups.
      </p>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Create Deployment Dialog                                           */
/* ------------------------------------------------------------------ */

interface CreateDeploymentDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function CreateDeploymentDialog({ open, onOpenChange }: CreateDeploymentDialogProps) {
  const { toast } = useToast();
  const createDeploymentMutation = useCreateDeployment();

  const form = useForm<CreateDeploymentValues>({
    resolver: zodResolver(createDeploymentSchema),
    defaultValues: {
      targetGroupId: "",
      releaseVersion: "",
      strategy: "rolling",
      percentage: 100,
    },
  });

  const selectedStrategy = form.watch("strategy");

  const handleSubmit = async (values: CreateDeploymentValues) => {
    try {
      await createDeploymentMutation.mutateAsync(values);
      toast({ title: "Deployment created", description: "The deployment has been created and is being processed." });
      onOpenChange(false);
      form.reset();
    } catch {
      toast({ title: "Failed to create deployment", variant: "destructive" });
    }
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) form.reset();
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>Create Deployment</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Deployment</DialogTitle>
          <DialogDescription>
            Configure a new OTA update deployment for a target group.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="targetGroupId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Target Group</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select a device group…" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="group-1">Production Devices</SelectItem>
                      <SelectItem value="group-2">Staging Devices</SelectItem>
                      <SelectItem value="group-3">Canary Group</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="releaseVersion"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Release Version</FormLabel>
                  <FormControl>
                    <Input placeholder="1.0.0" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="strategy"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Strategy</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="rolling">Rolling</SelectItem>
                      <SelectItem value="blue-green">Blue-Green</SelectItem>
                      <SelectItem value="canary">Canary</SelectItem>
                      <SelectItem value="full">Full (All at once)</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {selectedStrategy === "rolling" && "Updates roll out gradually across devices over time."}
                    {selectedStrategy === "blue-green" && "Switches between two environments for zero-downtime updates."}
                    {selectedStrategy === "canary" && "Rolls out to a small subset first, then expands on success."}
                    {selectedStrategy === "full" && "All devices receive the update simultaneously."}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {selectedStrategy === "rolling" && (
              <FormField
                control={form.control}
                name="percentage"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Initial Percentage</FormLabel>
                    <FormControl>
                      <Input type="number" min={1} max={100} {...field} />
                    </FormControl>
                    <FormDescription>Percentage of devices to target in the initial rollout wave.</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <DialogFooter>
              <Button type="submit" disabled={createDeploymentMutation.isPending}>
                {createDeploymentMutation.isPending ? "Creating…" : "Create Deployment"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}

/* ------------------------------------------------------------------ */
/*  Page component                                                     */
/* ------------------------------------------------------------------ */

export default function DeploymentsPage() {
  const navigate = useNavigate();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);

  const { data: deployments, isLoading, isError, error, refetch } = useDeployments();

  const deploymentList = useMemo<Deployment[]>(() => deployments ?? [], [deployments]);

  const handleRowClick = (deployment: Deployment) => {
    navigate({ to: "/deployments/$deploymentId", params: { deploymentId: deployment.id } });
  };

  /* ---- states ---- */
  if (isLoading) {
    return (
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Deployments</h1>
        </div>
        <LoadingSkeleton />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Deployments</h1>
        </div>
        <ErrorState
          message={(error as Error)?.message ?? "An unexpected error occurred."}
          onRetry={() => refetch()}
        />
      </div>
    );
  }

  const isEmpty = !deploymentList || deploymentList.length === 0;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Deployments</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Manage rollout of OTA updates to device groups.
          </p>
        </div>
        <CreateDeploymentDialog open={createDialogOpen} onOpenChange={setCreateDialogOpen} />
      </div>

      {isEmpty ? (
        <EmptyState />
      ) : (
        <DataTable columns={columns} data={deploymentList} onRowClick={handleRowClick} />
      )}
    </div>
  );
}
