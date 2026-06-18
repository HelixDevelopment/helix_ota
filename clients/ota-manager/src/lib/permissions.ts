// ──────────────────────────────────────────────────────────
// Permission helpers — RBAC checks for the OTA Manager UI.
// Roles: viewer (read-only), operator (manage), admin (admin),
//        device (device-scoped), super_admin (unrestricted).
// ──────────────────────────────────────────────────────────

import type { Role, ProjectAccess } from '../types/api';

// ── Role hierarchy ─────────────────────────────
// Higher index = more privileged.

const ROLE_HIERARCHY: Record<Role, number> = {
  viewer: 0,
  device: 0,
  operator: 1,
  admin: 2,
  super_admin: 3,
};

function roleWeight(role: Role): number {
  return ROLE_HIERARCHY[role] ?? -1;
}

// ── Resource-action permission matrix ──────────
// Resource groups map to the API route categories.
// Each entry lists the minimum role level required for the action.

type ResourceAction = 'view' | 'create' | 'update' | 'delete';

const RESOURCE_PERMISSIONS: Record<string, Record<ResourceAction, Role>> = {
  devices:          { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  artifacts:        { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  deltas:           { view: 'viewer',   create: 'operator', update: 'admin',    delete: 'admin' },
  releases:         { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  deployments:      { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  rollouts:         { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  groups:           { view: 'viewer',   create: 'operator', update: 'operator', delete: 'admin' },
  group_members:    { view: 'viewer',   create: 'operator', update: 'operator', delete: 'operator' },
  audit:            { view: 'admin',    create: 'admin',     update: 'admin',    delete: 'admin' },
  telemetry:        { view: 'viewer',   create: 'device',   update: 'admin',    delete: 'admin' },
  projects:         { view: 'viewer',   create: 'admin',    update: 'admin',    delete: 'super_admin' },
};

// ── Public helpers ─────────────────────────────

/**
 * Returns true when `role` has sufficient privilege for `action` on `resource`.
 */
export function can(role: Role, action: ResourceAction, resource: string): boolean {
  const required = RESOURCE_PERMISSIONS[resource]?.[action];
  if (!required) {
    // Unknown resource: default to admin-only for safety.
    return roleWeight(role) >= roleWeight('admin');
  }
  return roleWeight(role) >= roleWeight(required);
}

/** Convenience: may this role view the given resource? */
export function canView(role: Role, resource: string): boolean {
  return can(role, 'view', resource);
}

/** Convenience: may this role create on the given resource? */
export function canCreate(role: Role, resource: string): boolean {
  return can(role, 'create', resource);
}

/** Convenience: may this role update the given resource? */
export function canUpdate(role: Role, resource: string): boolean {
  return can(role, 'update', resource);
}

/** Convenience: may this role delete on the given resource? */
export function canDelete(role: Role, resource: string): boolean {
  return can(role, 'delete', resource);
}

/** Returns true when role can read audit logs. */
export function canViewAudit(role: Role): boolean {
  return roleWeight(role) >= roleWeight('admin');
}

/** Returns true when role can delete groups (admin-only on the server). */
export function canDeleteGroup(role: Role): boolean {
  return roleWeight(role) >= roleWeight('admin');
}

// ── Project scoping ────────────────────────────

/**
 * Returns true when the given project ID appears in the user's
 * project-access list.
 */
export function hasProjectAccess(
  access: ProjectAccess[],
  projectId: string,
): boolean {
  return access.some((a) => a.project_id === projectId);
}

/**
 * Returns the effective role for a project, or null when the user
 * has no access.
 */
export function projectRole(
  access: ProjectAccess[],
  projectId: string,
): Role | null {
  return access.find((a) => a.project_id === projectId)?.role ?? null;
}

// ── List filtering ─────────────────────────────

/**
 * Filters an array of items down to those whose string keyed by
 * `resourceKey` the user's role can access.
 */
export function filterByRole<T>(
  items: T[],
  role: Role,
  resourceKey: keyof T,
): T[] {
  return items.filter((item) => {
    const resource = String(item[resourceKey]);
    return canView(role, resource);
  });
}

// ── Role display helpers ───────────────────────

const ROLE_LABELS: Record<Role, string> = {
  viewer: 'Viewer',
  operator: 'Operator',
  admin: 'Admin',
  device: 'Device',
  super_admin: 'Super Admin',
};

export function roleLabel(role: Role): string {
  return ROLE_LABELS[role] ?? role;
}

export function isSuperAdmin(role: Role): boolean {
  return role === 'super_admin';
}

export function isAdmin(role: Role): boolean {
  return roleWeight(role) >= roleWeight('admin');
}

export function isOperator(role: Role): boolean {
  return roleWeight(role) >= roleWeight('operator');
}

export function isReadOnly(role: Role): boolean {
  return roleWeight(role) <= roleWeight('viewer');
}
