package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// English note.
type Agent struct {
	openAIClient          *openai.Client
	config                *config.OpenAIConfig
	agentConfig           *config.AgentConfig
	memoryCompressor      *MemoryCompressor
	mcpServer             *mcp.Server
	externalMCPMgr        *mcp.ExternalMCPManager // MCP
	logger                *zap.Logger
	maxIterations         int
	resultStorage         ResultStorage     // 
	largeResultThreshold  int               // （）
	mu                    sync.RWMutex      // 
	toolNameMapping       map[string]string // ：OpenAI -> （MCP）
	currentConversationID string            // ID（）
	promptBaseDir         string            //  system_prompt_path （ config.yaml ）
}

// English note.
type ResultStorage interface {
	SaveResult(executionID string, toolName string, result string) error
	GetResult(executionID string) (string, error)
	GetResultPage(executionID string, page int, limit int) (*storage.ResultPage, error)
	SearchResult(executionID string, keyword string, useRegex bool) ([]string, error)
	FilterResult(executionID string, filter string, useRegex bool) ([]string, error)
	GetResultMetadata(executionID string) (*storage.ResultMetadata, error)
	GetResultPath(executionID string) string
	DeleteResult(executionID string) error
}

// English note.
func NewAgent(cfg *config.OpenAIConfig, agentCfg *config.AgentConfig, mcpServer *mcp.Server, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger, maxIterations int) *Agent {
	// English note.
	if maxIterations <= 0 {
		maxIterations = 30
	}

	// English note.
	largeResultThreshold := 50 * 1024
	if agentCfg != nil && agentCfg.LargeResultThreshold > 0 {
		largeResultThreshold = agentCfg.LargeResultThreshold
	}

	// English note.
	resultStorageDir := "tmp"
	if agentCfg != nil && agentCfg.ResultStorageDir != "" {
		resultStorageDir = agentCfg.ResultStorageDir
	}

	// English note.
	var resultStorage ResultStorage
	if resultStorageDir != "" {
		// English note.
		// English note.
		// English note.
	}

	// English note.
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   300 * time.Second,
			KeepAlive: 300 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Minute, // ：15，
		DisableKeepAlives:     false,            // 
	}

	// English note.
	// English note.
	httpClient := &http.Client{
		Timeout:   30 * time.Minute, // 530
		Transport: transport,
	}
	llmClient := openai.NewClient(cfg, httpClient, logger)

	var memoryCompressor *MemoryCompressor
	if cfg != nil {
		mc, err := NewMemoryCompressor(MemoryCompressorConfig{
			MaxTotalTokens: cfg.MaxTotalTokens,
			OpenAIConfig:   cfg,
			HTTPClient:     httpClient,
			Logger:         logger,
		})
		if err != nil {
			logger.Warn("MemoryCompressor，", zap.Error(err))
		} else {
			memoryCompressor = mc
		}
	} else {
		logger.Warn("OpenAI，MemoryCompressor")
	}

	return &Agent{
		openAIClient:         llmClient,
		config:               cfg,
		agentConfig:          agentCfg,
		memoryCompressor:     memoryCompressor,
		mcpServer:            mcpServer,
		externalMCPMgr:       externalMCPMgr,
		logger:               logger,
		maxIterations:        maxIterations,
		resultStorage:        resultStorage,
		largeResultThreshold: largeResultThreshold,
		toolNameMapping:      make(map[string]string), // 
	}
}

// English note.
func (a *Agent) SetResultStorage(storage ResultStorage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resultStorage = storage
}

// English note.
func (a *Agent) SetPromptBaseDir(dir string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.promptBaseDir = strings.TrimSpace(dir)
}

// English note.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// English note.
func (cm ChatMessage) MarshalJSON() ([]byte, error) {
	// English note.
	aux := map[string]interface{}{
		"role": cm.Role,
	}

	// English note.
	if cm.Content != "" {
		aux["content"] = cm.Content
	}

	// English note.
	if cm.ToolCallID != "" {
		aux["tool_call_id"] = cm.ToolCallID
	}

	// English note.
	if len(cm.ToolCalls) > 0 {
		toolCallsJSON := make([]map[string]interface{}, len(cm.ToolCalls))
		for i, tc := range cm.ToolCalls {
			// English note.
			argsJSON := ""
			if tc.Function.Arguments != nil {
				argsBytes, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					return nil, err
				}
				argsJSON = string(argsBytes)
			}

			toolCallsJSON[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": argsJSON,
				},
			}
		}
		aux["tool_calls"] = toolCallsJSON
	}

	return json.Marshal(aux)
}

// English note.
type OpenAIRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

// English note.
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Error   *Error   `json:"error,omitempty"`
}

// English note.
type Choice struct {
	Message      MessageWithTools `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// English note.
type MessageWithTools struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// English note.
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// English note.
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// English note.
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// English note.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// English note.
type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// English note.
func (fc *FunctionCall) UnmarshalJSON(data []byte) error {
	type Alias FunctionCall
	aux := &struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
		*Alias
	}{
		Alias: (*Alias)(fc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	fc.Name = aux.Name

	// English note.
	switch v := aux.Arguments.(type) {
	case map[string]interface{}:
		fc.Arguments = v
	case string:
		// English note.
		if err := json.Unmarshal([]byte(v), &fc.Arguments); err != nil {
			// English note.
			fc.Arguments = map[string]interface{}{
				"raw": v,
			}
		}
	case nil:
		fc.Arguments = make(map[string]interface{})
	default:
		// English note.
		fc.Arguments = map[string]interface{}{
			"value": v,
		}
	}

	return nil
}

// English note.
type AgentLoopResult struct {
	Response        string
	MCPExecutionIDs []string
	LastReActInput  string // ReAct（messages，JSON）
	LastReActOutput string // 
}

// English note.
type ProgressCallback func(eventType, message string, data interface{})

// English note.
func (a *Agent) AgentLoop(ctx context.Context, userInput string, historyMessages []ChatMessage) (*AgentLoopResult, error) {
	return a.AgentLoopWithProgress(ctx, userInput, historyMessages, "", nil, nil, nil)
}

// English note.
func (a *Agent) AgentLoopWithConversationID(ctx context.Context, userInput string, historyMessages []ChatMessage, conversationID string) (*AgentLoopResult, error) {
	return a.AgentLoopWithProgress(ctx, userInput, historyMessages, conversationID, nil, nil, nil)
}

// English note.
func (a *Agent) EinoSingleAgentSystemInstruction(roleSkills []string) string {
	systemPrompt := DefaultSingleAgentSystemPrompt()
	if a.agentConfig != nil {
		if p := strings.TrimSpace(a.agentConfig.SystemPromptPath); p != "" {
			path := p
			a.mu.RLock()
			base := a.promptBaseDir
			a.mu.RUnlock()
			if !filepath.IsAbs(path) && base != "" {
				path = filepath.Join(base, path)
			}
			if b, err := os.ReadFile(path); err != nil {
				a.logger.Warn(" system_prompt_path ，", zap.String("path", path), zap.Error(err))
			} else if s := strings.TrimSpace(string(b)); s != "" {
				systemPrompt = s
			}
		}
	}
	if len(roleSkills) > 0 {
		var skillsHint strings.Builder
		skillsHint.WriteString("\n\nSkills：\n")
		for i, skillName := range roleSkills {
			if i > 0 {
				skillsHint.WriteString("、")
			}
			skillsHint.WriteString("`")
			skillsHint.WriteString(skillName)
			skillsHint.WriteString("`")
		}
		skillsHint.WriteString("\n-  skills/  SKILL.md  `name` 。")
		skillsHint.WriteString("\n-  Eino  `skill` ，； MCP 。")
		skillsHint.WriteString("\n-  skill  `")
		skillsHint.WriteString(roleSkills[0])
		skillsHint.WriteString("`")
		systemPrompt += skillsHint.String()
	}
	return systemPrompt
}

// English note.
// English note.
func (a *Agent) AgentLoopWithProgress(ctx context.Context, userInput string, historyMessages []ChatMessage, conversationID string, callback ProgressCallback, roleTools []string, roleSkills []string) (*AgentLoopResult, error) {
	// English note.
	a.mu.Lock()
	a.currentConversationID = conversationID
	a.mu.Unlock()
	// English note.
	sendProgress := func(eventType, message string, data interface{}) {
		if callback != nil {
			callback(eventType, message, data)
		}
	}

	systemPrompt := DefaultSingleAgentSystemPrompt()
	if a.agentConfig != nil {
		if p := strings.TrimSpace(a.agentConfig.SystemPromptPath); p != "" {
			path := p
			a.mu.RLock()
			base := a.promptBaseDir
			a.mu.RUnlock()
			if !filepath.IsAbs(path) && base != "" {
				path = filepath.Join(base, path)
			}
			if b, err := os.ReadFile(path); err != nil {
				a.logger.Warn(" system_prompt_path ，", zap.String("path", path), zap.Error(err))
			} else if s := strings.TrimSpace(string(b)); s != "" {
				systemPrompt = s
			}
		}
	}

	// English note.
	if len(roleSkills) > 0 {
		var skillsHint strings.Builder
		skillsHint.WriteString("\n\nSkills：\n")
		for i, skillName := range roleSkills {
			if i > 0 {
				skillsHint.WriteString("、")
			}
			skillsHint.WriteString("`")
			skillsHint.WriteString(skillName)
			skillsHint.WriteString("`")
		}
		skillsHint.WriteString("\n-  skills/  SKILL.md  `name` ； **Eino **  `skill` ")
		skillsHint.WriteString("\n- ： Eino skill  skill  `")
		skillsHint.WriteString(roleSkills[0])
		skillsHint.WriteString("`")
		skillsHint.WriteString("\n-  MCP  skill ；（DeepAgent）")
		systemPrompt += skillsHint.String()
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// English note.
	a.logger.Info("",
		zap.Int("count", len(historyMessages)),
	)
	addedCount := 0
	for i, msg := range historyMessages {
		// English note.
		// English note.
		if msg.Role == "tool" || msg.Content != "" {
			messages = append(messages, ChatMessage{
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCalls:  msg.ToolCalls,
				ToolCallID: msg.ToolCallID,
			})
			addedCount++
			contentPreview := msg.Content
			if len(contentPreview) > 50 {
				contentPreview = contentPreview[:50] + "..."
			}
			a.logger.Info("",
				zap.Int("index", i),
				zap.String("role", msg.Role),
				zap.String("content", contentPreview),
				zap.Int("toolCalls", len(msg.ToolCalls)),
				zap.String("toolCallID", msg.ToolCallID),
			)
		}
	}

	a.logger.Info("",
		zap.Int("historyMessages", len(historyMessages)),
		zap.Int("addedMessages", addedCount),
		zap.Int("totalMessages", len(messages)),
	)

	// English note.
	// English note.
	if len(messages) > 0 {
		if fixed := a.repairOrphanToolMessages(&messages); fixed {
			a.logger.Info("tool")
		}
	}

	// English note.
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	result := &AgentLoopResult{
		MCPExecutionIDs: make([]string, 0),
	}

	// English note.
	var currentReActInput string

	maxIterations := a.maxIterations
	thinkingStreamSeq := 0
	for i := 0; i < maxIterations; i++ {
		// English note.
		tools := a.getAvailableTools(roleTools)
		toolsTokens := a.countToolsTokens(tools)
		messages = a.applyMemoryCompression(ctx, messages, toolsTokens)

		// English note.
		isLastIteration := (i == maxIterations-1)

		// English note.
		// English note.
		messagesJSON, err := json.Marshal(messages)
		if err != nil {
			a.logger.Warn("ReAct", zap.Error(err))
		} else {
			currentReActInput = string(messagesJSON)
			// English note.
			result.LastReActInput = currentReActInput
		}

		// English note.
		select {
		case <-ctx.Done():
			// English note.
			a.logger.Info("，ReAct", zap.Error(ctx.Err()))
			result.LastReActInput = currentReActInput
			if ctx.Err() == context.Canceled {
				result.Response = "。"
			} else {
				result.Response = fmt.Sprintf(": %v", ctx.Err())
			}
			result.LastReActOutput = result.Response
			return result, ctx.Err()
		default:
		}

		// English note.
		if a.memoryCompressor != nil {
			messagesTokens, systemCount, regularCount := a.memoryCompressor.totalTokensFor(messages)
			totalTokens := messagesTokens + toolsTokens
			a.logger.Info("memory compressor context stats",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
				zap.Int("systemMessages", systemCount),
				zap.Int("regularMessages", regularCount),
				zap.Int("messagesTokens", messagesTokens),
				zap.Int("toolsTokens", toolsTokens),
				zap.Int("totalTokens", totalTokens),
				zap.Int("maxTotalTokens", a.memoryCompressor.maxTotalTokens),
			)
		}

		// English note.
		if i == 0 {
			sendProgress("iteration", "", map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		} else if isLastIteration {
			sendProgress("iteration", fmt.Sprintf(" %d （）", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
				"isLast":    true,
			})
		} else {
			sendProgress("iteration", fmt.Sprintf(" %d ", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		}

		// English note.
		if i == 0 {
			a.logger.Info("OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
			// English note.
			for j, msg := range messages {
				if j >= 5 { // 5
					break
				}
				contentPreview := msg.Content
				if len(contentPreview) > 100 {
					contentPreview = contentPreview[:100] + "..."
				}
				a.logger.Debug("",
					zap.Int("index", j),
					zap.String("role", msg.Role),
					zap.String("content", contentPreview),
				)
			}
		} else {
			a.logger.Info("OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
		}

		// English note.
		sendProgress("progress", "AI...", nil)
		thinkingStreamSeq++
		thinkingStreamId := fmt.Sprintf("thinking-stream-%s-%d-%d", conversationID, i+1, thinkingStreamSeq)
		thinkingStreamStarted := false

		response, err := a.callOpenAIStreamWithToolCalls(ctx, messages, tools, func(delta string) error {
			if delta == "" {
				return nil
			}
			if !thinkingStreamStarted {
				thinkingStreamStarted = true
				sendProgress("thinking_stream_start", " ", map[string]interface{}{
					"streamId":   thinkingStreamId,
					"iteration":  i + 1,
					"toolStream": false,
				})
			}
			sendProgress("thinking_stream_delta", delta, map[string]interface{}{
				"streamId":  thinkingStreamId,
				"iteration": i + 1,
			})
			return nil
		})
		if err != nil {
			// English note.
			result.LastReActInput = currentReActInput
			errorMsg := fmt.Sprintf("OpenAI: %v", err)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			a.logger.Warn("OpenAI，ReAct", zap.Error(err))
			return result, fmt.Errorf("OpenAI: %w", err)
		}

		if response.Error != nil {
			if handled, toolName := a.handleMissingToolError(response.Error.Message, &messages); handled {
				sendProgress("warning", fmt.Sprintf("：%s，。", toolName), map[string]interface{}{
					"toolName": toolName,
				})
				a.logger.Warn("，",
					zap.String("tool", toolName),
					zap.String("error", response.Error.Message),
				)
				continue
			}
			if a.handleToolRoleError(response.Error.Message, &messages) {
				sendProgress("warning", "，。", map[string]interface{}{
					"error": response.Error.Message,
				})
				a.logger.Warn("，",
					zap.String("error", response.Error.Message),
				)
				continue
			}
			// English note.
			result.LastReActInput = currentReActInput
			errorMsg := fmt.Sprintf("OpenAI: %s", response.Error.Message)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("OpenAI: %s", response.Error.Message)
		}

		if len(response.Choices) == 0 {
			// English note.
			result.LastReActInput = currentReActInput
			errorMsg := ""
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("")
		}

		choice := response.Choices[0]

		// English note.
		if len(choice.Message.ToolCalls) > 0 {
			// English note.
			// English note.
			if choice.Message.Content != "" {
				sendProgress("thinking", choice.Message.Content, map[string]interface{}{
					"iteration": i + 1,
					"streamId":  thinkingStreamId,
				})
			}

			// English note.
			messages = append(messages, ChatMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			// English note.
			sendProgress("tool_calls_detected", fmt.Sprintf(" %d ", len(choice.Message.ToolCalls)), map[string]interface{}{
				"count":     len(choice.Message.ToolCalls),
				"iteration": i + 1,
			})

			// English note.
			for idx, toolCall := range choice.Message.ToolCalls {
				// English note.
				toolArgsJSON, _ := json.Marshal(toolCall.Function.Arguments)
				sendProgress("tool_call", fmt.Sprintf(": %s", toolCall.Function.Name), map[string]interface{}{
					"toolName":     toolCall.Function.Name,
					"arguments":    string(toolArgsJSON),
					"argumentsObj": toolCall.Function.Arguments,
					"toolCallId":   toolCall.ID,
					"index":        idx + 1,
					"total":        len(choice.Message.ToolCalls),
					"iteration":    i + 1,
				})

				// English note.
				toolCtx := context.WithValue(ctx, security.ToolOutputCallbackCtxKey, security.ToolOutputCallback(func(chunk string) {
					if strings.TrimSpace(chunk) == "" {
						return
					}
					sendProgress("tool_result_delta", chunk, map[string]interface{}{
						"toolName":    toolCall.Function.Name,
						"toolCallId":  toolCall.ID,
						"index":       idx + 1,
						"total":       len(choice.Message.ToolCalls),
						"iteration":   i + 1,
						// English note.
					})
				}))

				execResult, err := a.executeToolViaMCP(toolCtx, toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					// English note.
					errorMsg := a.formatToolError(toolCall.Function.Name, toolCall.Function.Arguments, err)
					messages = append(messages, ChatMessage{
						Role:       "tool",
						ToolCallID: toolCall.ID,
						Content:    errorMsg,
					})

					// English note.
					sendProgress("tool_result", fmt.Sprintf(" %s ", toolCall.Function.Name), map[string]interface{}{
						"toolName":   toolCall.Function.Name,
						"success":    false,
						"isError":    true,
						"error":      err.Error(),
						"toolCallId": toolCall.ID,
						"index":      idx + 1,
						"total":      len(choice.Message.ToolCalls),
						"iteration":  i + 1,
					})

					a.logger.Warn("，",
						zap.String("tool", toolCall.Function.Name),
						zap.Error(err),
					)
				} else {
					// English note.
					messages = append(messages, ChatMessage{
						Role:       "tool",
						ToolCallID: toolCall.ID,
						Content:    execResult.Result,
					})
					// English note.
					if execResult.ExecutionID != "" {
						result.MCPExecutionIDs = append(result.MCPExecutionIDs, execResult.ExecutionID)
					}

					// English note.
					resultPreview := execResult.Result
					if len(resultPreview) > 200 {
						resultPreview = resultPreview[:200] + "..."
					}
					sendProgress("tool_result", fmt.Sprintf(" %s ", toolCall.Function.Name), map[string]interface{}{
						"toolName":      toolCall.Function.Name,
						"success":       !execResult.IsError,
						"isError":       execResult.IsError,
						"result":        execResult.Result, // 
						"resultPreview": resultPreview,     // 
						"executionId":   execResult.ExecutionID,
						"toolCallId":    toolCall.ID,
						"index":         idx + 1,
						"total":         len(choice.Message.ToolCalls),
						"iteration":     i + 1,
					})

					// English note.
					if execResult.IsError {
						a.logger.Warn("，",
							zap.String("tool", toolCall.Function.Name),
							zap.String("result", execResult.Result),
						)
					}
				}
			}

			// English note.
			if isLastIteration {
				sendProgress("progress", "：...", nil)
				// English note.
				messages = append(messages, ChatMessage{
					Role:    "user",
					Content: "。、。，。，。",
				})
				messages = a.applyMemoryCompression(ctx, messages, 0) //  tools，
				// English note.
				sendProgress("response_start", "", map[string]interface{}{
					"conversationId":     conversationID,
					"mcpExecutionIds":   result.MCPExecutionIDs,
					"messageGeneratedBy": "summary",
				})
				streamText, _ := a.callOpenAIStreamText(ctx, messages, []Tool{}, func(delta string) error {
					sendProgress("response_delta", delta, map[string]interface{}{
						"conversationId": conversationID,
					})
					return nil
				})
				if strings.TrimSpace(streamText) != "" {
					result.Response = streamText
					result.LastReActOutput = result.Response
					sendProgress("progress", "", nil)
					return result, nil
				}
				// English note.
				break
			}

			continue
		}

		// English note.
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		})

		// English note.
		if choice.Message.Content != "" && !thinkingStreamStarted {
			sendProgress("thinking", choice.Message.Content, map[string]interface{}{
				"iteration": i + 1,
			})
		}

		// English note.
		if isLastIteration {
			sendProgress("progress", "：...", nil)
			// English note.
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: "。、。，。，。",
			})
			messages = a.applyMemoryCompression(ctx, messages, 0) //  tools，
			// English note.
			sendProgress("response_start", "", map[string]interface{}{
				"conversationId":     conversationID,
				"mcpExecutionIds":   result.MCPExecutionIDs,
				"messageGeneratedBy": "summary",
			})
			streamText, _ := a.callOpenAIStreamText(ctx, messages, []Tool{}, func(delta string) error {
				sendProgress("response_delta", delta, map[string]interface{}{
					"conversationId": conversationID,
				})
				return nil
			})
			if strings.TrimSpace(streamText) != "" {
				result.Response = streamText
				result.LastReActOutput = result.Response
				sendProgress("progress", "", nil)
				return result, nil
			}
			// English note.
			if choice.Message.Content != "" {
				result.Response = choice.Message.Content
				result.LastReActOutput = result.Response
				return result, nil
			}
			// English note.
			break
		}

		// English note.
		if choice.FinishReason == "stop" {
			sendProgress("progress", "...", nil)
			result.Response = choice.Message.Content
			result.LastReActOutput = result.Response
			return result, nil
		}
	}

	// English note.
	// English note.
	sendProgress("progress", "，...", nil)
	finalSummaryPrompt := ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("（%d）。、。，。，。", a.maxIterations),
	}
	messages = append(messages, finalSummaryPrompt)
	messages = a.applyMemoryCompression(ctx, messages, 0) //  tools，

	// English note.
	sendProgress("response_start", "", map[string]interface{}{
		"conversationId":     conversationID,
		"mcpExecutionIds":   result.MCPExecutionIDs,
		"messageGeneratedBy": "max_iter_summary",
	})
	streamText, _ := a.callOpenAIStreamText(ctx, messages, []Tool{}, func(delta string) error {
		sendProgress("response_delta", delta, map[string]interface{}{
			"conversationId": conversationID,
		})
		return nil
	})
	if strings.TrimSpace(streamText) != "" {
		result.Response = streamText
		result.LastReActOutput = result.Response
		sendProgress("progress", "", nil)
		return result, nil
	}

	// English note.
	result.Response = fmt.Sprintf("（%d）。，，。，。", a.maxIterations)
	result.LastReActOutput = result.Response
	return result, nil
}

// English note.
// English note.
// English note.
func (a *Agent) getAvailableTools(roleTools []string) []Tool {
	// English note.
	roleToolSet := make(map[string]bool)
	if len(roleTools) > 0 {
		for _, toolKey := range roleTools {
			roleToolSet[toolKey] = true
		}
	}

	// English note.
	mcpTools := a.mcpServer.GetAllTools()

	// English note.
	tools := make([]Tool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		// English note.
		if len(roleToolSet) > 0 {
			toolKey := mcpTool.Name // key
			if !roleToolSet[toolKey] {
				continue // ，
			}
		}
		// English note.
		description := mcpTool.ShortDescription
		if description == "" {
			description = mcpTool.Description
		}

		// English note.
		convertedSchema := a.convertSchemaTypes(mcpTool.InputSchema)

		tools = append(tools, Tool{
			Type: "function",
			Function: FunctionDefinition{
				Name:        mcpTool.Name,
				Description: description, // token
				Parameters:  convertedSchema,
			},
		})
	}

	// English note.
	if a.externalMCPMgr != nil {
		// English note.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		externalTools, err := a.externalMCPMgr.GetAllTools(ctx)
		if err != nil {
			a.logger.Warn("MCP", zap.Error(err))
		} else {
			// English note.
			externalMCPConfigs := a.externalMCPMgr.GetConfigs()

			// English note.
			a.mu.Lock()
			a.toolNameMapping = make(map[string]string)
			a.mu.Unlock()

			// English note.
			for _, externalTool := range externalTools {
				// English note.
				externalToolKey := externalTool.Name

				// English note.
				if len(roleToolSet) > 0 {
					if !roleToolSet[externalToolKey] {
						continue // ，
					}
				}

				// English note.
				var mcpName, actualToolName string
				if idx := strings.Index(externalTool.Name, "::"); idx > 0 {
					mcpName = externalTool.Name[:idx]
					actualToolName = externalTool.Name[idx+2:]
				} else {
					continue // 
				}

				// English note.
				enabled := false
				if cfg, exists := externalMCPConfigs[mcpName]; exists {
					// English note.
					if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
						enabled = false // MCP，
					} else {
						// English note.
						// English note.
						if cfg.ToolEnabled == nil {
							enabled = true // ，
						} else if toolEnabled, exists := cfg.ToolEnabled[actualToolName]; exists {
							enabled = toolEnabled // 
						} else {
							enabled = true // ，
						}
					}
				}

				// English note.
				if !enabled {
					continue
				}

				// English note.
				description := externalTool.ShortDescription
				if description == "" {
					description = externalTool.Description
				}

				// English note.
				convertedSchema := a.convertSchemaTypes(externalTool.InputSchema)

				// English note.
				// English note.
				openAIName := strings.ReplaceAll(externalTool.Name, "::", "__")

				// English note.
				a.mu.Lock()
				a.toolNameMapping[openAIName] = externalTool.Name
				a.mu.Unlock()

				tools = append(tools, Tool{
					Type: "function",
					Function: FunctionDefinition{
						Name:        openAIName, // OpenAI
						Description: description,
						Parameters:  convertedSchema,
					},
				})
			}
		}
	}

	a.logger.Debug("",
		zap.Int("internalTools", len(mcpTools)),
		zap.Int("totalTools", len(tools)),
	)

	return tools
}

// English note.
func (a *Agent) convertSchemaTypes(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return schema
	}

	// English note.
	converted := make(map[string]interface{})
	for k, v := range schema {
		converted[k] = v
	}

	// English note.
	if properties, ok := converted["properties"].(map[string]interface{}); ok {
		convertedProperties := make(map[string]interface{})
		for propName, propValue := range properties {
			if prop, ok := propValue.(map[string]interface{}); ok {
				convertedProp := make(map[string]interface{})
				for pk, pv := range prop {
					if pk == "type" {
						// English note.
						if typeStr, ok := pv.(string); ok {
							convertedProp[pk] = a.convertToOpenAIType(typeStr)
						} else {
							convertedProp[pk] = pv
						}
					} else {
						convertedProp[pk] = pv
					}
				}
				convertedProperties[propName] = convertedProp
			} else {
				convertedProperties[propName] = propValue
			}
		}
		converted["properties"] = convertedProperties
	}

	return converted
}

// English note.
func (a *Agent) convertToOpenAIType(configType string) string {
	switch configType {
	case "bool":
		return "boolean"
	case "int", "integer":
		return "number"
	case "float", "double":
		return "number"
	case "string", "array", "object":
		return configType
	default:
		// English note.
		return configType
	}
}

// English note.
func (a *Agent) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// English note.
	retryableErrors := []string{
		"connection reset",
		"connection reset by peer",
		"connection refused",
		"timeout",
		"i/o timeout",
		"context deadline exceeded",
		"no such host",
		"network is unreachable",
		"broken pipe",
		"EOF",
		"read tcp",
		"write tcp",
		"dial tcp",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}
	return false
}

// English note.
func (a *Agent) callOpenAI(ctx context.Context, messages []ChatMessage, tools []Tool) (*OpenAIResponse, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := a.callOpenAISingle(ctx, messages, tools)
		if err == nil {
			if attempt > 0 {
				a.logger.Info("OpenAI API",
					zap.Int("attempt", attempt+1),
					zap.Int("maxRetries", maxRetries),
				)
			}
			return response, nil
		}

		lastErr = err

		// English note.
		if !a.isRetryableError(err) {
			return nil, err
		}

		// English note.
		if attempt < maxRetries-1 {
			// English note.
			backoff := time.Duration(1<<uint(attempt+1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second // 30
			}
			a.logger.Warn("OpenAI API，",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			// English note.
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf(": %w", ctx.Err())
			case <-time.After(backoff):
				// English note.
			}
		}
	}

	return nil, fmt.Errorf("%d: %w", maxRetries, lastErr)
}

// English note.
func (a *Agent) callOpenAISingle(ctx context.Context, messages []ChatMessage, tools []Tool) (*OpenAIResponse, error) {
	reqBody := OpenAIRequest{
		Model:    a.config.Model,
		Messages: messages,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	a.logger.Debug("OpenAI",
		zap.Int("messagesCount", len(messages)),
		zap.Int("toolsCount", len(tools)),
	)

	var response OpenAIResponse
	if a.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI")
	}
	if err := a.openAIClient.ChatCompletion(ctx, reqBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// English note.
// English note.
func (a *Agent) callOpenAISingleStreamText(ctx context.Context, messages []ChatMessage, tools []Tool, onDelta func(delta string) error) (string, error) {
	reqBody := OpenAIRequest{
		Model:    a.config.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	if a.openAIClient == nil {
		return "", fmt.Errorf("OpenAI")
	}

	return a.openAIClient.ChatCompletionStream(ctx, reqBody, onDelta)
}

// English note.
func (a *Agent) callOpenAIStreamText(ctx context.Context, messages []ChatMessage, tools []Tool, onDelta func(delta string) error) (string, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		var deltasSent bool
		full, err := a.callOpenAISingleStreamText(ctx, messages, tools, func(delta string) error {
			deltasSent = true
			return onDelta(delta)
		})
		if err == nil {
			if attempt > 0 {
				a.logger.Info("OpenAI stream ",
					zap.Int("attempt", attempt+1),
					zap.Int("maxRetries", maxRetries),
				)
			}
			return full, nil
		}

		lastErr = err
		// English note.
		if deltasSent {
			return "", err
		}

		if !a.isRetryableError(err) {
			return "", err
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt+1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			a.logger.Warn("OpenAI stream ，",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return "", fmt.Errorf(": %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
	}

	return "", fmt.Errorf("%d: %w", maxRetries, lastErr)
}

// English note.
func (a *Agent) callOpenAISingleStreamWithToolCalls(
	ctx context.Context,
	messages []ChatMessage,
	tools []Tool,
	onContentDelta func(delta string) error,
) (*OpenAIResponse, error) {
	reqBody := OpenAIRequest{
		Model:    a.config.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
	}
	if a.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI")
	}

	content, streamToolCalls, finishReason, err := a.openAIClient.ChatCompletionStreamWithToolCalls(ctx, reqBody, onContentDelta)
	if err != nil {
		return nil, err
	}

	toolCalls := make([]ToolCall, 0, len(streamToolCalls))
	for _, stc := range streamToolCalls {
		fnArgsStr := stc.FunctionArgsStr
		args := make(map[string]interface{})
		if strings.TrimSpace(fnArgsStr) != "" {
			if err := json.Unmarshal([]byte(fnArgsStr), &args); err != nil {
				// English note.
				args = map[string]interface{}{"raw": fnArgsStr}
			}
		}

		typ := stc.Type
		if strings.TrimSpace(typ) == "" {
			typ = "function"
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:   stc.ID,
			Type: typ,
			Function: FunctionCall{
				Name:      stc.FunctionName,
				Arguments: args,
			},
		})
	}

	response := &OpenAIResponse{
		ID: "",
		Choices: []Choice{
			{
				Message: MessageWithTools{
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}
	return response, nil
}

// English note.
func (a *Agent) callOpenAIStreamWithToolCalls(
	ctx context.Context,
	messages []ChatMessage,
	tools []Tool,
	onContentDelta func(delta string) error,
) (*OpenAIResponse, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		deltasSent := false
		resp, err := a.callOpenAISingleStreamWithToolCalls(ctx, messages, tools, func(delta string) error {
			deltasSent = true
			if onContentDelta != nil {
				return onContentDelta(delta)
			}
			return nil
		})
		if err == nil {
			if attempt > 0 {
				a.logger.Info("OpenAI stream ",
					zap.Int("attempt", attempt+1),
					zap.Int("maxRetries", maxRetries),
				)
			}
			return resp, nil
		}

		lastErr = err
		if deltasSent {
			// English note.
			return nil, err
		}

		if !a.isRetryableError(err) {
			return nil, err
		}
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<uint(attempt+1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			a.logger.Warn("OpenAI stream ，",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf(": %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("%d: %w", maxRetries, lastErr)
}

// English note.
type ToolExecutionResult struct {
	Result      string
	ExecutionID string
	IsError     bool // 
}

// English note.
// English note.
func (a *Agent) executeToolViaMCP(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	a.logger.Info("MCP",
		zap.String("tool", toolName),
		zap.Any("args", args),
	)

	// English note.
	if toolName == builtin.ToolRecordVulnerability {
		a.mu.RLock()
		conversationID := a.currentConversationID
		a.mu.RUnlock()

		if conversationID != "" {
			args["conversation_id"] = conversationID
			a.logger.Debug("conversation_idrecord_vulnerability",
				zap.String("conversation_id", conversationID),
			)
		} else {
			a.logger.Warn("record_vulnerabilityconversation_id")
		}
	}

	var result *mcp.ToolResult
	var executionID string
	var err error

	// English note.
	toolCtx := ctx
	var toolCancel context.CancelFunc
	if a.agentConfig != nil && a.agentConfig.ToolTimeoutMinutes > 0 {
		toolCtx, toolCancel = context.WithTimeout(ctx, time.Duration(a.agentConfig.ToolTimeoutMinutes)*time.Minute)
		defer func() {
			if toolCancel != nil {
				toolCancel()
			}
		}()
	}

	// English note.
	a.mu.RLock()
	originalToolName, isExternalTool := a.toolNameMapping[toolName]
	a.mu.RUnlock()

	if isExternalTool && a.externalMCPMgr != nil {
		// English note.
		a.logger.Debug("MCP",
			zap.String("openAIName", toolName),
			zap.String("originalName", originalToolName),
		)
		result, executionID, err = a.externalMCPMgr.CallTool(toolCtx, originalToolName, args)
	} else {
		// English note.
		result, executionID, err = a.mcpServer.CallTool(toolCtx, toolName, args)
	}

	// English note.
	if err != nil {
		detail := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			min := 10
			if a.agentConfig != nil && a.agentConfig.ToolTimeoutMinutes > 0 {
				min = a.agentConfig.ToolTimeoutMinutes
			}
			detail = fmt.Sprintf(" %d （ config.yaml  agent.tool_timeout_minutes ）", min)
		}
		errorMsg := fmt.Sprintf(`

: %s
: 
: %s

：
-  "%s" 
- （agent.tool_timeout_minutes）
- 
- 

：
- 
- ， agent.tool_timeout_minutes
- 
- ，`, toolName, detail, toolName)

		return &ToolExecutionResult{
			Result:      errorMsg,
			ExecutionID: executionID,
			IsError:     true,
		}, nil //  nil ，
	}

	// English note.
	var resultText strings.Builder
	for _, content := range result.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}

	resultStr := resultText.String()
	resultSize := len(resultStr)

	// English note.
	a.mu.RLock()
	threshold := a.largeResultThreshold
	storage := a.resultStorage
	a.mu.RUnlock()

	if resultSize > threshold && storage != nil {
		// English note.
		go func() {
			if err := storage.SaveResult(executionID, toolName, resultStr); err != nil {
				a.logger.Warn("",
					zap.String("executionID", executionID),
					zap.String("toolName", toolName),
					zap.Error(err),
				)
			} else {
				a.logger.Info("",
					zap.String("executionID", executionID),
					zap.String("toolName", toolName),
					zap.Int("size", resultSize),
				)
			}
		}()

		// English note.
		lines := strings.Split(resultStr, "\n")
		filePath := ""
		if storage != nil {
			filePath = storage.GetResultPath(executionID)
		}
		notification := a.formatMinimalNotification(executionID, toolName, resultSize, len(lines), filePath)

		return &ToolExecutionResult{
			Result:      notification,
			ExecutionID: executionID,
			IsError:     result != nil && result.IsError,
		}, nil
	}

	return &ToolExecutionResult{
		Result:      resultStr,
		ExecutionID: executionID,
		IsError:     result != nil && result.IsError,
	}, nil
}

// English note.
func (a *Agent) formatMinimalNotification(executionID string, toolName string, size int, lineCount int, filePath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("。（ID: %s）。\n\n", executionID))
	sb.WriteString("：\n")
	sb.WriteString(fmt.Sprintf("  - : %s\n", toolName))
	sb.WriteString(fmt.Sprintf("  - : %d  (%.2f KB)\n", size, float64(size)/1024))
	sb.WriteString(fmt.Sprintf("  - : %d \n", lineCount))
	if filePath != "" {
		sb.WriteString(fmt.Sprintf("  - : %s\n", filePath))
	}
	sb.WriteString("\n")
	sb.WriteString(" query_execution_result ：\n")
	sb.WriteString(fmt.Sprintf("  - : query_execution_result(execution_id=\"%s\", page=1, limit=100)\n", executionID))
	sb.WriteString(fmt.Sprintf("  - : query_execution_result(execution_id=\"%s\", search=\"\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - : query_execution_result(execution_id=\"%s\", filter=\"error\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - : query_execution_result(execution_id=\"%s\", search=\"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", use_regex=true)\n", executionID))
	sb.WriteString("\n")
	if filePath != "" {
		sb.WriteString(" query_execution_result ，：\n")
		sb.WriteString("\n")
		sb.WriteString("**：**\n")
		sb.WriteString(fmt.Sprintf("  - 100: exec(command=\"head\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 100: exec(command=\"tail\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 50-150: exec(command=\"sed\", args=[\"-n\", \"50,150p\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**：**\n")
		sb.WriteString(fmt.Sprintf("  - : exec(command=\"grep\", args=[\"\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - IP: exec(command=\"grep\", args=[\"-E\", \"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - : exec(command=\"grep\", args=[\"-i\", \"\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - : exec(command=\"grep\", args=[\"-n\", \"\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**：**\n")
		sb.WriteString(fmt.Sprintf("  - : exec(command=\"wc\", args=[\"-l\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - error: exec(command=\"grep\", args=[\"error\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - : exec(command=\"grep\", args=[\"-v\", \"^$\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**（）：**\n")
		sb.WriteString(fmt.Sprintf("  -  cat : cat(file=\"%s\")\n", filePath))
		sb.WriteString(fmt.Sprintf("  -  exec : exec(command=\"cat\", args=[\"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**：**\n")
		sb.WriteString("  - \n")
		sb.WriteString("  - ，\n")
		sb.WriteString("  -  POSIX \n")
	}

	return sb.String()
}

// English note.
func (a *Agent) UpdateConfig(cfg *config.OpenAIConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
	if a.openAIClient != nil {
		a.openAIClient.UpdateConfig(cfg)
	}

	// English note.
	if a.memoryCompressor != nil {
		a.memoryCompressor.UpdateConfig(cfg)
	}

	a.logger.Info("Agent",
		zap.String("base_url", cfg.BaseURL),
		zap.String("model", cfg.Model),
	)
}

// English note.
func (a *Agent) UpdateMaxIterations(maxIterations int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if maxIterations > 0 {
		a.maxIterations = maxIterations
		a.logger.Info("Agent", zap.Int("max_iterations", maxIterations))
	}
}

// English note.
func (a *Agent) formatToolError(toolName string, args map[string]interface{}, err error) string {
	errorMsg := fmt.Sprintf(`

: %s
: %v
: %v

：
1. ，
2. ，
3. ，
4. ，`, toolName, args, err)

	return errorMsg
}

// English note.
func (a *Agent) applyMemoryCompression(ctx context.Context, messages []ChatMessage, reservedTokens int) []ChatMessage {
	if a.memoryCompressor == nil {
		return messages
	}

	compressed, changed, err := a.memoryCompressor.CompressHistory(ctx, messages, reservedTokens)
	if err != nil {
		a.logger.Warn("，", zap.Error(err))
		return messages
	}
	if changed {
		a.logger.Info("",
			zap.Int("originalMessages", len(messages)),
			zap.Int("compressedMessages", len(compressed)),
		)
		return compressed
	}

	return messages
}

// English note.
func (a *Agent) countToolsTokens(tools []Tool) int {
	if len(tools) == 0 || a.memoryCompressor == nil {
		return 0
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return 0
	}
	return a.memoryCompressor.CountTextTokens(string(data))
}

// English note.
func (a *Agent) handleMissingToolError(errMsg string, messages *[]ChatMessage) (bool, string) {
	lowerMsg := strings.ToLower(errMsg)
	if !(strings.Contains(lowerMsg, "non-exist tool") || strings.Contains(lowerMsg, "non exist tool")) {
		return false, ""
	}

	toolName := extractQuotedToolName(errMsg)
	if toolName == "" {
		toolName = "unknown_tool"
	}

	notice := fmt.Sprintf("System notice: the previous call failed with error: %s. Please verify tool availability and proceed using existing tools or pure reasoning.", errMsg)
	*messages = append(*messages, ChatMessage{
		Role:    "user",
		Content: notice,
	})

	return true, toolName
}

// English note.
func (a *Agent) handleToolRoleError(errMsg string, messages *[]ChatMessage) bool {
	if messages == nil {
		return false
	}

	lowerMsg := strings.ToLower(errMsg)
	if !(strings.Contains(lowerMsg, "role 'tool'") && strings.Contains(lowerMsg, "tool_calls")) {
		return false
	}

	fixed := a.repairOrphanToolMessages(messages)
	if !fixed {
		return false
	}

	notice := "System notice: the previous call failed because some tool outputs lost their corresponding assistant tool_calls context. The history has been repaired. Please continue."
	*messages = append(*messages, ChatMessage{
		Role:    "user",
		Content: notice,
	})

	return true
}

// English note.
// English note.
// English note.
func (a *Agent) RepairOrphanToolMessages(messages *[]ChatMessage) bool {
	return a.repairOrphanToolMessages(messages)
}

// English note.
// English note.
func (a *Agent) repairOrphanToolMessages(messages *[]ChatMessage) bool {
	if messages == nil {
		return false
	}

	msgs := *messages
	if len(msgs) == 0 {
		return false
	}

	pending := make(map[string]int)
	cleaned := make([]ChatMessage, 0, len(msgs))
	removed := false

	for _, msg := range msgs {
		switch strings.ToLower(msg.Role) {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// English note.
				for _, tc := range msg.ToolCalls {
					if tc.ID != "" {
						pending[tc.ID]++
					}
				}
			}
			cleaned = append(cleaned, msg)
		case "tool":
			callID := msg.ToolCallID
			if callID == "" {
				removed = true
				continue
			}
			if count, exists := pending[callID]; exists && count > 0 {
				if count == 1 {
					delete(pending, callID)
				} else {
					pending[callID] = count - 1
				}
				cleaned = append(cleaned, msg)
			} else {
				removed = true
				continue
			}
		default:
			cleaned = append(cleaned, msg)
		}
	}

	// English note.
	// English note.
	if len(pending) > 0 {
		// English note.
		for i := len(cleaned) - 1; i >= 0; i-- {
			if strings.ToLower(cleaned[i].Role) == "assistant" && len(cleaned[i].ToolCalls) > 0 {
				// English note.
				originalCount := len(cleaned[i].ToolCalls)
				validToolCalls := make([]ToolCall, 0)
				for _, tc := range cleaned[i].ToolCalls {
					if tc.ID != "" && pending[tc.ID] > 0 {
						// English note.
						removed = true
						delete(pending, tc.ID)
					} else {
						validToolCalls = append(validToolCalls, tc)
					}
				}
				// English note.
				if len(validToolCalls) != originalCount {
					cleaned[i].ToolCalls = validToolCalls
					a.logger.Info("tool_calls，",
						zap.Int("removed_count", originalCount-len(validToolCalls)),
					)
				}
				break
			}
		}
	}

	if removed {
		a.logger.Warn("tooltool_calls",
			zap.Int("original_messages", len(msgs)),
			zap.Int("cleaned_messages", len(cleaned)),
		)
		*messages = cleaned
	}

	return removed
}

// English note.
func (a *Agent) ToolsForRole(roleTools []string) []Tool {
	return a.getAvailableTools(roleTools)
}

// English note.
func (a *Agent) ExecuteMCPToolForConversation(ctx context.Context, conversationID, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	a.mu.Lock()
	prev := a.currentConversationID
	a.currentConversationID = conversationID
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.currentConversationID = prev
		a.mu.Unlock()
	}()
	return a.executeToolViaMCP(ctx, toolName, args)
}

// English note.
func extractQuotedToolName(errMsg string) string {
	start := strings.Index(errMsg, "\"")
	if start == -1 {
		return ""
	}
	rest := errMsg[start+1:]
	end := strings.Index(rest, "\"")
	if end == -1 {
		return ""
	}
	return rest[:end]
}
