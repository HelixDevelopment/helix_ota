import { useState, useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";

import { DataTable } from "@/components/data-table/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Input } from "@/components/ui/input";
import { useAuditLog } from "@/hooks/useAuditLog";

interface AuditEntry {
  id: string;
  timestamp: string;
  action: string;
  actor: string;
  target: string;
  detail: string;
}

const ACTION_BADGE_VARIANTS: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  create: "default",
  update: "secondary",
  delete: "destructive",
  deploy: "outline",
  rollback: "destructive",
};

function getActionVariant(action: string) {
  const lower = action.toLowerCase();
  for (const [key, variant] of Object.entries(ACTION_BADGE_VARIANTS)) {
    if (lower.includes(key)) return variant;
  }
  return "secondary";
}

export default function AuditPage() {
  const { data: entries, isLoading, isError, error, refetch } = useAuditLog();

  const [actionFilter, setActionFilter] = useState("");
  const [actorFilter, setActorFilter] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const filteredEntries = useMemo(() => {
    if (!entries) return [];

    return entries.filter((entry: AuditEntry) => {
      if (actionFilter && !entry.action.toLowerCase().includes(actionFilter.toLowerCase())) {
        return false;
      }
      if (actorFilter && !entry.actor.toLowerCase().includes(actorFilter.toLowerCase())) {
        return false;
      }
      if (dateFrom && new Date(entry.timestamp) < new Date(dateFrom)) {
        return false;
      }
      if (dateTo && new Date(entry.timestamp) > new Date(dateTo)) {
        return false;
      }
      return true;
    });
  }, [entries, actionFilter, actorFilter, dateFrom, dateTo]);

  const columns = useMemo<ColumnDef<AuditEntry>[]>(
    () => [
      {
        accessorKey: "timestamp",
        header: "Timestamp",
        cell: ({ row }) => (
          <span className="text-sm font-mono">
            {new Date(row.original.timestamp).toLocaleString()}
          </span>
        ),
      },
      {
        accessorKey: "action",
        header: "Action",
        cell: ({ row }) => (
          <Badge variant={getActionVariant(row.original.action)}>
            {row.original.action}
          </Badge>
        ),
      },
      {
        accessorKey: "actor",
        header: "Actor",
        cell: ({ row }) => (
          <span className="text-sm">{row.original.actor}</span>
        ),
      },
      {
        accessorKey: "target",
        header: "Target",
        cell: ({ row }) => (
          <span className="text-sm font-mono text-muted-foreground">
            {row.original.target}
          </span>
        ),
      },
      {
        accessorKey: "detail",
        header: "Detail",
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground max-w-xs truncate block" title={row.original.detail}>
            {row.original.detail}
          </span>
        ),
      },
    ],
    [],
  );

  // Loading skeleton state
  if (isLoading) {
    return (
      <div className="space-y-4 p-6">
        <div className="flex items-center justify-between">
          <Skeleton className="h-8 w-48" />
        </div>
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      </div>
    );
  }

  // Error state with retry button
  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 p-6">
        <p className="text-destructive text-lg font-semibold">Failed to load audit log</p>
        <p className="text-muted-foreground text-sm">{(error as Error)?.message ?? "An unexpected error occurred."}</p>
        <Button variant="outline" onClick={() => refetch()}>
          Retry
        </Button>
      </div>
    );
  }

  const isEmpty = !filteredEntries || filteredEntries.length === 0;

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Audit Log</h1>
        <p className="text-muted-foreground mt-1">Review all system actions, changes, and deployments.</p>
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex flex-col gap-1">
          <label htmlFor="action-filter" className="text-xs font-medium text-muted-foreground">
            Action
          </label>
          <Input
            id="action-filter"
            placeholder="Filter by action…"
            value={actionFilter}
            onChange={(e) => setActionFilter(e.target.value)}
            className="w-40"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="actor-filter" className="text-xs font-medium text-muted-foreground">
            Actor
          </label>
          <Input
            id="actor-filter"
            placeholder="Filter by actor…"
            value={actorFilter}
            onChange={(e) => setActorFilter(e.target.value)}
            className="w-40"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="date-from" className="text-xs font-medium text-muted-foreground">
            From
          </label>
          <Input
            id="date-from"
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className="w-40"
          />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="date-to" className="text-xs font-medium text-muted-foreground">
            To
          </label>
          <Input
            id="date-to"
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className="w-40"
          />
        </div>
      </div>

      {/* Table or empty state */}
      {isEmpty ? (
        <div className="flex flex-col items-center justify-center gap-4 py-16">
          <p className="text-muted-foreground text-lg">No audit entries found</p>
          <p className="text-muted-foreground text-sm">
            {entries && entries.length === 0
              ? "No audit entries have been recorded yet."
              : "Try adjusting the filter criteria above."}
          </p>
        </div>
      ) : (
        <DataTable columns={columns} data={filteredEntries} />
      )}
    </div>
  );
}
