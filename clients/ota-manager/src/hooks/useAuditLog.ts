// Adapter shim: maps the react-query AuditLogList onto a flat view-entry array
// the AuditPage table and the DashboardPage activity feed consume.
import { useAuditLog as useAuditLogQuery } from "./use-audit";
import type { AuditFilter } from "../types/api";

export interface AuditView {
  id: string;
  timestamp: string;
  action: string;
  actor: string;
  target: string;
  detail: string;
}

export function useAuditLog(filters?: AuditFilter) {
  const query = useAuditLogQuery(filters);

  const data: AuditView[] = (query.data?.items ?? []).map((e) => ({
    id: e.id,
    timestamp: e.created_at,
    action: e.action,
    actor: e.actor,
    target: e.resource_id ? `${e.resource_type}/${e.resource_id}` : e.resource_type,
    detail: e.details ? JSON.stringify(e.details) : "",
  }));

  return { ...query, data };
}
