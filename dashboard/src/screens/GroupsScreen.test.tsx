// Helix OTA dashboard — component tests for the GroupsScreen trio: GroupList,
// GroupCreateScreen, GroupDetail. Stubs only the apiClient methods + the useAuth
// hook (the latter to drive the RoleGate visibility branches) — both
// unit/component-test mocks permitted §11.4.27. Every assertion exercises real
// user-visible behaviour: the list table + empty/404 states, the create-form
// submit→navigate flow + its error panel, and the batch member-add interaction
// that projects the per-id { added / already_member / not_found } disposition.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { ApiError } from "../api/client";
import type {
  DeviceGroup,
  DeviceGroupList,
  DeviceGroupMembers,
  DeviceGroupMembersAddResult,
} from "../types/api";
import type { Role } from "../types/api";

const listGroups = vi.fn<[], Promise<DeviceGroupList>>();
const createGroup = vi.fn<[unknown], Promise<DeviceGroup>>();
const getGroupMembers = vi.fn<[string], Promise<DeviceGroupMembers>>();
const addGroupMembers = vi.fn<[string, string[]], Promise<DeviceGroupMembersAddResult>>();

vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual,
    apiClient: {
      listGroups: (...a: unknown[]) => listGroups(...(a as [])),
      createGroup: (...a: unknown[]) => createGroup(...(a as [unknown])),
      getGroupMembers: (...a: unknown[]) => getGroupMembers(...(a as [string])),
      addGroupMembers: (...a: unknown[]) => addGroupMembers(...(a as [string, string[]])),
    },
  };
});

// Drive RoleGate through a mocked useAuth (RoleGate reads roles via this hook).
let mockRoles: Role[] = ["operator"];
vi.mock("../auth/AuthContext", () => ({
  useAuth: () => ({ roles: mockRoles }),
}));

// Import AFTER mocks are registered.
import { GroupList, GroupCreateScreen, GroupDetail } from "./GroupsScreen";

beforeEach(() => {
  listGroups.mockReset();
  createGroup.mockReset();
  getGroupMembers.mockReset();
  addGroupMembers.mockReset();
  mockRoles = ["operator"];
});

describe("GroupList", () => {
  function renderList() {
    return render(
      <MemoryRouter>
        <GroupList />
      </MemoryRouter>,
    );
  }

  it("renders a populated group row with name / description / member_count badge", async () => {
    listGroups.mockResolvedValue({
      items: [
        {
          group_id: "grp_1",
          name: "canary-fleet",
          description: "early adopters",
          member_count: 12,
          created_at: "2026-06-10T00:00:00Z",
        },
      ],
    });
    renderList();
    await waitFor(() => expect(screen.getByText("canary-fleet")).toBeInTheDocument());
    expect(screen.getByText("early adopters")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    // The members link points at the group detail route.
    expect(screen.getByRole("link", { name: "members" })).toHaveAttribute(
      "href",
      "/groups/grp_1",
    );
  });

  it("renders the em-dash for a group with no description", async () => {
    listGroups.mockResolvedValue({
      items: [
        { group_id: "grp_2", name: "all", member_count: 0, created_at: "2026-06-10T00:00:00Z" },
      ],
    });
    renderList();
    await waitFor(() => expect(screen.getByText("all")).toBeInTheDocument());
    const row = screen.getByText("all").closest("tr")!;
    expect(within(row).getByText("—")).toBeInTheDocument();
  });

  it("renders the 'No groups yet.' empty state", async () => {
    listGroups.mockResolvedValue({ items: [] });
    renderList();
    await waitFor(() => expect(screen.getByText("No groups yet.")).toBeInTheDocument());
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("degrades to a 404 empty state (NOT an error alert) when groups are unavailable", async () => {
    listGroups.mockRejectedValue(new ApiError(404, "NOT_FOUND", "no groups", "req-1"));
    renderList();
    await waitFor(() =>
      expect(
        screen.getByText("Device groups are not available (endpoint returned 404)."),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces a non-404 ApiError via the error panel", async () => {
    listGroups.mockRejectedValue(new ApiError(500, "INTERNAL", "boom", "req-2"));
    renderList();
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("500 INTERNAL");
    });
  });

  it("shows the 'New group' action for operators but hides it for viewers (RoleGate)", async () => {
    listGroups.mockResolvedValue({ items: [] });
    const { unmount } = renderList();
    await waitFor(() => expect(screen.getByText("No groups yet.")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "New group" })).toBeInTheDocument();
    unmount();

    // A viewer session has no operator/admin role → the gate hides the action.
    mockRoles = ["viewer"];
    renderList();
    await waitFor(() => expect(screen.getByText("No groups yet.")).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "New group" })).not.toBeInTheDocument();
  });
});

describe("GroupCreateScreen", () => {
  function renderCreate() {
    return render(
      <MemoryRouter initialEntries={["/groups/new"]}>
        <Routes>
          <Route path="/groups/new" element={<GroupCreateScreen />} />
          <Route path="/groups/:groupId" element={<div>group detail stub</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("disables Create until a name is entered, then submits + navigates to the new group", async () => {
    createGroup.mockResolvedValue({
      group_id: "grp_new",
      name: "qa-ring",
      member_count: 0,
      created_at: "2026-06-10T00:00:00Z",
    });
    renderCreate();

    const submit = screen.getByRole("button", { name: "Create group" });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByText("name").parentElement!.querySelector("input")!, {
      target: { value: "qa-ring" },
    });
    fireEvent.change(
      screen.getByText("description (optional)").parentElement!.querySelector("input")!,
      { target: { value: "qa devices" } },
    );
    await waitFor(() => expect(submit).toBeEnabled());

    fireEvent.click(submit);

    // On success the screen navigates to the new group's detail route.
    await waitFor(() => expect(screen.getByText("group detail stub")).toBeInTheDocument());
    expect(createGroup).toHaveBeenCalledTimes(1);
    expect(createGroup.mock.calls[0][0]).toEqual({ name: "qa-ring", description: "qa devices" });
  });

  it("surfaces a 409 name-conflict via the error panel and does NOT navigate", async () => {
    createGroup.mockRejectedValue(new ApiError(409, "CONFLICT", "name taken", "req-409"));
    renderCreate();
    fireEvent.change(screen.getByText("name").parentElement!.querySelector("input")!, {
      target: { value: "dupe" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create group" }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("409 CONFLICT");
      expect(alert).toHaveTextContent("name taken");
    });
    // Still on the create screen — navigation did NOT happen.
    expect(screen.queryByText("group detail stub")).not.toBeInTheDocument();
  });
});

describe("GroupDetail", () => {
  const GROUP_ID = "grp_show";

  function renderDetail() {
    return render(
      <MemoryRouter initialEntries={[`/groups/${GROUP_ID}`]}>
        <Routes>
          <Route path="/groups/:groupId" element={<GroupDetail />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("renders the members table when the group has members", async () => {
    getGroupMembers.mockResolvedValue({
      group_id: GROUP_ID,
      items: [
        { device_id: "dev_001", added_at: "2026-06-10T00:00:00Z" },
        { device_id: "dev_002", added_at: "2026-06-10T01:00:00Z" },
      ],
      next_cursor: null,
    });
    renderDetail();
    await waitFor(() => expect(screen.getByText("dev_001")).toBeInTheDocument());
    expect(screen.getByText("dev_002")).toBeInTheDocument();
    // Each member links to its fleet device-detail screen.
    const links = screen.getAllByRole("link", { name: "device" });
    expect(links[0]).toHaveAttribute("href", "/fleet/dev_001");
  });

  it("renders the empty-members state", async () => {
    getGroupMembers.mockResolvedValue({ group_id: GROUP_ID, items: [], next_cursor: null });
    renderDetail();
    await waitFor(() =>
      expect(screen.getByText("No members in this group yet.")).toBeInTheDocument(),
    );
  });

  it("degrades to a 404 group-not-found empty state", async () => {
    getGroupMembers.mockRejectedValue(new ApiError(404, "NOT_FOUND", "gone", "req-1"));
    renderDetail();
    await waitFor(() =>
      expect(
        screen.getByText("This group was not found (endpoint returned 404)."),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("batch-adds device_ids and projects the per-id disposition + refetches members", async () => {
    // First members load: one member.
    getGroupMembers
      .mockResolvedValueOnce({
        group_id: GROUP_ID,
        items: [{ device_id: "dev_001", added_at: "2026-06-10T00:00:00Z" }],
        next_cursor: null,
      })
      // After the add, a refetch returns the expanded membership.
      .mockResolvedValueOnce({
        group_id: GROUP_ID,
        items: [
          { device_id: "dev_001", added_at: "2026-06-10T00:00:00Z" },
          { device_id: "dev_002", added_at: "2026-06-10T02:00:00Z" },
        ],
        next_cursor: null,
      });
    addGroupMembers.mockResolvedValue({
      added: ["dev_002"],
      already_member: ["dev_001"],
      not_found: ["dev_999"],
    });

    renderDetail();
    await waitFor(() => expect(screen.getByText("dev_001")).toBeInTheDocument());

    // Enter three device ids (newline + comma separated) into the batch textarea.
    const textarea = screen.getByPlaceholderText(/dev_001/) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "dev_002, dev_001\ndev_999" } });

    // The button label reflects the parsed count.
    const addBtn = screen.getByRole("button", { name: /Add 3 device/ });
    fireEvent.click(addBtn);

    // The disposition badges render the per-id breakdown.
    await waitFor(() => expect(screen.getByText("added: 1")).toBeInTheDocument());
    expect(screen.getByText("already_member: 1")).toBeInTheDocument();
    expect(screen.getByText("not_found: 1")).toBeInTheDocument();
    // The not_found id is listed under its disposition.
    expect(screen.getByText("dev_999")).toBeInTheDocument();

    // addGroupMembers was called with the parsed id list (order preserved).
    expect(addGroupMembers).toHaveBeenCalledTimes(1);
    expect(addGroupMembers.mock.calls[0][0]).toBe(GROUP_ID);
    expect(addGroupMembers.mock.calls[0][1]).toEqual(["dev_002", "dev_001", "dev_999"]);

    // Members were refetched → dev_002 now appears BOTH in the batch-add
    // disposition AND in the refetched members table (≥2 occurrences). If the
    // refetch had not happened, dev_002 would appear only once (disposition).
    await waitFor(() =>
      expect(screen.getAllByText("dev_002").length).toBeGreaterThanOrEqual(2),
    );
  });

  it("surfaces an add-members error via the error panel", async () => {
    getGroupMembers.mockResolvedValue({ group_id: GROUP_ID, items: [], next_cursor: null });
    addGroupMembers.mockRejectedValue(
      new ApiError(422, "VALIDATION", "bad ids", "req-422"),
    );
    renderDetail();
    await waitFor(() =>
      expect(screen.getByText("No members in this group yet.")).toBeInTheDocument(),
    );
    const textarea = screen.getByPlaceholderText(/dev_001/) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "dev_x" } });
    fireEvent.click(screen.getByRole("button", { name: /Add 1 device/ }));
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("422 VALIDATION");
    });
  });

  it("hides the batch-add card entirely for a viewer (RoleGate)", async () => {
    mockRoles = ["viewer"];
    getGroupMembers.mockResolvedValue({ group_id: GROUP_ID, items: [], next_cursor: null });
    renderDetail();
    await waitFor(() =>
      expect(screen.getByText("No members in this group yet.")).toBeInTheDocument(),
    );
    // The operator-only batch-add card title is absent.
    expect(screen.queryByText("Add devices (batch)")).not.toBeInTheDocument();
  });
});
