import { useState, useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { DataTable } from "@/components/data-table/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
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
import { Input } from "@/components/ui/input";
import { useToast } from "@/hooks/use-toast";
import { useGroups } from "@/hooks/useGroups";
import { useCreateGroup } from "@/hooks/useCreateGroup";
import { useDeleteGroup } from "@/hooks/useDeleteGroup";

const createGroupSchema = z.object({
  name: z.string().min(1, "Group name is required").max(128, "Name too long"),
  description: z.string().max(512, "Description too long").optional(),
});

type CreateGroupFormValues = z.infer<typeof createGroupSchema>;

interface Group {
  id: string;
  name: string;
  description: string | null;
  memberCount: number;
  createdAt: string;
}

export default function GroupsPage() {
  const { toast } = useToast();
  const { data: groups, isLoading, isError, error, refetch } = useGroups();
  const createGroupMutation = useCreateGroup();
  const deleteGroupMutation = useDeleteGroup();

  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [groupToDelete, setGroupToDelete] = useState<Group | null>(null);

  const form = useForm<CreateGroupFormValues>({
    resolver: zodResolver(createGroupSchema),
    defaultValues: { name: "", description: "" },
  });

  const columns = useMemo<ColumnDef<Group>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => (
          <Button
            variant="link"
            className="p-0 h-auto font-medium"
            onClick={() => {
              setSelectedGroup(row.original);
              setSheetOpen(true);
            }}
          >
            {row.original.name}
          </Button>
        ),
      },
      {
        accessorKey: "description",
        header: "Description",
        cell: ({ row }) => (
          <span className="text-muted-foreground text-sm">
            {row.original.description ?? "—"}
          </span>
        ),
      },
      {
        accessorKey: "memberCount",
        header: "Member Count",
        cell: ({ row }) => <Badge variant="secondary">{row.original.memberCount}</Badge>,
      },
      {
        accessorKey: "createdAt",
        header: "Created",
        cell: ({ row }) => (
          <span className="text-muted-foreground text-sm">
            {new Date(row.original.createdAt).toLocaleDateString()}
          </span>
        ),
      },
      {
        id: "actions",
        header: "Actions",
        cell: ({ row }) => (
          <Button
            variant="destructive"
            size="sm"
            onClick={() => {
              setGroupToDelete(row.original);
              setDeleteDialogOpen(true);
            }}
          >
            Delete
          </Button>
        ),
      },
    ],
    [],
  );

  const handleCreateGroup = async (values: CreateGroupFormValues) => {
    try {
      await createGroupMutation.mutateAsync(values);
      toast({ title: "Group created", description: `Group "${values.name}" has been created.` });
      setCreateDialogOpen(false);
      form.reset();
    } catch {
      toast({ title: "Failed to create group", variant: "destructive" });
    }
  };

  const handleDeleteGroup = async () => {
    if (!groupToDelete) return;
    try {
      await deleteGroupMutation.mutateAsync(groupToDelete.id);
      toast({ title: "Group deleted", description: `Group "${groupToDelete.name}" has been deleted.` });
    } catch {
      toast({ title: "Failed to delete group", variant: "destructive" });
    } finally {
      setDeleteDialogOpen(false);
      setGroupToDelete(null);
    }
  };

  // Loading skeleton state
  if (isLoading) {
    return (
      <div className="space-y-4 p-6">
        <div className="flex items-center justify-between">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-10 w-32" />
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
        <p className="text-destructive text-lg font-semibold">Failed to load device groups</p>
        <p className="text-muted-foreground text-sm">{(error as Error)?.message ?? "An unexpected error occurred."}</p>
        <Button variant="outline" onClick={() => refetch()}>
          Retry
        </Button>
      </div>
    );
  }

  // Empty state
  const isEmpty = !groups || groups.length === 0;

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Device Groups</h1>
          <p className="text-muted-foreground mt-1">Organise devices into groups for targeted deployments.</p>
        </div>
        <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
          <DialogTrigger asChild>
            <Button>Create Group</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create Group</DialogTitle>
              <DialogDescription>Create a new device group to organise your devices.</DialogDescription>
            </DialogHeader>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(handleCreateGroup)} className="space-y-4">
                <FormField
                  control={form.control}
                  name="name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Name</FormLabel>
                      <FormControl>
                        <Input placeholder="Production devices" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="description"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Description (optional)</FormLabel>
                      <FormControl>
                        <Input placeholder="Devices deployed in production" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <DialogFooter>
                  <Button type="submit" disabled={createGroupMutation.isPending}>
                    {createGroupMutation.isPending ? "Creating…" : "Create"}
                  </Button>
                </DialogFooter>
              </form>
            </Form>
          </DialogContent>
        </Dialog>
      </div>

      {isEmpty ? (
        <div className="flex flex-col items-center justify-center gap-4 py-16">
          <p className="text-muted-foreground text-lg">No device groups yet</p>
          <p className="text-muted-foreground text-sm">Create your first group to get started.</p>
        </div>
      ) : (
        <DataTable columns={columns} data={groups} />
      )}

      {/* Group detail sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{selectedGroup?.name}</SheetTitle>
            <SheetDescription>{selectedGroup?.description ?? "No description"}</SheetDescription>
          </SheetHeader>
          {selectedGroup && <GroupDetailContent groupId={selectedGroup.id} />}
        </SheetContent>
      </Sheet>

      {/* Delete confirmation dialog */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Group</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete "{groupToDelete?.name}"? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDeleteGroup} className="bg-destructive text-destructive-foreground">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function GroupDetailContent({ groupId }: { groupId: string }) {
  // Placeholder for member list and add/remove member functionality.
  // The group-membership hooks (useGroupMembers/useAddGroupMember) exist; wiring
  // the member table here is deferred (see EVIDENCE.md).
  return (
    <div className="mt-6 space-y-4">
      <p className="text-muted-foreground text-sm">
        Member management for group <span className="font-mono">{groupId}</span> will be implemented with the device
        membership API.
      </p>
    </div>
  );
}
