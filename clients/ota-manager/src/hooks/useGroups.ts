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

  const data: Group[] = (query.data?.items ?? []).map((g) => ({
    id: g.id,
    name: g.name,
    description: g.description || null,
    memberCount: g.device_count,
    createdAt: g.created_at,
  }));

  return { ...query, data };
}
