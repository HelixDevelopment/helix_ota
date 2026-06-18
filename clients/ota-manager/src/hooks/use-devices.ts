// ──────────────────────────────────────────────────────────
// Device hooks — registration, status, listing, telemetry.
// ──────────────────────────────────────────────────────────

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type {
  DeviceRegistration,
  DeviceRegistered,
  DeviceStatus,
  DeviceListFilter,
  DeviceList,
  TelemetryHistory,
  TelemetryFilter,
  TelemetryOverview,
} from '../types/api';
import { apiGet, apiPost } from '../lib/api-client';

export const deviceKeys = {
  all: ['devices'] as const,
  lists: () => [...deviceKeys.all, 'list'] as const,
  list: (filters?: DeviceListFilter) => [...deviceKeys.lists(), filters] as const,
  details: () => [...deviceKeys.all, 'detail'] as const,
  detail: (id: string) => [...deviceKeys.details(), id] as const,
  telemetry: (id: string, filters?: TelemetryFilter) =>
    [...deviceKeys.all, 'telemetry', id, filters] as const,
  overview: () => [...deviceKeys.all, 'overview'] as const,
};

// ── Register device ────────────────────────────

export function useRegisterDevice() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (registration: DeviceRegistration) =>
      apiPost<DeviceRegistered>('/devices/register', registration),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: deviceKeys.lists() });
    },
  });
}

// ── Device status (single) ─────────────────────

export function useDeviceStatus(deviceId: string) {
  return useQuery({
    queryKey: deviceKeys.detail(deviceId),
    queryFn: () => apiGet<DeviceStatus>(`/devices/${deviceId}/status`),
    enabled: !!deviceId,
    refetchInterval: 30_000, // poll every 30 s for live status
  });
}

// ── Device list (paginated, filterable) ────────

export function useDevices(filters?: DeviceListFilter) {
  return useQuery({
    queryKey: deviceKeys.list(filters),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (filters?.group) params.group = filters.group;
      if (filters?.os) params.os = filters.os;
      if (filters?.model) params.model = filters.model;
      if (filters?.status) params.status = filters.status;
      if (filters?.cursor) params.cursor = filters.cursor;
      if (filters?.limit !== undefined) params.limit = filters.limit;
      return apiGet<DeviceList>('/devices', { params });
    },
  });
}

// ── Device telemetry ───────────────────────────

export function useDeviceTelemetry(deviceId: string, filters?: TelemetryFilter) {
  return useQuery({
    queryKey: deviceKeys.telemetry(deviceId, filters),
    queryFn: () => {
      const params: Record<string, string | number | undefined> = {};
      if (filters?.event) params.event = filters.event;
      if (filters?.since) params.since = filters.since;
      if (filters?.until) params.until = filters.until;
      if (filters?.cursor) params.cursor = filters.cursor;
      if (filters?.limit !== undefined) params.limit = filters.limit;
      return apiGet<TelemetryHistory>(`/devices/${deviceId}/telemetry`, { params });
    },
    enabled: !!deviceId,
  });
}

// ── Telemetry overview (fleet-wide) ────────────

export function useTelemetryOverview() {
  return useQuery({
    queryKey: deviceKeys.overview(),
    queryFn: () => apiGet<TelemetryOverview>('/telemetry/overview'),
    refetchInterval: 60_000, // fleet overview refreshes every 60 s
  });
}
