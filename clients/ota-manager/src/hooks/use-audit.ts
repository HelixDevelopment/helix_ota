// ──────────────────────────────────────────────────────────
// Audit hooks — paginated audit log with filters.
// ──────────────────────────────────────────────────────────

import { useQuery } from '@tanstack/react-query';
import type { AuditLogList, AuditFilter } from '../types/api';
import { apiGet } from '../lib/api-client';

export const auditKeys = {
  all: ['audit'] as const,
  list: (filters?: AuditFilter) => [...auditKeys.all, filters] as const,
};

// ── Audit log ──────────────────────────────────

export function useAuditLog(filters?: AuditFilter) {
  return useQuery({
    queryKey: auditKeys.list(filters),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (filters?.action) params.action = filters.action;
      if (filters?.resource_type) params['resource_type'] = filters.resource_type;
      if (filters?.since) params.since = filters.since;
      if (filters?.until) params.until = filters.until;
      if (filters?.cursor) params.cursor = filters.cursor;
      if (filters?.limit !== undefined) params.limit = filters.limit;
      return apiGet<AuditLogList>('/audit', { params });
    },
  });
}
