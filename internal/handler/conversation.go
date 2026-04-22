package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cyberstrike-ai/internal/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type ConversationHandler struct {
	db     *database.DB
	logger *zap.Logger
}

// English note.
func NewConversationHandler(db *database.DB, logger *zap.Logger) *ConversationHandler {
	return &ConversationHandler{
		db:     db,
		logger: logger,
	}
}

// English note.
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// English note.
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := req.Title
	if title == "" {
		title = ""
	}

	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// English note.
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	search := c.Query("search") // 

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	conversations, err := h.db.ListConversations(limit, offset, search)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversations)
}

// English note.
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	id := c.Param("id")

	// English note.
	// English note.
	includeStr := c.DefaultQuery("include_process_details", "0")
	include := includeStr == "1" || includeStr == "true" || includeStr == "yes"

	var (
		conv *database.Conversation
		err  error
	)
	if include {
		conv, err = h.db.GetConversation(id)
	} else {
		conv, err = h.db.GetConversationLite(id)
	}
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// English note.
func (h *ConversationHandler) GetMessageProcessDetails(c *gin.Context) {
	messageID := c.Param("id")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message id required"})
		return
	}

	details, err := h.db.GetProcessDetails(messageID)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	out := make([]map[string]interface{}, 0, len(details))
	for _, d := range details {
		var data interface{}
		if d.Data != "" {
			if err := json.Unmarshal([]byte(d.Data), &data); err != nil {
				h.logger.Warn("", zap.Error(err))
			}
		}
		out = append(out, map[string]interface{}{
			"id":             d.ID,
			"messageId":      d.MessageID,
			"conversationId": d.ConversationID,
			"eventType":      d.EventType,
			"message":        d.Message,
			"data":           data,
			"createdAt":      d.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"processDetails": out})
}

// English note.
type UpdateConversationRequest struct {
	Title string `json:"title"`
}

// English note.
func (h *ConversationHandler) UpdateConversation(c *gin.Context) {
	id := c.Param("id")

	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	if err := h.db.UpdateConversationTitle(id, req.Title); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	conv, err := h.db.GetConversation(id)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// English note.
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.DeleteConversation(id); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
type DeleteTurnRequest struct {
	MessageID string `json:"messageId"`
}

// English note.
func (h *ConversationHandler) DeleteConversationTurn(c *gin.Context) {
	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation id required"})
		return
	}

	var req DeleteTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MessageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messageId required"})
		return
	}

	if _, err := h.db.GetConversation(conversationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	deletedIDs, err := h.db.DeleteConversationTurn(conversationID, req.MessageID)
	if err != nil {
		h.logger.Warn("",
			zap.String("conversationId", conversationID),
			zap.String("messageId", req.MessageID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deletedMessageIds": deletedIDs,
		"message":           "ok",
	})
}

