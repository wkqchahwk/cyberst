package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/multiagent"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// English note.
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// English note.
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// English note.
	truncated := string(runes[:maxLen])

	// English note.
	// English note.
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune("，。、 ,.;:!?！？/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // 
				goto found
			}
		}
	}

found:
	truncated = string(runes[:bestBreakPos])
	return truncated + "..."
}

// responsePlanAgg buffers main-assistant response_stream chunks for one "planning" process_detail row.
type responsePlanAgg struct {
	meta map[string]interface{}
	b    strings.Builder
}

func normalizeProcessDetailText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// discardPlanningIfEchoesToolResult drops buffered planning text when it only repeats the
// upcoming tool_result body. Streaming models often echo tool stdout in chunk.Content; flushing
// that into "planning" before persisting tool_result duplicates the output after page refresh.
func discardPlanningIfEchoesToolResult(respPlan *responsePlanAgg, toolData interface{}) {
	if respPlan == nil {
		return
	}
	plan := normalizeProcessDetailText(respPlan.b.String())
	if plan == "" {
		return
	}
	dataMap, ok := toolData.(map[string]interface{})
	if !ok {
		return
	}
	res, ok := dataMap["result"].(string)
	if !ok {
		return
	}
	r := normalizeProcessDetailText(res)
	if r == "" {
		return
	}
	if plan == r || strings.HasSuffix(plan, r) {
		respPlan.meta = nil
		respPlan.b.Reset()
	}
}

// English note.
type AgentHandler struct {
	agent            *agent.Agent
	db               *database.DB
	logger           *zap.Logger
	tasks            *AgentTaskManager
	batchTaskManager *BatchTaskManager
	config           *config.Config // ，
	knowledgeManager interface {    // 
		LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
	}
	agentsMarkdownDir string // ：Markdown  Agent （，）
	batchCronParser   cron.Parser
	batchRunnerMu     sync.Mutex
	batchRunning      map[string]struct{}
}

// English note.
func NewAgentHandler(agent *agent.Agent, db *database.DB, cfg *config.Config, logger *zap.Logger) *AgentHandler {
	batchTaskManager := NewBatchTaskManager(logger)
	batchTaskManager.SetDB(db)

	// English note.
	if err := batchTaskManager.LoadFromDB(); err != nil {
		logger.Warn("", zap.Error(err))
	}

	handler := &AgentHandler{
		agent:            agent,
		db:               db,
		logger:           logger,
		tasks:            NewAgentTaskManager(),
		batchTaskManager: batchTaskManager,
		config:           cfg,
		batchCronParser:  cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		batchRunning:     make(map[string]struct{}),
	}
	go handler.batchQueueSchedulerLoop()
	return handler
}

// English note.
func (h *AgentHandler) SetKnowledgeManager(manager interface {
	LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
}) {
	h.knowledgeManager = manager
}

// English note.
func (h *AgentHandler) SetAgentsMarkdownDir(absDir string) {
	h.agentsMarkdownDir = strings.TrimSpace(absDir)
}

// English note.
type ChatAttachment struct {
	FileName   string `json:"fileName"`          // 
	Content    string `json:"content,omitempty"` //  base64；
	MimeType   string `json:"mimeType,omitempty"`
	ServerPath string `json:"serverPath,omitempty"` //  chat_uploads （ POST /api/chat-uploads ）
}

// English note.
type ChatRequest struct {
	Message              string           `json:"message" binding:"required"`
	ConversationID       string           `json:"conversationId,omitempty"`
	Role                 string           `json:"role,omitempty"` // 
	Attachments          []ChatAttachment `json:"attachments,omitempty"`
	WebShellConnectionID string           `json:"webshellConnectionId,omitempty"` // WebShell  - AI ： ID， webshell_* 
	// English note.
	Orchestration string `json:"orchestration,omitempty"`
}

const (
	maxAttachments     = 10
	chatUploadsDirName = "chat_uploads" // （）
)

// English note.
func validateChatAttachmentServerPath(abs string) (string, error) {
	p := strings.TrimSpace(abs)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf(": %w", err)
	}
	root := filepath.Join(cwd, chatUploadsDirName)
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	sep := string(filepath.Separator)
	if pathAbs != rootAbs && !strings.HasPrefix(pathAbs, rootAbs+sep) {
		return "", fmt.Errorf("path outside chat_uploads")
	}
	st, err := os.Stat(pathAbs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("not a regular file")
	}
	return pathAbs, nil
}

// English note.
func avoidChatUploadDestCollision(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameNoExt := strings.TrimSuffix(base, ext)
	suffix := fmt.Sprintf("_%s_%s", time.Now().Format("150405"), shortRand(6))
	var unique string
	if ext != "" {
		unique = nameNoExt + suffix + ext
	} else {
		unique = base + suffix
	}
	return filepath.Join(dir, unique)
}

// English note.
func relocateManualOrNewUploadToConversation(absPath, conversationID string, logger *zap.Logger) (string, error) {
	conv := strings.TrimSpace(conversationID)
	if conv == "" {
		return absPath, nil
	}
	convSan := strings.ReplaceAll(conv, string(filepath.Separator), "_")
	if convSan == "" || convSan == "_manual" || convSan == "_new" {
		return absPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return absPath, err
	}
	rootAbs, err := filepath.Abs(filepath.Join(cwd, chatUploadsDirName))
	if err != nil {
		return absPath, err
	}
	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return absPath, nil
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	var segs []string
	for _, p := range strings.Split(rel, "/") {
		if p != "" && p != "." {
			segs = append(segs, p)
		}
	}
	// English note.
	if len(segs) != 3 {
		return absPath, nil
	}
	datePart, placeFolder, baseName := segs[0], segs[1], segs[2]
	if placeFolder != "_manual" && placeFolder != "_new" {
		return absPath, nil
	}
	targetDir := filepath.Join(rootAbs, datePart, convSan)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf(": %w", err)
	}
	dest := filepath.Join(targetDir, baseName)
	dest = avoidChatUploadDestCollision(dest)
	if err := os.Rename(absPath, dest); err != nil {
		return "", fmt.Errorf(": %w", err)
	}
	out, _ := filepath.Abs(dest)
	if logger != nil {
		logger.Info("",
			zap.String("from", absPath),
			zap.String("to", out),
			zap.String("conversationId", conv))
	}
	return out, nil
}

// English note.
// English note.
func saveAttachmentsToDateAndConversationDir(attachments []ChatAttachment, conversationID string, logger *zap.Logger) (savedPaths []string, err error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	dateDir := filepath.Join(cwd, chatUploadsDirName, time.Now().Format("2006-01-02"))
	convDirName := strings.TrimSpace(conversationID)
	if convDirName == "" {
		convDirName = "_new"
	} else {
		convDirName = strings.ReplaceAll(convDirName, string(filepath.Separator), "_")
	}
	targetDir := filepath.Join(dateDir, convDirName)
	if err = os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	savedPaths = make([]string, 0, len(attachments))
	for i, a := range attachments {
		if sp := strings.TrimSpace(a.ServerPath); sp != "" {
			valid, verr := validateChatAttachmentServerPath(sp)
			if verr != nil {
				return nil, fmt.Errorf(" %s: %w", a.FileName, verr)
			}
			finalPath, rerr := relocateManualOrNewUploadToConversation(valid, conversationID, logger)
			if rerr != nil {
				return nil, fmt.Errorf(" %s: %w", a.FileName, rerr)
			}
			savedPaths = append(savedPaths, finalPath)
			if logger != nil {
				logger.Debug("", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", finalPath))
			}
			continue
		}
		if strings.TrimSpace(a.Content) == "" {
			return nil, fmt.Errorf(" %s  serverPath", a.FileName)
		}
		raw, decErr := attachmentContentToBytes(a)
		if decErr != nil {
			return nil, fmt.Errorf(" %s : %w", a.FileName, decErr)
		}
		baseName := filepath.Base(a.FileName)
		if baseName == "" || baseName == "." {
			baseName = "file"
		}
		baseName = strings.ReplaceAll(baseName, string(filepath.Separator), "_")
		ext := filepath.Ext(baseName)
		nameNoExt := strings.TrimSuffix(baseName, ext)
		suffix := fmt.Sprintf("_%s_%s", time.Now().Format("150405"), shortRand(6))
		var unique string
		if ext != "" {
			unique = nameNoExt + suffix + ext
		} else {
			unique = baseName + suffix
		}
		fullPath := filepath.Join(targetDir, unique)
		if err = os.WriteFile(fullPath, raw, 0644); err != nil {
			return nil, fmt.Errorf(" %s : %w", a.FileName, err)
		}
		absPath, _ := filepath.Abs(fullPath)
		savedPaths = append(savedPaths, absPath)
		if logger != nil {
			logger.Debug("", zap.Int("index", i+1), zap.String("fileName", a.FileName), zap.String("path", absPath))
		}
	}
	return savedPaths, nil
}

func shortRand(n int) string {
	const letters = "0123456789abcdef"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func attachmentContentToBytes(a ChatAttachment) ([]byte, error) {
	content := a.Content
	if decoded, err := base64.StdEncoding.DecodeString(content); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	return []byte(content), nil
}

// English note.
func userMessageContentForStorage(message string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return message
	}
	var b strings.Builder
	b.WriteString(message)
	for i, a := range attachments {
		b.WriteString("\n📎 ")
		b.WriteString(a.FileName)
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(": ")
			b.WriteString(savedPaths[i])
		}
	}
	return b.String()
}

// English note.
func appendAttachmentsToMessage(msg string, attachments []ChatAttachment, savedPaths []string) string {
	if len(attachments) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\n[（，）]\n")
	for i, a := range attachments {
		if i < len(savedPaths) && savedPaths[i] != "" {
			b.WriteString(fmt.Sprintf("- %s: %s\n", a.FileName, savedPaths[i]))
		} else {
			b.WriteString(fmt.Sprintf("- %s: （，）\n", a.FileName))
		}
	}
	return b.String()
}

// English note.
type ChatResponse struct {
	Response        string    `json:"response"`
	MCPExecutionIDs []string  `json:"mcpExecutionIds,omitempty"` // MCPID
	ConversationID  string    `json:"conversationId"`            // ID
	Time            time.Time `json:"time"`
}

// English note.
func (h *AgentHandler) AgentLoop(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Agent Loop",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// English note.
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	} else {
		// English note.
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("", zap.String("conversationId", conversationID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": ""})
			return
		}
	}

	// English note.
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("ReAct，", zap.Error(err))
		// English note.
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// English note.
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("ReAct", zap.Int("count", len(agentHistoryMessages)))
	}

	// English note.
	if len(req.Attachments) > maxAttachments {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf(" %d ", maxAttachments)})
		return
	}

	// English note.
	finalMessage := req.Message
	var roleTools []string  // 
	var roleSkills []string // skills（AI，）

	// English note.
	if req.WebShellConnectionID != "" {
		conn, err := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if err != nil || conn == nil {
			h.logger.Warn("WebShell AI ：", zap.String("id", req.WebShellConnectionID), zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": " WebShell "})
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		webshellContext := fmt.Sprintf("[WebShell ]  ID：%s，：%s。（，connection_id  \"%s\"）：webshell_exec、webshell_file_list、webshell_file_read、webshell_file_write、record_vulnerability、list_knowledge_risk_types、search_knowledge_base。Skills 「 / Eino DeepAgent」 `skill` 。\n\n：%s",
			conn.ID, remark, conn.ID, req.Message)
		// English note.
		if req.Role != "" && req.Role != "" && h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled && role.UserPrompt != "" {
				finalMessage = role.UserPrompt + "\n\n" + webshellContext
				h.logger.Info("WebShell + : ", zap.String("role", req.Role))
			} else {
				finalMessage = webshellContext
			}
		} else {
			finalMessage = webshellContext
		}
		roleTools = []string{
			builtin.ToolWebshellExec,
			builtin.ToolWebshellFileList,
			builtin.ToolWebshellFileRead,
			builtin.ToolWebshellFileWrite,
			builtin.ToolRecordVulnerability,
			builtin.ToolListKnowledgeRiskTypes,
			builtin.ToolSearchKnowledgeBase,
		}
		roleSkills = nil
	} else if req.Role != "" && req.Role != "" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// English note.
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("", zap.String("role", req.Role))
				}
				// English note.
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				}
				// English note.
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("skills，AI", zap.String("role", req.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
			return
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	// English note.
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	_, err = h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	// English note.
	// English note.
	result, err := h.agent.AgentLoopWithProgress(c.Request.Context(), finalMessage, agentHistoryMessages, conversationID, nil, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop", zap.Error(err))

		// English note.
		if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
			if saveErr := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); saveErr != nil {
				h.logger.Warn("ReAct", zap.Error(saveErr))
			} else {
				h.logger.Info("ReAct", zap.String("conversationId", conversationID))
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		// English note.
		// English note.
	}

	// English note.
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("ReAct", zap.Error(err))
		} else {
			h.logger.Info("ReAct", zap.String("conversationId", conversationID))
		}
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  conversationID,
		Time:            time.Now(),
	})
}

// English note.
func (h *AgentHandler) ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error) {
	if conversationID == "" {
		title := safeTruncateString(message, 50)
		conv, createErr := h.db.CreateConversation(title)
		if createErr != nil {
			return "", "", fmt.Errorf(": %w", createErr)
		}
		conversationID = conv.ID
	} else {
		if _, getErr := h.db.GetConversation(conversationID); getErr != nil {
			return "", "", fmt.Errorf("")
		}
	}

	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		historyMessages, getErr := h.db.GetMessages(conversationID)
		if getErr != nil {
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{Role: msg.Role, Content: msg.Content})
			}
		}
	}

	finalMessage := message
	var roleTools, roleSkills []string
	if role != "" && role != "" && h.config.Roles != nil {
		if r, exists := h.config.Roles[role]; exists && r.Enabled {
			if r.UserPrompt != "" {
				finalMessage = r.UserPrompt + "\n\n" + message
			}
			roleTools = r.Tools
			roleSkills = r.Skills
		}
	}

	if _, err = h.db.AddMessage(conversationID, "user", message, nil); err != nil {
		return "", "", fmt.Errorf(": %w", err)
	}

	// English note.
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "...", nil)
	if err != nil {
		h.logger.Warn("：", zap.Error(err))
	}
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

	useRobotMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
	if useRobotMulti {
		resultMA, errMA := multiagent.RunDeepAgent(
			ctx,
			h.config,
			&h.config.MultiAgent,
			h.agent,
			h.logger,
			conversationID,
			finalMessage,
			agentHistoryMessages,
			roleTools,
			progressCallback,
			h.agentsMarkdownDir,
			"deep",
		)
		if errMA != nil {
			errMsg := ": " + errMA.Error()
			if assistantMessageID != "" {
				_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
				_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
			}
			return "", conversationID, errMA
		}
		if assistantMessageID != "" {
			mcpIDsJSON := ""
			if len(resultMA.MCPExecutionIDs) > 0 {
				jsonData, _ := json.Marshal(resultMA.MCPExecutionIDs)
				mcpIDsJSON = string(jsonData)
			}
			_, err = h.db.Exec(
				"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
				resultMA.Response, mcpIDsJSON, assistantMessageID,
			)
			if err != nil {
				h.logger.Warn("：", zap.Error(err))
			}
		} else {
			if _, err = h.db.AddMessage(conversationID, "assistant", resultMA.Response, resultMA.MCPExecutionIDs); err != nil {
				h.logger.Warn("：", zap.Error(err))
			}
		}
		if resultMA.LastReActInput != "" || resultMA.LastReActOutput != "" {
			_ = h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput)
		}
		return resultMA.Response, conversationID, nil
	}

	result, err := h.agent.AgentLoopWithProgress(ctx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		errMsg := ": " + err.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
			_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
		}
		return "", conversationID, err
	}

	// English note.
	if assistantMessageID != "" {
		mcpIDsJSON := ""
		if len(result.MCPExecutionIDs) > 0 {
			jsonData, _ := json.Marshal(result.MCPExecutionIDs)
			mcpIDsJSON = string(jsonData)
		}
		_, err = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response, mcpIDsJSON, assistantMessageID,
		)
		if err != nil {
			h.logger.Warn("：", zap.Error(err))
		}
	} else {
		if _, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs); err != nil {
			h.logger.Warn("：", zap.Error(err))
		}
	}
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		_ = h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput)
	}
	return result.Response, conversationID, nil
}

// English note.
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // 
	Data    interface{} `json:"data,omitempty"`
}

// English note.
// English note.
func (h *AgentHandler) createProgressCallback(conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	// English note.
	toolCallCache := make(map[string]map[string]interface{}) // toolCallId -> arguments

	// English note.
	type thinkingBuf struct {
		b    strings.Builder
		meta map[string]interface{}
	}
	thinkingStreams := make(map[string]*thinkingBuf) // streamId -> buf
	flushedThinking := make(map[string]bool)         // streamId -> flushed

	// English note.
	// English note.
	var respPlan responsePlanAgg
	flushResponsePlan := func() {
		if assistantMessageID == "" {
			return
		}
		content := strings.TrimSpace(respPlan.b.String())
		if content == "" {
			respPlan.meta = nil
			respPlan.b.Reset()
			return
		}
		data := map[string]interface{}{
			"source": "response_stream",
		}
		for k, v := range respPlan.meta {
			data[k] = v
		}
		if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "planning", content, data); err != nil {
			h.logger.Warn("", zap.Error(err), zap.String("eventType", "planning"))
		}
		respPlan.meta = nil
		respPlan.b.Reset()
	}

	flushThinkingStreams := func() {
		if assistantMessageID == "" {
			return
		}
		for sid, tb := range thinkingStreams {
			if sid == "" || flushedThinking[sid] || tb == nil {
				continue
			}
			content := strings.TrimSpace(tb.b.String())
			if content == "" {
				flushedThinking[sid] = true
				continue
			}
			data := map[string]interface{}{
				"streamId": sid,
			}
			for k, v := range tb.meta {
				// English note.
				if k == "streamId" {
					continue
				}
				data[k] = v
			}
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "thinking", content, data); err != nil {
				h.logger.Warn("", zap.Error(err), zap.String("eventType", "thinking"))
			}
			flushedThinking[sid] = true
		}
	}

	return func(eventType, message string, data interface{}) {
		// English note.
		if sendEventFunc != nil {
			sendEventFunc(eventType, message, data)
		}

		// English note.
		if eventType == "tool_call" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if argumentsObj, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							toolCallCache[toolCallId] = argumentsObj
						}
					}
				}
			}
		}

		// English note.
		if eventType == "tool_result" && h.knowledgeManager != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == builtin.ToolSearchKnowledgeBase {
					// English note.
					query := ""
					riskType := ""
					var retrievedItems []string

					// English note.
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if cachedArgs, exists := toolCallCache[toolCallId]; exists {
							if q, ok := cachedArgs["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := cachedArgs["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
							// English note.
							delete(toolCallCache, toolCallId)
						}
					}

					// English note.
					if query == "" {
						if arguments, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							if q, ok := arguments["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := arguments["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
						}
					}

					// English note.
					if query == "" {
						if result, ok := dataMap["result"].(string); ok && result != "" {
							// English note.
							if strings.Contains(result, " '") {
								start := strings.Index(result, " '") + len(" '")
								end := strings.Index(result[start:], "'")
								if end > 0 {
									query = result[start : start+end]
								}
							}
						}
						// English note.
						if query == "" {
							query = ""
						}
					}

					// English note.
					// English note.
					if result, ok := dataMap["result"].(string); ok && result != "" {
						// English note.
						metadataMatch := strings.Index(result, "<!-- METADATA:")
						if metadataMatch > 0 {
							// English note.
							metadataStart := metadataMatch + len("<!-- METADATA: ")
							metadataEnd := strings.Index(result[metadataStart:], " -->")
							if metadataEnd > 0 {
								metadataJSON := result[metadataStart : metadataStart+metadataEnd]
								var metadata map[string]interface{}
								if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
									if meta, ok := metadata["_metadata"].(map[string]interface{}); ok {
										if ids, ok := meta["retrievedItemIDs"].([]interface{}); ok {
											retrievedItems = make([]string, 0, len(ids))
											for _, id := range ids {
												if idStr, ok := id.(string); ok {
													retrievedItems = append(retrievedItems, idStr)
												}
											}
										}
									}
								}
							}
						}

						// English note.
						if len(retrievedItems) == 0 && strings.Contains(result, "") && !strings.Contains(result, "") {
							// English note.
							retrievedItems = []string{"_has_results"}
						}
					}

					// English note.
					go func() {
						if err := h.knowledgeManager.LogRetrieval(conversationID, assistantMessageID, query, riskType, retrievedItems); err != nil {
							h.logger.Warn("", zap.Error(err))
						}
					}()

					// English note.
					if assistantMessageID != "" {
						retrievalData := map[string]interface{}{
							"query":    query,
							"riskType": riskType,
							"toolName": toolName,
						}
						if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "knowledge_retrieval", fmt.Sprintf(": %s", query), retrievalData); err != nil {
							h.logger.Warn("", zap.Error(err))
						}
					}
				}
			}
		}

		// English note.
		if assistantMessageID != "" && eventType == "eino_agent_reply_stream_end" {
			flushResponsePlan()
			// English note.
			flushThinkingStreams()
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "eino_agent_reply", message, data); err != nil {
				h.logger.Warn("", zap.Error(err), zap.String("eventType", eventType))
			}
			return
		}

		// English note.
		if eventType == "response_start" {
			flushResponsePlan()
			respPlan.meta = nil
			if dataMap, ok := data.(map[string]interface{}); ok {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			respPlan.b.Reset()
			return
		}
		if eventType == "response_delta" {
			respPlan.b.WriteString(message)
			if dataMap, ok := data.(map[string]interface{}); ok && respPlan.meta == nil {
				respPlan.meta = make(map[string]interface{}, len(dataMap))
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			} else if dataMap, ok := data.(map[string]interface{}); ok {
				for k, v := range dataMap {
					respPlan.meta[k] = v
				}
			}
			return
		}
		if eventType == "response" {
			flushResponsePlan()
			return
		}

		// English note.
		if eventType == "thinking_stream_start" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}}
						thinkingStreams[sid] = tb
					}
					// English note.
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}
		if eventType == "thinking_stream_delta" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					tb := thinkingStreams[sid]
					if tb == nil {
						tb = &thinkingBuf{meta: map[string]interface{}{}}
						thinkingStreams[sid] = tb
					}
					// English note.
					tb.b.WriteString(message)
					// English note.
					for k, v := range dataMap {
						tb.meta[k] = v
					}
				}
			}
			return
		}

		// English note.
		// English note.
		// English note.
		if eventType == "thinking" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if sid, ok2 := dataMap["streamId"].(string); ok2 && sid != "" {
					if tb, exists := thinkingStreams[sid]; exists && tb != nil {
						if strings.TrimSpace(tb.b.String()) != "" {
							return
						}
					}
					if flushedThinking[sid] {
						return
					}
				}
			}
		}

		// English note.
		// English note.
		if assistantMessageID != "" &&
			eventType != "response" &&
			eventType != "done" &&
			eventType != "response_start" &&
			eventType != "response_delta" &&
			eventType != "tool_result_delta" &&
			eventType != "eino_agent_reply_stream_start" &&
			eventType != "eino_agent_reply_stream_delta" &&
			eventType != "eino_agent_reply_stream_end" {
			if eventType == "tool_result" {
				discardPlanningIfEchoesToolResult(&respPlan, data)
			}
			// English note.
			flushResponsePlan()
			flushThinkingStreams()
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}
}

// English note.
func (h *AgentHandler) AgentLoopStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// English note.
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		event := StreamEvent{
			Type:    "error",
			Message: ": " + err.Error(),
		}
		eventJSON, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		c.Writer.Flush()
		return
	}

	h.logger.Info("Agent Loop",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// English note.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // nginx

	// English note.
	// English note.
	clientDisconnected := false
	// English note.
	var sseWriteMu sync.Mutex
	// English note.
	var responseDeltaCount int
	var responseStartLogged bool

	sendEvent := func(eventType, message string, data interface{}) {
		if eventType == "response_start" {
			responseDeltaCount = 0
			responseStartLogged = true
			h.logger.Info("SSE: response_start",
				zap.Int("conversationIdPresent", func() int {
					if m, ok := data.(map[string]interface{}); ok {
						if v, ok2 := m["conversationId"]; ok2 && v != nil && fmt.Sprint(v) != "" {
							return 1
						}
					}
					return 0
				}()),
				zap.String("messageGeneratedBy", func() string {
					if m, ok := data.(map[string]interface{}); ok {
						if v, ok2 := m["messageGeneratedBy"]; ok2 {
							if s, ok3 := v.(string); ok3 {
								return s
							}
							return fmt.Sprint(v)
						}
					}
					return ""
				}()),
			)
		} else if eventType == "response_delta" {
			responseDeltaCount++
			// English note.
			if responseStartLogged && responseDeltaCount <= 3 {
				h.logger.Info("SSE: response_delta",
					zap.Int("index", responseDeltaCount),
					zap.Int("deltaLen", len(message)),
					zap.String("deltaPreview", func() string {
						p := strings.ReplaceAll(message, "\n", "\\n")
						if len(p) > 80 {
							return p[:80] + "..."
						}
						return p
					}()),
				)
			}
		}

		// English note.
		if clientDisconnected {
			return
		}

		// English note.
		select {
		case <-c.Request.Context().Done():
			clientDisconnected = true
			return
		default:
		}

		event := StreamEvent{
			Type:    eventType,
			Message: message,
			Data:    data,
		}
		eventJSON, _ := json.Marshal(event)

		sseWriteMu.Lock()
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		if err != nil {
			sseWriteMu.Unlock()
			clientDisconnected = true
			h.logger.Debug("，SSE", zap.Error(err))
			return
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
		sseWriteMu.Unlock()
	}

	// English note.
	conversationID := req.ConversationID
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		var conv *database.Conversation
		var err error
		if req.WebShellConnectionID != "" {
			conv, err = h.db.CreateConversationWithWebshell(strings.TrimSpace(req.WebShellConnectionID), title)
		} else {
			conv, err = h.db.CreateConversation(title)
		}
		if err != nil {
			h.logger.Error("", zap.Error(err))
			sendEvent("error", ": "+err.Error(), nil)
			return
		}
		conversationID = conv.ID
		sendEvent("conversation", "", map[string]interface{}{
			"conversationId": conversationID,
		})
	} else {
		// English note.
		_, err := h.db.GetConversation(conversationID)
		if err != nil {
			h.logger.Error("", zap.String("conversationId", conversationID), zap.Error(err))
			sendEvent("error", "", nil)
			return
		}
	}

	// English note.
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("ReAct，", zap.Error(err))
		// English note.
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// English note.
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("ReAct", zap.Int("count", len(agentHistoryMessages)))
	}

	// English note.
	if len(req.Attachments) > maxAttachments {
		sendEvent("error", fmt.Sprintf(" %d ", maxAttachments), nil)
		return
	}

	// English note.
	finalMessage := req.Message
	var roleTools []string // 
	var roleSkills []string
	if req.WebShellConnectionID != "" {
		conn, errConn := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if errConn != nil || conn == nil {
			h.logger.Warn("WebShell AI ：", zap.String("id", req.WebShellConnectionID), zap.Error(errConn))
			sendEvent("error", " WebShell ", nil)
			return
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		webshellContext := fmt.Sprintf("[WebShell ]  ID：%s，：%s。（，connection_id  \"%s\"）：webshell_exec、webshell_file_list、webshell_file_read、webshell_file_write、record_vulnerability、list_knowledge_risk_types、search_knowledge_base。Skills 「 / Eino DeepAgent」 `skill` 。\n\n：%s",
			conn.ID, remark, conn.ID, req.Message)
		// English note.
		if req.Role != "" && req.Role != "" && h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled && role.UserPrompt != "" {
				finalMessage = role.UserPrompt + "\n\n" + webshellContext
				h.logger.Info("WebShell + : （）", zap.String("role", req.Role))
			} else {
				finalMessage = webshellContext
			}
		} else {
			finalMessage = webshellContext
		}
		roleTools = []string{
			builtin.ToolWebshellExec,
			builtin.ToolWebshellFileList,
			builtin.ToolWebshellFileRead,
			builtin.ToolWebshellFileWrite,
			builtin.ToolRecordVulnerability,
			builtin.ToolListKnowledgeRiskTypes,
			builtin.ToolSearchKnowledgeBase,
		}
	} else if req.Role != "" && req.Role != "" {
		if h.config.Roles != nil {
			if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
				// English note.
				if role.UserPrompt != "" {
					finalMessage = role.UserPrompt + "\n\n" + req.Message
					h.logger.Info("", zap.String("role", req.Role))
				}
				// English note.
				if len(role.Tools) > 0 {
					roleTools = role.Tools
					h.logger.Info("", zap.String("role", req.Role), zap.Int("toolCount", len(roleTools)))
				} else if len(role.MCPs) > 0 {
					// English note.
					// English note.
					h.logger.Info("mcps，", zap.String("role", req.Role))
				}
				// English note.
				if len(role.Skills) > 0 {
					roleSkills = role.Skills
					h.logger.Info("skills，AI", zap.String("role", req.Role), zap.Int("skillCount", len(role.Skills)), zap.Strings("skills", role.Skills))
				}
			}
		}
	}
	var savedPaths []string
	if len(req.Attachments) > 0 {
		savedPaths, err = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if err != nil {
			h.logger.Error("", zap.Error(err))
			sendEvent("error", ": "+err.Error(), nil)
			return
		}
	}
	// English note.
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)
	// English note.

	// English note.
	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	userMsgRow, err := h.db.AddMessage(conversationID, "user", userContent, nil)
	if err != nil {
		h.logger.Error("", zap.Error(err))
	}

	// English note.
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "...", nil)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		// English note.
		assistantMsg = nil
	}

	// English note.
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	// English note.
	if userMsgRow != nil {
		sendEvent("message_saved", "", map[string]interface{}{
			"conversationId": conversationID,
			"userMessageId":  userMsgRow.ID,
		})
	}

	// English note.
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	// English note.
	// English note.
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ ，「」。"
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ : " + err.Error()
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_start_failed",
			})
		}

		// English note.
		if assistantMessageID != "" {
			if _, updateErr := h.db.Exec(
				"UPDATE messages SET content = ? WHERE id = ?",
				errorMsg,
				assistantMessageID,
			); updateErr != nil {
				h.logger.Warn("", zap.Error(updateErr))
			}
			// English note.
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, map[string]interface{}{
				"errorType": func() string {
					if errors.Is(err, ErrTaskAlreadyRunning) {
						return "task_already_running"
					}
					return "task_start_failed"
				}(),
			}); err != nil {
				h.logger.Warn("", zap.Error(err))
			}
		}

		sendEvent("done", "", map[string]interface{}{
			"conversationId": conversationID,
		})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	// English note.
	sendEvent("progress", "...", nil)
	// English note.
	stopKeepalive := make(chan struct{})
	go sseKeepalive(c, stopKeepalive, &sseWriteMu)
	defer close(stopKeepalive)

	result, err := h.agent.AgentLoopWithProgress(taskCtx, finalMessage, agentHistoryMessages, conversationID, progressCallback, roleTools, roleSkills)
	if err != nil {
		h.logger.Error("Agent Loop", zap.Error(err))
		cause := context.Cause(baseCtx)

		// English note.
		// English note.
		// English note.
		isCancelled := errors.Is(cause, ErrTaskCancelled)

		switch {
		case isCancelled:
			taskStatus = "cancelled"
			cancelMsg := "，。"

			// English note.
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					cancelMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}

			// English note.
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("ReAct", zap.Error(err))
				} else {
					h.logger.Info("ReAct", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("cancelled", cancelMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded):
			taskStatus = "timeout"
			timeoutMsg := "，。"

			// English note.
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					timeoutMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}

			// English note.
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("ReAct", zap.Error(err))
				} else {
					h.logger.Info("ReAct", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("error", timeoutMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		default:
			taskStatus = "failed"
			errorMsg := ": " + err.Error()

			// English note.
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					errorMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil)
			}

			// English note.
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("ReAct", zap.Error(err))
				} else {
					h.logger.Info("ReAct", zap.String("conversationId", conversationID))
				}
			}

			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
		}
		return
	}

	// English note.
	if assistantMsg != nil {
		_, err = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response,
			func() string {
				if len(result.MCPExecutionIDs) > 0 {
					jsonData, _ := json.Marshal(result.MCPExecutionIDs)
					return string(jsonData)
				}
				return ""
			}(),
			assistantMessageID,
		)
		if err != nil {
			h.logger.Error("", zap.Error(err))
		}
	} else {
		// English note.
		_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
		if err != nil {
			h.logger.Error("", zap.Error(err))
		}
	}

	// English note.
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("ReAct", zap.Error(err))
		} else {
			h.logger.Info("ReAct", zap.String("conversationId", conversationID))
		}
	}

	// English note.
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID, // ID，
	})
	sendEvent("done", "", map[string]interface{}{
		"conversationId": conversationID,
	})
}

// English note.
func (h *AgentHandler) CancelAgentLoop(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.tasks.CancelTask(req.ConversationID, ErrTaskCancelled)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "cancelling",
		"conversationId": req.ConversationID,
		"message":        "，。",
	})
}

// English note.
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetActiveTasks(),
	})
}

// English note.
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetCompletedTasks(),
	})
}

// English note.
type BatchTaskRequest struct {
	Title        string   `json:"title"`                    // （）
	Tasks        []string `json:"tasks" binding:"required"` // ，
	Role         string   `json:"role,omitempty"`           // （，）
	AgentMode    string   `json:"agentMode,omitempty"`      // single | eino_single | deep | plan_execute | supervisor（react  single； multi  deep）
	ScheduleMode string   `json:"scheduleMode,omitempty"`   // manual | cron
	CronExpr     string   `json:"cronExpr,omitempty"`       // scheduleMode=cron 
	ExecuteNow   bool     `json:"executeNow,omitempty"`     // （ false）
}

func normalizeBatchQueueAgentMode(mode string) string {
	m := strings.TrimSpace(strings.ToLower(mode))
	if m == "multi" {
		return "deep"
	}
	if m == "" || m == "single" || m == "react" {
		return "single"
	}
	if m == "eino_single" {
		return "eino_single"
	}
	switch config.NormalizeMultiAgentOrchestration(m) {
	case "plan_execute":
		return "plan_execute"
	case "supervisor":
		return "supervisor"
	default:
		return "deep"
	}
}

// English note.
func batchQueueWantsEino(agentMode string) bool {
	m := strings.TrimSpace(strings.ToLower(agentMode))
	return m == "multi" || m == "deep" || m == "plan_execute" || m == "supervisor"
}

func normalizeBatchQueueScheduleMode(mode string) string {
	if strings.TrimSpace(mode) == "cron" {
		return "cron"
	}
	return "manual"
}

// English note.
func (h *AgentHandler) CreateBatchQueue(c *gin.Context) {
	var req BatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	// English note.
	validTasks := make([]string, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		if task != "" {
			validTasks = append(validTasks, task)
		}
	}

	if len(validTasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	agentMode := normalizeBatchQueueAgentMode(req.AgentMode)
	scheduleMode := normalizeBatchQueueScheduleMode(req.ScheduleMode)
	cronExpr := strings.TrimSpace(req.CronExpr)
	var nextRunAt *time.Time
	if scheduleMode == "cron" {
		if cronExpr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": " Cron ，"})
			return
		}
		schedule, err := h.batchCronParser.Parse(cronExpr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": " Cron : " + err.Error()})
			return
		}
		next := schedule.Next(time.Now())
		nextRunAt = &next
	}

	queue, createErr := h.batchTaskManager.CreateBatchQueue(req.Title, req.Role, agentMode, scheduleMode, cronExpr, nextRunAt, validTasks)
	if createErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": createErr.Error()})
		return
	}
	started := false
	if req.ExecuteNow {
		ok, err := h.startBatchQueueExecution(queue.ID, false)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": ""})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "queueId": queue.ID})
			return
		}
		started = true
		if refreshed, exists := h.batchTaskManager.GetBatchQueue(queue.ID); exists {
			queue = refreshed
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"queueId": queue.ID,
		"queue":   queue,
		"started": started,
	})
}

// English note.
func (h *AgentHandler) GetBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// English note.
type ListBatchQueuesResponse struct {
	Queues     []*BatchTaskQueue `json:"queues"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// English note.
func (h *AgentHandler) ListBatchQueues(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	pageStr := c.Query("page")
	status := c.Query("status")
	keyword := c.Query("keyword")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	page := 1

	// English note.
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// English note.
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	// English note.
	const maxOffset = 100000
	if offset > maxOffset {
		offset = maxOffset
	}

	// English note.
	if status == "" {
		status = "all"
	}

	// English note.
	queues, total, err := h.batchTaskManager.ListQueues(limit, offset, status, keyword)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// English note.
	if pageStr == "" {
		page = (offset / limit) + 1
	}

	response := ListBatchQueuesResponse{
		Queues:     queues,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// English note.
func (h *AgentHandler) StartBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	ok, err := h.startBatchQueueExecution(queueID, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "queueId": queueID})
}

// English note.
func (h *AgentHandler) RerunBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	if queue.Status != "completed" && queue.Status != "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}
	if !h.batchTaskManager.ResetQueueForRerun(queueID) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": ""})
		return
	}
	ok, err := h.startBatchQueueExecution(queueID, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "queueId": queueID})
}

// English note.
func (h *AgentHandler) PauseBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.PauseQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
func (h *AgentHandler) UpdateBatchQueueMetadata(c *gin.Context) {
	queueID := c.Param("queueId")
	var req struct {
		Title     string `json:"title"`
		Role      string `json:"role"`
		AgentMode string `json:"agentMode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.batchTaskManager.UpdateQueueMetadata(queueID, req.Title, req.Role, req.AgentMode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.batchTaskManager.GetBatchQueue(queueID)
	c.JSON(http.StatusOK, gin.H{"queue": updated})
}

// English note.
func (h *AgentHandler) UpdateBatchQueueSchedule(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	// English note.
	if queue.Status == "running" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "，"})
		return
	}
	var req struct {
		ScheduleMode string `json:"scheduleMode"`
		CronExpr     string `json:"cronExpr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scheduleMode := normalizeBatchQueueScheduleMode(req.ScheduleMode)
	cronExpr := strings.TrimSpace(req.CronExpr)
	var nextRunAt *time.Time
	if scheduleMode == "cron" {
		if cronExpr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": " Cron ，"})
			return
		}
		schedule, err := h.batchCronParser.Parse(cronExpr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": " Cron : " + err.Error()})
			return
		}
		next := schedule.Next(time.Now())
		nextRunAt = &next
	}
	h.batchTaskManager.UpdateQueueSchedule(queueID, scheduleMode, cronExpr, nextRunAt)
	updated, _ := h.batchTaskManager.GetBatchQueue(queueID)
	c.JSON(http.StatusOK, gin.H{"queue": updated})
}

// English note.
func (h *AgentHandler) SetBatchQueueScheduleEnabled(c *gin.Context) {
	queueID := c.Param("queueId")
	if _, exists := h.batchTaskManager.GetBatchQueue(queueID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	var req struct {
		ScheduleEnabled bool `json:"scheduleEnabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.batchTaskManager.SetScheduleEnabled(queueID, req.ScheduleEnabled) {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	queue, _ := h.batchTaskManager.GetBatchQueue(queueID)
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// English note.
func (h *AgentHandler) DeleteBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.DeleteQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
func (h *AgentHandler) UpdateBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	err := h.batchTaskManager.UpdateTaskMessage(queueID, taskID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// English note.
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "queue": queue})
}

// English note.
func (h *AgentHandler) AddBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	task, err := h.batchTaskManager.AddTaskToQueue(queueID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// English note.
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "task": task, "queue": queue})
}

// English note.
func (h *AgentHandler) DeleteBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	err := h.batchTaskManager.DeleteTask(queueID, taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// English note.
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "", "queue": queue})
}

func (h *AgentHandler) markBatchQueueRunning(queueID string) bool {
	h.batchRunnerMu.Lock()
	defer h.batchRunnerMu.Unlock()
	if _, exists := h.batchRunning[queueID]; exists {
		return false
	}
	h.batchRunning[queueID] = struct{}{}
	return true
}

func (h *AgentHandler) unmarkBatchQueueRunning(queueID string) {
	h.batchRunnerMu.Lock()
	defer h.batchRunnerMu.Unlock()
	delete(h.batchRunning, queueID)
}

func (h *AgentHandler) nextBatchQueueRunAt(cronExpr string, from time.Time) (*time.Time, error) {
	expr := strings.TrimSpace(cronExpr)
	if expr == "" {
		return nil, nil
	}
	schedule, err := h.batchCronParser.Parse(expr)
	if err != nil {
		return nil, err
	}
	next := schedule.Next(from)
	return &next, nil
}

func (h *AgentHandler) startBatchQueueExecution(queueID string, scheduled bool) (bool, error) {
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		return false, nil
	}
	if !h.markBatchQueueRunning(queueID) {
		return true, nil
	}

	if scheduled {
		if queue.ScheduleMode != "cron" {
			h.unmarkBatchQueueRunning(queueID)
			err := fmt.Errorf(" cron ")
			h.batchTaskManager.SetLastScheduleError(queueID, err.Error())
			return true, err
		}
		if queue.Status == "running" || queue.Status == "paused" || queue.Status == "cancelled" {
			h.unmarkBatchQueueRunning(queueID)
			err := fmt.Errorf("")
			h.batchTaskManager.SetLastScheduleError(queueID, err.Error())
			return true, err
		}
		if !h.batchTaskManager.ResetQueueForRerun(queueID) {
			h.unmarkBatchQueueRunning(queueID)
			err := fmt.Errorf("")
			h.batchTaskManager.SetLastScheduleError(queueID, err.Error())
			return true, err
		}
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
	} else if queue.Status != "pending" && queue.Status != "paused" {
		h.unmarkBatchQueueRunning(queueID)
		return true, fmt.Errorf("")
	}

	if queue != nil && batchQueueWantsEino(queue.AgentMode) && (h.config == nil || !h.config.MultiAgent.Enabled) {
		h.unmarkBatchQueueRunning(queueID)
		err := fmt.Errorf(" Eino ，")
		if scheduled {
			h.batchTaskManager.SetLastScheduleError(queueID, err.Error())
		}
		return true, err
	}

	if scheduled {
		h.batchTaskManager.RecordScheduledRunStart(queueID)
	}
	h.batchTaskManager.UpdateQueueStatus(queueID, "running")
	if queue != nil && queue.ScheduleMode == "cron" {
		nextRunAt, err := h.nextBatchQueueRunAt(queue.CronExpr, time.Now())
		if err == nil {
			h.batchTaskManager.UpdateQueueSchedule(queueID, "cron", queue.CronExpr, nextRunAt)
		}
	}

	go h.executeBatchQueue(queueID)
	return true, nil
}

func (h *AgentHandler) batchQueueSchedulerLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		queues := h.batchTaskManager.GetLoadedQueues()
		now := time.Now()
		for _, queue := range queues {
			if queue == nil || queue.ScheduleMode != "cron" || !queue.ScheduleEnabled || queue.Status == "cancelled" || queue.Status == "running" || queue.Status == "paused" {
				continue
			}
			nextRunAt := queue.NextRunAt
			if nextRunAt == nil {
				next, err := h.nextBatchQueueRunAt(queue.CronExpr, now)
				if err != nil {
					h.logger.Warn(" cron ，", zap.String("queueId", queue.ID), zap.String("cronExpr", queue.CronExpr), zap.Error(err))
					continue
				}
				h.batchTaskManager.UpdateQueueSchedule(queue.ID, "cron", queue.CronExpr, next)
				nextRunAt = next
			}
			if nextRunAt != nil && (nextRunAt.Before(now) || nextRunAt.Equal(now)) {
				if _, err := h.startBatchQueueExecution(queue.ID, true); err != nil {
					h.logger.Warn("", zap.String("queueId", queue.ID), zap.Error(err))
				}
			}
		}
	}
}

// English note.
func (h *AgentHandler) executeBatchQueue(queueID string) {
	defer h.unmarkBatchQueueRunning(queueID)
	h.logger.Info("", zap.String("queueId", queueID))

	for {
		// English note.
		queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
		if !exists || queue.Status == "cancelled" || queue.Status == "completed" || queue.Status == "paused" {
			break
		}

		// English note.
		task, hasNext := h.batchTaskManager.GetNextTask(queueID)
		if !hasNext {
			// English note.
			q, ok := h.batchTaskManager.GetBatchQueue(queueID)
			lastRunErr := ""
			if ok {
				for _, t := range q.Tasks {
					if t.Status == "failed" && t.Error != "" {
						lastRunErr = t.Error
					}
				}
			}
			h.batchTaskManager.SetLastRunError(queueID, lastRunErr)
			h.batchTaskManager.UpdateQueueStatus(queueID, "completed")
			h.logger.Info("", zap.String("queueId", queueID))
			break
		}

		// English note.
		h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "running", "", "")

		// English note.
		title := safeTruncateString(task.Message, 50)
		conv, err := h.db.CreateConversation(title)
		var conversationID string
		if err != nil {
			h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
			h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", ": "+err.Error())
			h.batchTaskManager.MoveToNextTask(queueID)
			continue
		}
		conversationID = conv.ID

		// English note.
		h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "running", "", "", conversationID)

		// English note.
		finalMessage := task.Message
		var roleTools []string  // 
		var roleSkills []string // skills（AI，）
		if queue.Role != "" && queue.Role != "" {
			if h.config.Roles != nil {
				if role, exists := h.config.Roles[queue.Role]; exists && role.Enabled {
					// English note.
					if role.UserPrompt != "" {
						finalMessage = role.UserPrompt + "\n\n" + task.Message
						h.logger.Info("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role))
					}
					// English note.
					if len(role.Tools) > 0 {
						roleTools = role.Tools
						h.logger.Info("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("toolCount", len(roleTools)))
					}
					// English note.
					if len(role.Skills) > 0 {
						roleSkills = role.Skills
						h.logger.Info("skills，AI", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("role", queue.Role), zap.Int("skillCount", len(roleSkills)), zap.Strings("skills", roleSkills))
					}
				}
			}
		}

		// English note.
		_, err = h.db.AddMessage(conversationID, "user", task.Message, nil)
		if err != nil {
			h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
		}

		// English note.
		assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "...", nil)
		if err != nil {
			h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
			// English note.
			assistantMsg = nil
		}

		// English note.
		var assistantMessageID string
		if assistantMsg != nil {
			assistantMessageID = assistantMsg.ID
		}
		progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

		// English note.
		h.logger.Info("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("message", task.Message), zap.String("role", queue.Role), zap.String("conversationId", conversationID))

		// English note.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		// English note.
		h.batchTaskManager.SetTaskCancel(queueID, cancel)
		// English note.
		// English note.
		useBatchMulti := false
		useEinoSingle := false
		batchOrch := "deep"
		am := strings.TrimSpace(strings.ToLower(queue.AgentMode))
		if am == "multi" {
			am = "deep"
		}
		if am == "eino_single" {
			useEinoSingle = true
		} else if batchQueueWantsEino(queue.AgentMode) && h.config != nil && h.config.MultiAgent.Enabled {
			useBatchMulti = true
			batchOrch = config.NormalizeMultiAgentOrchestration(am)
		} else if queue.AgentMode == "" {
			// English note.
			if h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.BatchUseMultiAgent {
				useBatchMulti = true
				batchOrch = "deep"
			}
		}
		useRunResult := useBatchMulti || useEinoSingle
		var result *agent.AgentLoopResult
		var resultMA *multiagent.RunResult
		var runErr error
		switch {
		case useBatchMulti:
			resultMA, runErr = multiagent.RunDeepAgent(ctx, h.config, &h.config.MultiAgent, h.agent, h.logger, conversationID, finalMessage, []agent.ChatMessage{}, roleTools, progressCallback, h.agentsMarkdownDir, batchOrch)
		case useEinoSingle:
			if h.config == nil {
				runErr = fmt.Errorf("")
			} else {
				resultMA, runErr = multiagent.RunEinoSingleChatModelAgent(ctx, h.config, &h.config.MultiAgent, h.agent, h.logger, conversationID, finalMessage, []agent.ChatMessage{}, roleTools, roleSkills, progressCallback)
			}
		default:
			result, runErr = h.agent.AgentLoopWithProgress(ctx, finalMessage, []agent.ChatMessage{}, conversationID, progressCallback, roleTools, roleSkills)
		}
		// English note.
		h.batchTaskManager.SetTaskCancel(queueID, nil)
		cancel()

		if runErr != nil {
			// English note.
			// English note.
			// English note.
			// English note.
			errStr := runErr.Error()
			partialResp := ""
			if useRunResult && resultMA != nil {
				partialResp = resultMA.Response
			} else if result != nil {
				partialResp = result.Response
			}
			isCancelled := errors.Is(runErr, context.Canceled) ||
				strings.Contains(strings.ToLower(errStr), "context canceled") ||
				strings.Contains(strings.ToLower(errStr), "context cancelled") ||
				(partialResp != "" && (strings.Contains(partialResp, "") || strings.Contains(partialResp, "")))

			if isCancelled {
				h.logger.Info("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				cancelMsg := "，。"
				// English note.
				if partialResp != "" && (strings.Contains(partialResp, "") || strings.Contains(partialResp, "")) {
					cancelMsg = partialResp
				}
				// English note.
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						cancelMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// English note.
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil); err != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else {
					// English note.
					_, errMsg := h.db.AddMessage(conversationID, "assistant", cancelMsg, nil)
					if errMsg != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(errMsg))
					}
				}
				// English note.
				if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
						h.logger.Warn("ReAct", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else if useRunResult && resultMA != nil && (resultMA.LastReActInput != "" || resultMA.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, resultMA.LastReActInput, resultMA.LastReActOutput); err != nil {
						h.logger.Warn("ReAct", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "cancelled", cancelMsg, "", conversationID)
			} else {
				h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(runErr))
				errorMsg := ": " + runErr.Error()
				// English note.
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						errorMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// English note.
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil); err != nil {
						h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", runErr.Error())
			}
		} else {
			h.logger.Info("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))

			var resText string
			var mcpIDs []string
			var lastIn, lastOut string
			if useRunResult {
				resText = resultMA.Response
				mcpIDs = resultMA.MCPExecutionIDs
				lastIn = resultMA.LastReActInput
				lastOut = resultMA.LastReActOutput
			} else {
				resText = result.Response
				mcpIDs = result.MCPExecutionIDs
				lastIn = result.LastReActInput
				lastOut = result.LastReActOutput
			}

			// English note.
			if assistantMessageID != "" {
				mcpIDsJSON := ""
				if len(mcpIDs) > 0 {
					jsonData, _ := json.Marshal(mcpIDs)
					mcpIDsJSON = string(jsonData)
				}
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
					resText,
					mcpIDsJSON,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					// English note.
					_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
					if err != nil {
						h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
					}
				}
			} else {
				// English note.
				_, err = h.db.AddMessage(conversationID, "assistant", resText, mcpIDs)
				if err != nil {
					h.logger.Error("", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				}
			}

			// English note.
			if lastIn != "" || lastOut != "" {
				if err := h.db.SaveReActData(conversationID, lastIn, lastOut); err != nil {
					h.logger.Warn("ReAct", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
				} else {
					h.logger.Info("ReAct", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				}
			}

			// English note.
			h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "completed", resText, "", conversationID)
		}

		// English note.
		h.batchTaskManager.MoveToNextTask(queueID)

		// English note.
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
		if queue.Status == "cancelled" || queue.Status == "paused" {
			break
		}
	}
}

// English note.
// English note.
func (h *AgentHandler) loadHistoryFromReActData(conversationID string) ([]agent.ChatMessage, error) {
	// English note.
	reactInputJSON, reactOutput, err := h.db.GetReActData(conversationID)
	if err != nil {
		return nil, fmt.Errorf("ReAct: %w", err)
	}

	// English note.
	if reactInputJSON == "" {
		return nil, fmt.Errorf("ReAct，")
	}

	dataSource := "database_last_react_input"

	// English note.
	var messagesArray []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messagesArray); err != nil {
		return nil, fmt.Errorf("ReActJSON: %w", err)
	}

	messageCount := len(messagesArray)

	h.logger.Info("ReAct",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("reactInputSize", len(reactInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("reactOutputSize", len(reactOutput)),
	)
	// fmt.Println("messagesArray:", messagesArray)//debug

	// English note.
	agentMessages := make([]agent.ChatMessage, 0, len(messagesArray))
	for _, msgMap := range messagesArray {
		msg := agent.ChatMessage{}

		// English note.
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		} else {
			continue // 
		}

		// English note.
		if msg.Role == "system" {
			continue
		}

		// English note.
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		// English note.
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				msg.ToolCalls = make([]agent.ToolCall, 0, len(toolCallsArray))
				for _, tcRaw := range toolCallsArray {
					if tcMap, ok := tcRaw.(map[string]interface{}); ok {
						toolCall := agent.ToolCall{}

						// English note.
						if id, ok := tcMap["id"].(string); ok {
							toolCall.ID = id
						}

						// English note.
						if toolType, ok := tcMap["type"].(string); ok {
							toolCall.Type = toolType
						}

						// English note.
						if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Function = agent.FunctionCall{}

							// English note.
							if name, ok := funcMap["name"].(string); ok {
								toolCall.Function.Name = name
							}

							// English note.
							if argsRaw, ok := funcMap["arguments"]; ok {
								if argsStr, ok := argsRaw.(string); ok {
									// English note.
									var argsMap map[string]interface{}
									if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
										toolCall.Function.Arguments = argsMap
									}
								} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
									// English note.
									toolCall.Function.Arguments = argsMap
								}
							}
						}

						if toolCall.ID != "" {
							msg.ToolCalls = append(msg.ToolCalls, toolCall)
						}
					}
				}
			}
		}

		// English note.
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		agentMessages = append(agentMessages, msg)
	}

	// English note.
	// English note.
	if reactOutput != "" {
		// English note.
		// English note.
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				// English note.
				lastMsg.Content = reactOutput
			} else {
				// English note.
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: reactOutput,
				})
			}
		} else {
			// English note.
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: reactOutput,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("ReAct")
	}

	// English note.
	// English note.
	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("ReActtool",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("ReAct",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasReactOutput", reactOutput != ""),
	)
	fmt.Println("agentMessages:", agentMessages) //debug
	return agentMessages, nil
}
