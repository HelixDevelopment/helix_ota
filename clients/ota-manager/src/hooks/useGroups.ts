// Adapter shim: maps the react-query group-list result onto the view shape the
// GroupsPage table consumes.
import { useGroups as useGroupsQuery } from "./use-groups";

export { useGroup, useUpdateGroup, useDeleteGroup, useGroupMembers, useAddGroupMember, useRemoveGroupMember, groupKeys } from "./use-groups";

export interface Group {
  id: string;
  name: string;
  description: string | null;
  memberCount: number;
  createdAt: string;
}

export function useGroups() {
  const query = useGroupsQuery();

  // g.group_id/g.member_count are the REAL Group fields — server/internal/
  // api/handlers_group.go:54-60 (§11.4.6/§11.4.108: previously `g.id`/
  // `g.device_count`, neither of which exists on the wire).
  const data: Group[] = (query.data?.items ?? []).map((g) => ({
    id: g.group_id,
    name: g.name,
    description: g.description || null,
    memberCount: g.member_count,
    createdAt: g.created_at,
  }));

  return { ...query, data };
}
