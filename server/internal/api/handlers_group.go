package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// GroupList is the GET /groups body.
type GroupList struct {
	Items []GroupView `json:"items"`
}

// MemberAdd is the POST /groups/{id}/members body.
type MemberAdd struct {
	DeviceID string `json:"device_id"`
}

// GroupMembers is the GET /groups/{id}/members body.
type GroupMembers struct {
	GroupID   string   `json:"group_id"`
	DeviceIDs []string `json:"device_ids"`
}

func toGroupView(g store.Group) GroupView {
	return GroupView{ID: g.ID, Name: g.Name, Description: g.Description, CreatedAt: g.CreatedAt}
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
	groups, err := s.repo.ListGroups(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not list groups")
		return
	}
	items := make([]GroupView, 0, len(groups))
	for _, g := range groups {
		items = append(items, toGroupView(g))
	}
	c.JSON(http.StatusOK, GroupList{Items: items})
}

func (s *Server) handleGetGroup(c *gin.Context) {
	g, err := s.repo.GetGroup(c.Request.Context(), c.Param("groupId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	c.JSON(http.StatusOK, toGroupView(g))
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
	c.JSON(http.StatusOK, toGroupView(existing))
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
	members, err := s.repo.ListGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
		return
	}
	if members == nil {
		members = []string{}
	}
	c.JSON(http.StatusOK, GroupMembers{GroupID: groupID, DeviceIDs: members})
}

func (s *Server) handleAddGroupMember(c *gin.Context) {
	var req MemberAdd
	if err := bindJSON(c, &req); err != nil {
		respondValidation(c, "malformed member body")
		return
	}
	if req.DeviceID == "" {
		respondValidation(c, "device_id is required", ErrorDetail{Field: "device_id", Issue: "required"})
		return
	}
	if err := s.repo.AddGroupMember(c.Request.Context(), c.Param("groupId"), req.DeviceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, CodeNotFound, "group not found")
			return
		}
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not add member")
		return
	}
	c.Status(http.StatusNoContent)
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
