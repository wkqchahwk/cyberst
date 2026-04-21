package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"cyberstrike-ai/internal/attackchain"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type AttackChainHandler struct {
	db           *database.DB
	logger       *zap.Logger
	openAIConfig *config.OpenAIConfig
	mu           sync.RWMutex // 保护 openAIConfig 的并发访问
	// English note.
	generatingLocks sync.Map // map[string]*sync.Mutex
}

// English note.
func NewAttackChainHandler(db *database.DB, openAIConfig *config.OpenAIConfig, logger *zap.Logger) *AttackChainHandler {
	return &AttackChainHandler{
		db:           db,
		logger:       logger,
		openAIConfig: openAIConfig,
	}
}

// English note.
func (h *AttackChainHandler) UpdateConfig(cfg *config.OpenAIConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.openAIConfig = cfg
	h.logger.Info("AttackChainHandler配置已更新",
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)
}

// English note.
func (h *AttackChainHandler) getOpenAIConfig() *config.OpenAIConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.openAIConfig
}

// English note.
// GET /api/attack-chain/:conversationId
func (h *AttackChainHandler) GetAttackChain(c *gin.Context) {
	conversationID := c.Param("conversationId")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId is required"})
		return
	}

	// English note.
	_, err := h.db.GetConversation(conversationID)
	if err != nil {
		h.logger.Warn("对话不存在", zap.String("conversationId", conversationID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	// English note.
	openAIConfig := h.getOpenAIConfig()
	builder := attackchain.NewBuilder(h.db, openAIConfig, h.logger)
	chain, err := builder.LoadChainFromDatabase(conversationID)
	if err == nil && len(chain.Nodes) > 0 {
		// English note.
		h.logger.Info("返回已存在的攻击链", zap.String("conversationId", conversationID))
		c.JSON(http.StatusOK, chain)
		return
	}

	// English note.
	// English note.
	lockInterface, _ := h.generatingLocks.LoadOrStore(conversationID, &sync.Mutex{})
	lock := lockInterface.(*sync.Mutex)
	
	// English note.
	acquired := lock.TryLock()
	if !acquired {
		h.logger.Info("攻击链正在生成中，请稍后再试", zap.String("conversationId", conversationID))
		c.JSON(http.StatusConflict, gin.H{"error": "攻击链正在生成中，请稍后再试"})
		return
	}
	defer lock.Unlock()

	// English note.
	chain, err = builder.LoadChainFromDatabase(conversationID)
	if err == nil && len(chain.Nodes) > 0 {
		h.logger.Info("返回已存在的攻击链（在锁等待期间已生成）", zap.String("conversationId", conversationID))
		c.JSON(http.StatusOK, chain)
		return
	}

	h.logger.Info("开始生成攻击链", zap.String("conversationId", conversationID))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	chain, err = builder.BuildChainFromConversation(ctx, conversationID)
	if err != nil {
		h.logger.Error("生成攻击链失败", zap.String("conversationId", conversationID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成攻击链失败: " + err.Error()})
		return
	}

	// English note.
	// h.generatingLocks.Delete(conversationID)

	c.JSON(http.StatusOK, chain)
}

// English note.
// POST /api/attack-chain/:conversationId/regenerate
func (h *AttackChainHandler) RegenerateAttackChain(c *gin.Context) {
	conversationID := c.Param("conversationId")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversationId is required"})
		return
	}

	// English note.
	_, err := h.db.GetConversation(conversationID)
	if err != nil {
		h.logger.Warn("对话不存在", zap.String("conversationId", conversationID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	// English note.
	if err := h.db.DeleteAttackChain(conversationID); err != nil {
		h.logger.Warn("删除旧攻击链失败", zap.Error(err))
	}

	// English note.
	lockInterface, _ := h.generatingLocks.LoadOrStore(conversationID, &sync.Mutex{})
	lock := lockInterface.(*sync.Mutex)
	
	acquired := lock.TryLock()
	if !acquired {
		h.logger.Info("攻击链正在生成中，请稍后再试", zap.String("conversationId", conversationID))
		c.JSON(http.StatusConflict, gin.H{"error": "攻击链正在生成中，请稍后再试"})
		return
	}
	defer lock.Unlock()

	// English note.
	h.logger.Info("重新生成攻击链", zap.String("conversationId", conversationID))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	openAIConfig := h.getOpenAIConfig()
	builder := attackchain.NewBuilder(h.db, openAIConfig, h.logger)
	chain, err := builder.BuildChainFromConversation(ctx, conversationID)
	if err != nil {
		h.logger.Error("生成攻击链失败", zap.String("conversationId", conversationID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成攻击链失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, chain)
}

