package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type MonitorHandler struct {
	mcpServer      *mcp.Server
	externalMCPMgr *mcp.ExternalMCPManager
	executor       *security.Executor
	db             *database.DB
	logger         *zap.Logger
}

// English note.
func NewMonitorHandler(mcpServer *mcp.Server, executor *security.Executor, db *database.DB, logger *zap.Logger) *MonitorHandler {
	return &MonitorHandler{
		mcpServer:      mcpServer,
		externalMCPMgr: nil, // 
		executor:       executor,
		db:             db,
		logger:         logger,
	}
}

// English note.
func (h *MonitorHandler) SetExternalMCPManager(mgr *mcp.ExternalMCPManager) {
	h.externalMCPMgr = mgr
}

// English note.
type MonitorResponse struct {
	Executions []*mcp.ToolExecution      `json:"executions"`
	Stats      map[string]*mcp.ToolStats `json:"stats"`
	Timestamp  time.Time                  `json:"timestamp"`
	Total      int                        `json:"total,omitempty"`
	Page       int                        `json:"page,omitempty"`
	PageSize   int                        `json:"page_size,omitempty"`
	TotalPages int                        `json:"total_pages,omitempty"`
}

// English note.
func (h *MonitorHandler) Monitor(c *gin.Context) {
	// English note.
	page := 1
	pageSize := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// English note.
	status := c.Query("status")
	// English note.
	toolName := c.Query("tool")

	executions, total := h.loadExecutionsWithPagination(page, pageSize, status, toolName)
	stats := h.loadStats()

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, MonitorResponse{
		Executions: executions,
		Stats:      stats,
		Timestamp:  time.Now(),
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (h *MonitorHandler) loadExecutions() []*mcp.ToolExecution {
	executions, _ := h.loadExecutionsWithPagination(1, 1000, "", "")
	return executions
}

func (h *MonitorHandler) loadExecutionsWithPagination(page, pageSize int, status, toolName string) ([]*mcp.ToolExecution, int) {
	if h.db == nil {
		allExecutions := h.mcpServer.GetAllExecutions()
		// English note.
		if status != "" || toolName != "" {
			filtered := make([]*mcp.ToolExecution, 0)
			for _, exec := range allExecutions {
				matchStatus := status == "" || exec.Status == status
				// English note.
				matchTool := toolName == "" || strings.Contains(strings.ToLower(exec.ToolName), strings.ToLower(toolName))
				if matchStatus && matchTool {
					filtered = append(filtered, exec)
				}
			}
			allExecutions = filtered
		}
		total := len(allExecutions)
		offset := (page - 1) * pageSize
		end := offset + pageSize
		if end > total {
			end = total
		}
		if offset >= total {
			return []*mcp.ToolExecution{}, total
		}
		return allExecutions[offset:end], total
	}

	offset := (page - 1) * pageSize
	executions, err := h.db.LoadToolExecutionsWithPagination(offset, pageSize, status, toolName)
	if err != nil {
		h.logger.Warn("，", zap.Error(err))
		allExecutions := h.mcpServer.GetAllExecutions()
		// English note.
		if status != "" || toolName != "" {
			filtered := make([]*mcp.ToolExecution, 0)
			for _, exec := range allExecutions {
				matchStatus := status == "" || exec.Status == status
				// English note.
				matchTool := toolName == "" || strings.Contains(strings.ToLower(exec.ToolName), strings.ToLower(toolName))
				if matchStatus && matchTool {
					filtered = append(filtered, exec)
				}
			}
			allExecutions = filtered
		}
		total := len(allExecutions)
		offset := (page - 1) * pageSize
		end := offset + pageSize
		if end > total {
			end = total
		}
		if offset >= total {
			return []*mcp.ToolExecution{}, total
		}
		return allExecutions[offset:end], total
	}

	// English note.
	total, err := h.db.CountToolExecutions(status, toolName)
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		// English note.
		total = offset + len(executions)
		if len(executions) == pageSize {
			total = offset + len(executions) + 1
		}
	}

	return executions, total
}

func (h *MonitorHandler) loadStats() map[string]*mcp.ToolStats {
	// English note.
	stats := make(map[string]*mcp.ToolStats)

	// English note.
	if h.db == nil {
		internalStats := h.mcpServer.GetStats()
		for k, v := range internalStats {
			stats[k] = v
		}
	} else {
		dbStats, err := h.db.LoadToolStats()
		if err != nil {
			h.logger.Warn("，", zap.Error(err))
			internalStats := h.mcpServer.GetStats()
			for k, v := range internalStats {
				stats[k] = v
			}
		} else {
			for k, v := range dbStats {
				stats[k] = v
			}
		}
	}

	// English note.
	if h.externalMCPMgr != nil {
		externalStats := h.externalMCPMgr.GetToolStats()
		for k, v := range externalStats {
			// English note.
			if existing, exists := stats[k]; exists {
				existing.TotalCalls += v.TotalCalls
				existing.SuccessCalls += v.SuccessCalls
				existing.FailedCalls += v.FailedCalls
				// English note.
				if v.LastCallTime != nil && (existing.LastCallTime == nil || v.LastCallTime.After(*existing.LastCallTime)) {
					existing.LastCallTime = v.LastCallTime
				}
			} else {
				stats[k] = v
			}
		}
	}

	return stats
}


// English note.
func (h *MonitorHandler) GetExecution(c *gin.Context) {
	id := c.Param("id")

	// English note.
	exec, exists := h.mcpServer.GetExecution(id)
	if exists {
		c.JSON(http.StatusOK, exec)
		return
	}

	// English note.
	if h.externalMCPMgr != nil {
		exec, exists = h.externalMCPMgr.GetExecution(id)
		if exists {
			c.JSON(http.StatusOK, exec)
			return
		}
	}

	// English note.
	if h.db != nil {
		exec, err := h.db.GetToolExecution(id)
		if err == nil && exec != nil {
			c.JSON(http.StatusOK, exec)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": ""})
}

// English note.
func (h *MonitorHandler) BatchGetToolNames(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := make(map[string]string, len(req.IDs))
	for _, id := range req.IDs {
		// English note.
		if exec, exists := h.mcpServer.GetExecution(id); exists {
			result[id] = exec.ToolName
			continue
		}
		// English note.
		if h.externalMCPMgr != nil {
			if exec, exists := h.externalMCPMgr.GetExecution(id); exists {
				result[id] = exec.ToolName
				continue
			}
		}
		// English note.
		if h.db != nil {
			if exec, err := h.db.GetToolExecution(id); err == nil && exec != nil {
				result[id] = exec.ToolName
			}
		}
	}

	c.JSON(http.StatusOK, result)
}

// English note.
func (h *MonitorHandler) GetStats(c *gin.Context) {
	stats := h.loadStats()
	c.JSON(http.StatusOK, stats)
}

// English note.
func (h *MonitorHandler) DeleteExecution(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID"})
		return
	}

	// English note.
	if h.db != nil {
		// English note.
		exec, err := h.db.GetToolExecution(id)
		if err != nil {
			// English note.
			h.logger.Warn("，", zap.String("executionId", id), zap.Error(err))
			c.JSON(http.StatusOK, gin.H{"message": ""})
			return
		}

		// English note.
		err = h.db.DeleteToolExecution(id)
		if err != nil {
			h.logger.Error("", zap.Error(err), zap.String("executionId", id))
			c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
			return
		}

		// English note.
		totalCalls := 1
		successCalls := 0
		failedCalls := 0
		if exec.Status == "failed" {
			failedCalls = 1
		} else if exec.Status == "completed" {
			successCalls = 1
		}

		if exec.ToolName != "" {
			if err := h.db.DecreaseToolStats(exec.ToolName, totalCalls, successCalls, failedCalls); err != nil {
				h.logger.Warn("", zap.Error(err), zap.String("toolName", exec.ToolName))
				// English note.
			}
		}

		h.logger.Info("", zap.String("executionId", id), zap.String("toolName", exec.ToolName))
		c.JSON(http.StatusOK, gin.H{"message": ""})
		return
	}

	// English note.
	// English note.
	h.logger.Info("", zap.String("executionId", id))
	c.JSON(http.StatusOK, gin.H{"message": "（）"})
}

// English note.
func (h *MonitorHandler) DeleteExecutions(c *gin.Context) {
	var request struct {
		IDs []string `json:"ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	if len(request.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID"})
		return
	}

	// English note.
	if h.db != nil {
		// English note.
		executions, err := h.db.GetToolExecutionsByIds(request.IDs)
		if err != nil {
			h.logger.Error("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
			return
		}

		// English note.
		toolStats := make(map[string]struct {
			totalCalls   int
			successCalls int
			failedCalls  int
		})

		for _, exec := range executions {
			if exec.ToolName == "" {
				continue
			}

			stats := toolStats[exec.ToolName]
			stats.totalCalls++
			if exec.Status == "failed" {
				stats.failedCalls++
			} else if exec.Status == "completed" {
				stats.successCalls++
			}
			toolStats[exec.ToolName] = stats
		}

		// English note.
		err = h.db.DeleteToolExecutions(request.IDs)
		if err != nil {
			h.logger.Error("", zap.Error(err), zap.Int("count", len(request.IDs)))
			c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
			return
		}

		// English note.
		for toolName, stats := range toolStats {
			if err := h.db.DecreaseToolStats(toolName, stats.totalCalls, stats.successCalls, stats.failedCalls); err != nil {
				h.logger.Warn("", zap.Error(err), zap.String("toolName", toolName))
				// English note.
			}
		}

		h.logger.Info("", zap.Int("count", len(request.IDs)))
		c.JSON(http.StatusOK, gin.H{"message": "", "deleted": len(executions)})
		return
	}

	// English note.
	// English note.
	h.logger.Info("", zap.Int("count", len(request.IDs)))
	c.JSON(http.StatusOK, gin.H{"message": "（）"})
}


