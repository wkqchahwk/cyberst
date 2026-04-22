package handler

import (
	"net/http"
	"time"

	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type GroupHandler struct {
	db     *database.DB
	logger *zap.Logger
}

// English note.
func NewGroupHandler(db *database.DB, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		db:     db,
		logger: logger,
	}
}

// English note.
type CreateGroupRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// English note.
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	group, err := h.db.CreateGroup(req.Name, req.Icon)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		// English note.
		if err.Error() == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": ""})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// English note.
func (h *GroupHandler) ListGroups(c *gin.Context) {
	groups, err := h.db.ListGroups()
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// English note.
func (h *GroupHandler) GetGroup(c *gin.Context) {
	id := c.Param("id")

	group, err := h.db.GetGroup(id)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	c.JSON(http.StatusOK, group)
}

// English note.
type UpdateGroupRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// English note.
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	if err := h.db.UpdateGroup(id, req.Name, req.Icon); err != nil {
		h.logger.Error("", zap.Error(err))
		// English note.
		if err.Error() == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": ""})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	group, err := h.db.GetGroup(id)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// English note.
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.DeleteGroup(id); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
type AddConversationToGroupRequest struct {
	ConversationID string `json:"conversationId"`
	GroupID        string `json:"groupId"`
}

// English note.
func (h *GroupHandler) AddConversationToGroup(c *gin.Context) {
	var req AddConversationToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.AddConversationToGroup(req.ConversationID, req.GroupID); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
func (h *GroupHandler) RemoveConversationFromGroup(c *gin.Context) {
	conversationID := c.Param("conversationId")
	groupID := c.Param("id")

	if err := h.db.RemoveConversationFromGroup(conversationID, groupID); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
type GroupConversation struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Pinned      bool      `json:"pinned"`
	GroupPinned bool      `json:"groupPinned"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// English note.
func (h *GroupHandler) GetGroupConversations(c *gin.Context) {
	groupID := c.Param("id")
	searchQuery := c.Query("search") // 

	var conversations []*database.Conversation
	var err error

	// English note.
	if searchQuery != "" {
		conversations, err = h.db.SearchConversationsByGroup(groupID, searchQuery)
	} else {
		conversations, err = h.db.GetConversationsByGroup(groupID)
	}

	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	groupConvs := make([]GroupConversation, 0, len(conversations))
	for _, conv := range conversations {
		// English note.
		var groupPinned int
		err := h.db.QueryRow(
			"SELECT COALESCE(pinned, 0) FROM conversation_group_mappings WHERE conversation_id = ? AND group_id = ?",
			conv.ID, groupID,
		).Scan(&groupPinned)
		if err != nil {
			h.logger.Warn("", zap.String("conversationId", conv.ID), zap.Error(err))
			groupPinned = 0
		}

		groupConvs = append(groupConvs, GroupConversation{
			ID:          conv.ID,
			Title:       conv.Title,
			Pinned:      conv.Pinned,
			GroupPinned: groupPinned != 0,
			CreatedAt:   conv.CreatedAt,
			UpdatedAt:   conv.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, groupConvs)
}

// English note.
func (h *GroupHandler) GetAllMappings(c *gin.Context) {
	mappings, err := h.db.GetAllGroupMappings()
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, mappings)
}

// English note.
type UpdateConversationPinnedRequest struct {
	Pinned bool `json:"pinned"`
}

// English note.
func (h *GroupHandler) UpdateConversationPinned(c *gin.Context) {
	conversationID := c.Param("id")

	var req UpdateConversationPinnedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateConversationPinned(conversationID, req.Pinned); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
type UpdateGroupPinnedRequest struct {
	Pinned bool `json:"pinned"`
}

// English note.
func (h *GroupHandler) UpdateGroupPinned(c *gin.Context) {
	groupID := c.Param("id")

	var req UpdateGroupPinnedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateGroupPinned(groupID, req.Pinned); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
type UpdateConversationPinnedInGroupRequest struct {
	Pinned bool `json:"pinned"`
}

// English note.
func (h *GroupHandler) UpdateConversationPinnedInGroup(c *gin.Context) {
	groupID := c.Param("id")
	conversationID := c.Param("conversationId")

	var req UpdateConversationPinnedInGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateConversationPinnedInGroup(conversationID, groupID, req.Pinned); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}
