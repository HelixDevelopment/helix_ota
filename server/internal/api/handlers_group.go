package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// parsePage reads the shared ?limit/?cursor pagination params. limit defaults to
// def and is capped at max; cursor is a non-negative integer offset. On a
// malformed value it writes a 400 VALIDATION_FAILED and returns ok=false.
func parsePage(c *gin.Context, def, max int) (limit, offset int, ok bool) {
	limit = def
	if lim := c.Query("limit"); lim != "" {
		n, err := strconv.Atoi(lim)
		if err != nil || n < 1 || n > max {
			respondValidation(c, "limit must be an integer in [1,"+strconv.Itoa(max)+"]",
				ErrorDetail{Field: "limit", Issue: "out of range"})
			return 0, 0, false
		}
		limit = n
	}
	if cur := c.Query("cursor"); cur != "" {
		n, err := strconv.Atoi(cur)
		if err != nil || n < 0 {
			respondValidation(c, "cursor is malformed", ErrorDetail{Field: "cursor", Issue: "invalid"})
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}

// --- wire types (operational_endpoints.md §6) ---

// GroupCreate is the POST /groups body.
type GroupCreate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GroupUpdate is the PATCH /groups/{id} body.
type GroupUpdate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GroupView is a device-group response.
type GroupView struct {
	GroupID     string    `json:"group_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// GroupList is the GET /groups body — cursor-paginated. NextCursor is nil on the
// last page.
type GroupList struct {
	Items      []GroupView `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

// MemberAdd is the POST /groups/{id}/members body — a BATCH of device ids
// (operational_endpoints.md §6.4).
type MemberAdd struct {
	DeviceIDs []string `json:"device_ids"`
}

// MemberAddResult is the 200 response of a batch member-add: which ids were
// newly added, were already members, or are not registered devices.
type MemberAddResult struct {
	Added         []string `json:"added"`
	AlreadyMember []string `json:"already_member"`
	NotFound      []string `json:"not_found"`
}

// GroupMemberView is one membership with its join time.
type GroupMemberView struct {
	DeviceID string    `json:"device_id"`
	AddedAt  time.Time `json:"added_at"`
}

// GroupMembers is the GET /groups/{id}/members body — items of {device_id,
// added_at}, oldest-first and cursor-paginated. NextCursor is nil on the last page.
type GroupMembers struct {
	GroupID    string            `json:"group_id"`
	Items      []GroupMemberView `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

func toGroupView(g store.Group) GroupView {
	return GroupView{GroupID: g.ID, Name: g.Name, Description: g.Description, CreatedAt: g.CreatedAt}
}

func toGroupViewWithCount(g store.Group, members int) GroupView {
	v := toGroupView(g)
	v.MemberCount = members
	return v
}

// groupViewWithMembers builds a GroupView whose member_count reflects the live
// membership (best-effort: a count error degrades to 0, never fails the read).
func (s *Server) groupViewWithMembers(c *gin.Context, g store.Group) GroupView {
	members, err := s.repo.ListGroupMembers(c.Request.Context(), g.ID)
	if err != nil {
		return toGroupView(g)
	}
	return toGroupViewWithCount(g, len(members))
}

// --- handlers ---

func (s *Server) handleCreateGroup(c *gin.Context) {
	var req GroupCreate
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed group body")
		return
	}
	if req.Name == "" {
		respondValidation(c, "name is required", ErrorDetail{Field: "name", Issue: "required"})
		return
	}
	g := store.Group{ID: s.newID(), Name: req.Name, Description: req.Description, CreatedAt: s.now()}
	if err := s.repo.CreateGroup(c.Request.Context(), g); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a group with that name already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not create group")
		return
	}
	c.JSON(http.StatusCreated, toGroupView(g))
}

func (s *Server) handleListGroups(c *gin.Context) {
	limit, offset, ok := parsePage(c, 50, 200)
	if !ok {
		return
	}
	groups, err := s.repo.ListGroups(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list groups")
		return
	}
	items := make([]GroupView, 0, limit)
	for i := offset; i < len(groups) && len(items) < limit; i++ {
		items = append(items, s.groupViewWithMembers(c, groups[i]))
	}
	body := GroupList{Items: items}
	if next := offset + len(items); next < len(groups) {
		nc := strconv.Itoa(next)
		body.NextCursor = &nc
	}
	c.JSON(http.StatusOK, body)
}

func (s *Server) handleGetGroup(c *gin.Context) {
	g, err := s.repo.GetGroup(c.Request.Context(), c.Param("groupId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	c.JSON(http.StatusOK, s.groupViewWithMembers(c, g))
}

func (s *Server) handleUpdateGroup(c *gin.Context) {
	var req GroupUpdate
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed group body")
		return
	}
	existing, err := s.repo.GetGroup(c.Request.Context(), c.Param("groupId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	if err := s.repo.UpdateGroup(c.Request.Context(), existing); err != nil {
		if errors.Is(err, store.ErrConflict) {
			respondError(c, http.StatusConflict, CodeConflict, "a group with that name already exists")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not update group")
		return
	}
	c.JSON(http.StatusOK, s.groupViewWithMembers(c, existing))
}

func (s *Server) handleDeleteGroup(c *gin.Context) {
	if err := s.repo.DeleteGroup(c.Request.Context(), c.Param("groupId")); err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleListGroupMembers(c *gin.Context) {
	groupID := c.Param("groupId")
	limit, offset, ok := parsePage(c, 50, 200)
	if !ok {
		return
	}
	members, err := s.repo.ListGroupMembersDetailed(c.Request.Context(), groupID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	items := make([]GroupMemberView, 0, limit)
	for i := offset; i < len(members) && len(items) < limit; i++ {
		items = append(items, GroupMemberView{DeviceID: members[i].DeviceID, AddedAt: members[i].AddedAt})
	}
	body := GroupMembers{GroupID: groupID, Items: items}
	if next := offset + len(items); next < len(members) {
		nc := strconv.Itoa(next)
		body.NextCursor = &nc
	}
	c.JSON(http.StatusOK, body)
}

func (s *Server) handleAddGroupMembers(c *gin.Context) {
	var req MemberAdd
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed member body")
		return
	}
	if len(req.DeviceIDs) == 0 {
		respondValidation(c, "device_ids is required and must be non-empty",
			ErrorDetail{Field: "device_ids", Issue: "required"})
		return
	}
	ctx := c.Request.Context()
	groupID := c.Param("groupId")
	// Group must exist (a batch into a missing group is a 404, not a partial).
	existingMembers, err := s.repo.ListGroupMembers(ctx, groupID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	already := make(map[string]bool, len(existingMembers))
	for _, m := range existingMembers {
		already[m] = true
	}
	result := MemberAddResult{Added: []string{}, AlreadyMember: []string{}, NotFound: []string{}}
	seen := make(map[string]bool, len(req.DeviceIDs))
	for _, id := range req.DeviceIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		switch {
		case already[id]:
			result.AlreadyMember = append(result.AlreadyMember, id)
		default:
			if _, derr := s.repo.GetDevice(ctx, id); derr != nil {
				result.NotFound = append(result.NotFound, id)
				continue
			}
			if aerr := s.repo.AddGroupMember(ctx, groupID, id, s.now()); aerr != nil {
				if errors.Is(aerr, store.ErrNotFound) {
					respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
					return
				}
				respondError(c, http.StatusInternalServerError, CodeInternal, "could not add member")
				return
			}
			result.Added = append(result.Added, id)
			already[id] = true
		}
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleRemoveGroupMember(c *gin.Context) {
	if err := s.repo.RemoveGroupMember(c.Request.Context(), c.Param("groupId"), c.Param("deviceId")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not remove member")
		return
	}
	c.Status(http.StatusNoContent)
}
