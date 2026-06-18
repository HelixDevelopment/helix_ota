import { useState, useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { DataTable } from "@/components/data-table/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import { useReleases, type Release } from "@/hooks/useReleases";
import { useCreateRelease } from "@/hooks/useCreateRelease";
import { useUploadArtifact } from "@/hooks/useUploadArtifact";
import { useArtifact } from "@/hooks/useArtifact";

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

type ReleaseStatus = "draft" | "active" | "superseded" | "rolled-back" | "failed";

const STATUS_BADGE_VARIANT: Record<
  ReleaseStatus,
  "default" | "secondary" | "outline" | "destructive"
> = {
  draft: "outline",
  active: "default",
  superseded: "secondary",
  "rolled-back": "destructive",
  failed: "destructive",
};

const STATUS_LABEL: Record<ReleaseStatus, string> = {
  draft: "Draft",
  active: "Active",
  superseded: "Superseded",
  "rolled-back": "Rolled Back",
  failed: "Failed",
};

/* ------------------------------------------------------------------ */
/*  Create release step schemas                                        */
/* ------------------------------------------------------------------ */

const step1Schema = z.object({
  artifactId: z.string().min(1, "Please select an artifact"),
});

const step2Schema = z.object({
  version: z
    .string()
    .min(1, "Version is required")
    .regex(/^\d+\.\d+\.\d+(-[\w.]+)?$/, "Must be a valid semver (e.g. 1.0.0)"),
  os: z.string().min(1, "OS is required"),
  targetModel: z.string().min(1, "Target model is required"),
});

const step3Schema = z.object({
  confirm: z.literal(true, {
    errorMap: () => ({ message: "Please confirm before creating" }),
  }),
});

type Step1Values = z.infer<typeof step1Schema>;
type Step2Values = z.infer<typeof step2Schema>;
type Step3Values = z.infer<typeof step3Schema>;

/* ------------------------------------------------------------------ */
/*  Status badge helper                                                */
/* ------------------------------------------------------------------ */

function StatusBadge({ status }: { status: string }) {
  const key = status.toLowerCase().replace(/\s+/g, "-") as ReleaseStatus;
  const variant = STATUS_BADGE_VARIANT[key] ?? "secondary";
  const label = STATUS_LABEL[key] ?? status;
  return <Badge variant={variant}>{label}</Badge>;
}

/* ------------------------------------------------------------------ */
/*  Columns                                                            */
/* ------------------------------------------------------------------ */

const columns: ColumnDef<Release>[] = [
  {
    accessorKey: "version",
    header: "Version",
    cell: ({ row }) => (
      <span className="font-mono text-sm font-medium">{row.getValue("version")}</span>
    ),
  },
  {
    accessorKey: "os",
    header: "OS",
  },
  {
    accessorKey: "targetModel",
    header: "Target Model",
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge status={row.getValue("status")} />,
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
  {
    id: "actions",
    header: "Actions",
    cell: () => (
      <Button size="sm" variant="outline">
        View
      </Button>
    ),
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
        <Skeleton className="h-10 w-32" />
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
      <h3 className="mb-2 text-lg font-semibold">Failed to Load Releases</h3>
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
      <h3 className="mb-2 text-lg font-semibold">No Releases Yet</h3>
      <p className="mb-6 max-w-sm text-sm text-muted-foreground">
        Create your first release to make a new OTA update available for deployment.
      </p>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Filter bar                                                         */
/* ------------------------------------------------------------------ */

interface FilterBarProps {
  osFilter: string;
  onOsFilterChange: (value: string) => void;
  modelFilter: string;
  onModelFilterChange: (value: string) => void;
  statusFilter: string;
  onStatusFilterChange: (value: string) => void;
}

function FilterBar({
  osFilter,
  onOsFilterChange,
  modelFilter,
  onModelFilterChange,
  statusFilter,
  onStatusFilterChange,
}: FilterBarProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input
        placeholder="OS"
        value={osFilter}
        onChange={(e) => onOsFilterChange(e.target.value)}
        className="w-40"
      />
      <Input
        placeholder="Target Model"
        value={modelFilter}
        onChange={(e) => onModelFilterChange(e.target.value)}
        className="w-40"
      />
      <Input
        placeholder="Status"
        value={statusFilter}
        onChange={(e) => onStatusFilterChange(e.target.value)}
        className="w-40"
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Create Release Dialog (multi-step form)                            */
/* ------------------------------------------------------------------ */

interface CreateReleaseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function CreateReleaseDialog({ open, onOpenChange }: CreateReleaseDialogProps) {
  const { toast } = useToast();
  const createReleaseMutation = useCreateRelease();
  const { data: artifacts } = useUploadArtifact();
  const { data: artifactDetail } = useArtifact();

  const [step, setStep] = useState(0);

  const step1Form = useForm<Step1Values>({
    resolver: zodResolver(step1Schema),
    defaultValues: { artifactId: "" },
  });

  const step2Form = useForm<Step2Values>({
    resolver: zodResolver(step2Schema),
    defaultValues: { version: "", os: "", targetModel: "" },
  });

  const step3Form = useForm<Step3Values>({
    resolver: zodResolver(step3Schema),
    defaultValues: { confirm: false as unknown as true },
  });

  const step1Values = step1Form.watch();
  const step2Values = step2Form.watch();

  const handleNext = async () => {
    if (step === 0) {
      const valid = await step1Form.trigger();
      if (!valid) return;
      setStep(1);
    } else if (step === 1) {
      const valid = await step2Form.trigger();
      if (!valid) return;
      setStep(2);
    }
  };

  const handleBack = () => {
    setStep((s) => Math.max(0, s - 1));
  };

  const handleSubmit = async () => {
    const valid = await step3Form.trigger();
    if (!valid) return;

    try {
      await createReleaseMutation.mutateAsync({
        artifactId: step1Values.artifactId,
        version: step2Values.version,
        os: step2Values.os,
        targetModel: step2Values.targetModel,
      });
      toast({ title: "Release created", description: `Release ${step2Values.version} has been created.` });
      onOpenChange(false);
      setStep(0);
      step1Form.reset();
      step2Form.reset();
      step3Form.reset();
    } catch {
      toast({ title: "Failed to create release", variant: "destructive" });
    }
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setStep(0);
      step1Form.reset();
      step2Form.reset();
      step3Form.reset();
    }
    onOpenChange(nextOpen);
  };

  const selectedArtifactName = artifacts?.find((a: { id: string }) => a.id === step1Values.artifactId)?.name;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>Create Release</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Release</DialogTitle>
          <DialogDescription>
            Step {step + 1} of 3
          </DialogDescription>
        </DialogHeader>

        {/* Step indicators */}
        <div className="flex items-center gap-2 mb-4">
          {["Select Artifact", "Set Details", "Review & Submit"].map((label, i) => (
            <div key={label} className="flex items-center gap-2 flex-1">
              <div
                className={`flex items-center justify-center w-7 h-7 rounded-full text-xs font-medium ${
                  i === step
                    ? "bg-primary text-primary-foreground"
                    : i < step
                      ? "bg-primary/20 text-primary"
                      : "bg-muted text-muted-foreground"
                }`}
              >
                {i < step ? "✓" : i + 1}
              </div>
              <span
                className={`text-xs hidden sm:inline ${
                  i === step ? "font-medium text-foreground" : "text-muted-foreground"
                }`}
              >
                {label}
              </span>
              {i < 2 && <div className="h-px flex-1 bg-border" />}
            </div>
          ))}
        </div>

        {/* Step 1: Select artifact */}
        {step === 0 && (
          <Form {...step1Form}>
            <form className="space-y-4">
              <FormField
                control={step1Form.control}
                name="artifactId"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Artifact</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="Search and select an artifact…" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {artifacts && artifacts.length > 0 ? (
                          artifacts.map((artifact: { id: string; name: string }) => (
                            <SelectItem key={artifact.id} value={artifact.id}>
                              {artifact.name}
                            </SelectItem>
                          ))
                        ) : (
                          <SelectItem value="__placeholder__" disabled>
                            No artifacts available
                          </SelectItem>
                        )}
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      Choose the build artifact this release will serve to devices.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedArtifactName && artifactDetail && (
                <Card className="bg-muted/30">
                  <CardHeader className="py-3">
                    <CardTitle className="text-sm">Artifact Details</CardTitle>
                  </CardHeader>
                  <CardContent className="text-sm text-muted-foreground">
                    <p>Size: {(artifactDetail as { size?: number })?.size ?? "—"} bytes</p>
                    <p>Checksum: {(artifactDetail as { checksum?: string })?.checksum ?? "—"}</p>
                  </CardContent>
                </Card>
              )}
            </form>
          </Form>
        )}

        {/* Step 2: Set version, OS, target model */}
        {step === 1 && (
          <Form {...step2Form}>
            <form className="space-y-4">
              <FormField
                control={step2Form.control}
                name="version"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Version</FormLabel>
                    <FormControl>
                      <Input placeholder="1.0.0" {...field} />
                    </FormControl>
                    <FormDescription>Semantic version for this release.</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={step2Form.control}
                name="os"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>OS</FormLabel>
                    <FormControl>
                      <Input placeholder="Android 14" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={step2Form.control}
                name="targetModel"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Target Model</FormLabel>
                    <FormControl>
                      <Input placeholder="Orange Pi 5 Max" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </form>
          </Form>
        )}

        {/* Step 3: Review + submit */}
        {step === 2 && (
          <Form {...step3Form}>
            <form className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Release Summary</CardTitle>
                </CardHeader>
                <CardContent className="grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <span className="text-muted-foreground">Artifact</span>
                    <p className="font-medium">{selectedArtifactName ?? step1Values.artifactId}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Version</span>
                    <p className="font-medium font-mono">{step2Values.version}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">OS</span>
                    <p className="font-medium">{step2Values.os}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Target Model</span>
                    <p className="font-medium">{step2Values.targetModel}</p>
                  </div>
                </CardContent>
              </Card>

              <FormField
                control={step3Form.control}
                name="confirm"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start gap-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <input
                        type="checkbox"
                        checked={field.value as unknown as boolean}
                        onChange={field.onChange}
                        className="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary"
                      />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>I confirm the details above are correct</FormLabel>
                      <FormDescription>
                        Once created, this release will be available for deployment to matching devices.
                      </FormDescription>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </form>
          </Form>
        )}

        <DialogFooter className="gap-2">
          {step > 0 && (
            <Button variant="outline" onClick={handleBack} disabled={createReleaseMutation.isPending}>
              Back
            </Button>
          )}
          {step < 2 ? (
            <Button onClick={handleNext}>Next</Button>
          ) : (
            <Button onClick={handleSubmit} disabled={createReleaseMutation.isPending}>
              {createReleaseMutation.isPending ? "Creating…" : "Create Release"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/* ------------------------------------------------------------------ */
/*  Page component                                                     */
/* ------------------------------------------------------------------ */

export default function ReleasesPage() {
  const [osFilter, setOsFilter] = useState("");
  const [modelFilter, setModelFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [createDialogOpen, setCreateDialogOpen] = useState(false);

  const { data: releases, isLoading, isError, error, refetch } = useReleases({
    os: osFilter,
    targetModel: modelFilter,
    status: statusFilter,
  });

  const filteredReleases = useMemo(() => {
    if (!releases) return [];
    return releases as Release[];
  }, [releases]);

  /* ---- states ---- */
  if (isLoading) {
    return (
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Releases</h1>
        </div>
        <LoadingSkeleton />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold tracking-tight">Releases</h1>
        </div>
        <ErrorState
          message={(error as Error)?.message ?? "An unexpected error occurred."}
          onRetry={() => refetch()}
        />
      </div>
    );
  }

  const isEmpty = !filteredReleases || filteredReleases.length === 0;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Releases</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Manage OTA update releases and their target configurations.
          </p>
        </div>
        <CreateReleaseDialog open={createDialogOpen} onOpenChange={setCreateDialogOpen} />
      </div>

      {isEmpty ? (
        <EmptyState />
      ) : (
        <>
          <div className="mb-4">
            <FilterBar
              osFilter={osFilter}
              onOsFilterChange={setOsFilter}
              modelFilter={modelFilter}
              onModelFilterChange={setModelFilter}
              statusFilter={statusFilter}
              onStatusFilterChange={setStatusFilter}
            />
          </div>

          <DataTable columns={columns} data={filteredReleases} />
        </>
      )}
    </div>
  );
}
