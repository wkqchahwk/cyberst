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
	externalMCPMgr        *mcp.ExternalMCPManager // 外部MCP管理器
	logger                *zap.Logger
	maxIterations         int
	resultStorage         ResultStorage     // 结果存储
	largeResultThreshold  int               // 大结果阈值（字节）
	mu                    sync.RWMutex      // 添加互斥锁以支持并发更新
	toolNameMapping       map[string]string // 工具名称映射：OpenAI格式 -> 原始格式（用于外部MCP工具）
	currentConversationID string            // 当前对话ID（用于自动传递给工具）
	promptBaseDir         string            // 解析 system_prompt_path 时相对路径的基准目录（通常为 config.yaml 所在目录）
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
		ResponseHeaderTimeout: 60 * time.Minute, // 响应头超时：增加到15分钟，应对大响应
		DisableKeepAlives:     false,            // 启用连接复用
	}

	// English note.
	// English note.
	httpClient := &http.Client{
		Timeout:   30 * time.Minute, // 从5分钟增加到30分钟
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
			logger.Warn("初始化MemoryCompressor失败，将跳过上下文压缩", zap.Error(err))
		} else {
			memoryCompressor = mc
		}
	} else {
		logger.Warn("OpenAI配置为空，无法初始化MemoryCompressor")
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
		toolNameMapping:      make(map[string]string), // 初始化工具名称映射
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
	LastReActInput  string // 最后一轮ReAct的输入（压缩后的messages，JSON格式）
	LastReActOutput string // 最终大模型的输出
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
				a.logger.Warn("读取单代理 system_prompt_path 失败，使用内置提示", zap.String("path", path), zap.Error(err))
			} else if s := strings.TrimSpace(string(b)); s != "" {
				systemPrompt = s
			}
		}
	}
	if len(roleSkills) > 0 {
		var skillsHint strings.Builder
		skillsHint.WriteString("\n\n本角色推荐使用的Skills：\n")
		for i, skillName := range roleSkills {
			if i > 0 {
				skillsHint.WriteString("、")
			}
			skillsHint.WriteString("`")
			skillsHint.WriteString(skillName)
			skillsHint.WriteString("`")
		}
		skillsHint.WriteString("\n- 这些名称与 skills/ 下 SKILL.md 的 `name` 一致。")
		skillsHint.WriteString("\n- 若当前会话已启用 Eino 内置 `skill` 工具，请按需加载；否则以 MCP 与文本工作流完成。")
		skillsHint.WriteString("\n- 例如传入 skill 参数为 `")
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
				a.logger.Warn("读取单代理 system_prompt_path 失败，使用内置提示", zap.String("path", path), zap.Error(err))
			} else if s := strings.TrimSpace(string(b)); s != "" {
				systemPrompt = s
			}
		}
	}

	// English note.
	if len(roleSkills) > 0 {
		var skillsHint strings.Builder
		skillsHint.WriteString("\n\n本角色推荐使用的Skills：\n")
		for i, skillName := range roleSkills {
			if i > 0 {
				skillsHint.WriteString("、")
			}
			skillsHint.WriteString("`")
			skillsHint.WriteString(skillName)
			skillsHint.WriteString("`")
		}
		skillsHint.WriteString("\n- 这些名称与 skills/ 下 SKILL.md 的 `name` 一致；在 **Eino 多代理** 会话中请用内置 `skill` 工具按需加载全文")
		skillsHint.WriteString("\n- 例如：在支持 Eino skill 工具时传入 skill 参数为 `")
		skillsHint.WriteString(roleSkills[0])
		skillsHint.WriteString("`")
		skillsHint.WriteString("\n- 单代理 MCP 模式不会注入 skill 工具；需要时请使用多代理（DeepAgent）")
		systemPrompt += skillsHint.String()
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// English note.
	a.logger.Info("处理历史消息",
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
			a.logger.Info("添加历史消息到上下文",
				zap.Int("index", i),
				zap.String("role", msg.Role),
				zap.String("content", contentPreview),
				zap.Int("toolCalls", len(msg.ToolCalls)),
				zap.String("toolCallID", msg.ToolCallID),
			)
		}
	}

	a.logger.Info("构建消息数组",
		zap.Int("historyMessages", len(historyMessages)),
		zap.Int("addedMessages", addedCount),
		zap.Int("totalMessages", len(messages)),
	)

	// English note.
	// English note.
	if len(messages) > 0 {
		if fixed := a.repairOrphanToolMessages(&messages); fixed {
			a.logger.Info("修复了历史消息中的失配tool消息")
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
			a.logger.Warn("序列化ReAct输入失败", zap.Error(err))
		} else {
			currentReActInput = string(messagesJSON)
			// English note.
			result.LastReActInput = currentReActInput
		}

		// English note.
		select {
		case <-ctx.Done():
			// English note.
			a.logger.Info("检测到上下文取消，保存当前ReAct数据", zap.Error(ctx.Err()))
			result.LastReActInput = currentReActInput
			if ctx.Err() == context.Canceled {
				result.Response = "任务已被取消。"
			} else {
				result.Response = fmt.Sprintf("任务执行中断: %v", ctx.Err())
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
			sendProgress("iteration", "开始分析请求并制定测试策略", map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		} else if isLastIteration {
			sendProgress("iteration", fmt.Sprintf("第 %d 轮迭代（最后一次）", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
				"isLast":    true,
			})
		} else {
			sendProgress("iteration", fmt.Sprintf("第 %d 轮迭代", i+1), map[string]interface{}{
				"iteration": i + 1,
				"total":     maxIterations,
			})
		}

		// English note.
		if i == 0 {
			a.logger.Info("调用OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
			// English note.
			for j, msg := range messages {
				if j >= 5 { // 只记录前5条
					break
				}
				contentPreview := msg.Content
				if len(contentPreview) > 100 {
					contentPreview = contentPreview[:100] + "..."
				}
				a.logger.Debug("消息内容",
					zap.Int("index", j),
					zap.String("role", msg.Role),
					zap.String("content", contentPreview),
				)
			}
		} else {
			a.logger.Info("调用OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
		}

		// English note.
		sendProgress("progress", "正在调用AI模型...", nil)
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
			errorMsg := fmt.Sprintf("调用OpenAI失败: %v", err)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			a.logger.Warn("OpenAI调用失败，已保存ReAct数据", zap.Error(err))
			return result, fmt.Errorf("调用OpenAI失败: %w", err)
		}

		if response.Error != nil {
			if handled, toolName := a.handleMissingToolError(response.Error.Message, &messages); handled {
				sendProgress("warning", fmt.Sprintf("模型尝试调用不存在的工具：%s，已提示其改用可用工具。", toolName), map[string]interface{}{
					"toolName": toolName,
				})
				a.logger.Warn("模型调用了不存在的工具，将重试",
					zap.String("tool", toolName),
					zap.String("error", response.Error.Message),
				)
				continue
			}
			if a.handleToolRoleError(response.Error.Message, &messages) {
				sendProgress("warning", "检测到未配对的工具结果，已自动修复上下文并重试。", map[string]interface{}{
					"error": response.Error.Message,
				})
				a.logger.Warn("检测到未配对的工具消息，已修复并重试",
					zap.String("error", response.Error.Message),
				)
				continue
			}
			// English note.
			result.LastReActInput = currentReActInput
			errorMsg := fmt.Sprintf("OpenAI错误: %s", response.Error.Message)
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("OpenAI错误: %s", response.Error.Message)
		}

		if len(response.Choices) == 0 {
			// English note.
			result.LastReActInput = currentReActInput
			errorMsg := "没有收到响应"
			result.Response = errorMsg
			result.LastReActOutput = errorMsg
			return result, fmt.Errorf("没有收到响应")
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
			sendProgress("tool_calls_detected", fmt.Sprintf("检测到 %d 个工具调用", len(choice.Message.ToolCalls)), map[string]interface{}{
				"count":     len(choice.Message.ToolCalls),
				"iteration": i + 1,
			})

			// English note.
			for idx, toolCall := range choice.Message.ToolCalls {
				// English note.
				toolArgsJSON, _ := json.Marshal(toolCall.Function.Arguments)
				sendProgress("tool_call", fmt.Sprintf("正在调用工具: %s", toolCall.Function.Name), map[string]interface{}{
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
					sendProgress("tool_result", fmt.Sprintf("工具 %s 执行失败", toolCall.Function.Name), map[string]interface{}{
						"toolName":   toolCall.Function.Name,
						"success":    false,
						"isError":    true,
						"error":      err.Error(),
						"toolCallId": toolCall.ID,
						"index":      idx + 1,
						"total":      len(choice.Message.ToolCalls),
						"iteration":  i + 1,
					})

					a.logger.Warn("工具执行失败，已返回详细错误信息",
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
					sendProgress("tool_result", fmt.Sprintf("工具 %s 执行完成", toolCall.Function.Name), map[string]interface{}{
						"toolName":      toolCall.Function.Name,
						"success":       !execResult.IsError,
						"isError":       execResult.IsError,
						"result":        execResult.Result, // 完整结果
						"resultPreview": resultPreview,     // 预览结果
						"executionId":   execResult.ExecutionID,
						"toolCallId":    toolCall.ID,
						"index":         idx + 1,
						"total":         len(choice.Message.ToolCalls),
						"iteration":     i + 1,
					})

					// English note.
					if execResult.IsError {
						a.logger.Warn("工具返回错误结果，但继续处理",
							zap.String("tool", toolCall.Function.Name),
							zap.String("result", execResult.Result),
						)
					}
				}
			}

			// English note.
			if isLastIteration {
				sendProgress("progress", "最后一次迭代：正在生成总结和下一步计划...", nil)
				// English note.
				messages = append(messages, ChatMessage{
					Role:    "user",
					Content: "这是最后一次迭代。请总结到目前为止的所有测试结果、发现的问题和已完成的工作。如果需要继续测试，请提供详细的下一步执行计划。请直接回复，不要调用工具。",
				})
				messages = a.applyMemoryCompression(ctx, messages, 0) // 总结时不带 tools，不预留
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
					sendProgress("progress", "总结生成完成", nil)
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
			sendProgress("progress", "最后一次迭代：正在生成总结和下一步计划...", nil)
			// English note.
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: "这是最后一次迭代。请总结到目前为止的所有测试结果、发现的问题和已完成的工作。如果需要继续测试，请提供详细的下一步执行计划。请直接回复，不要调用工具。",
			})
			messages = a.applyMemoryCompression(ctx, messages, 0) // 总结时不带 tools，不预留
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
				sendProgress("progress", "总结生成完成", nil)
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
			sendProgress("progress", "正在生成最终回复...", nil)
			result.Response = choice.Message.Content
			result.LastReActOutput = result.Response
			return result, nil
		}
	}

	// English note.
	// English note.
	sendProgress("progress", "达到最大迭代次数，正在生成总结...", nil)
	finalSummaryPrompt := ChatMessage{
		Role:    "user",
		Content: fmt.Sprintf("已达到最大迭代次数（%d轮）。请总结到目前为止的所有测试结果、发现的问题和已完成的工作。如果需要继续测试，请提供详细的下一步执行计划。请直接回复，不要调用工具。", a.maxIterations),
	}
	messages = append(messages, finalSummaryPrompt)
	messages = a.applyMemoryCompression(ctx, messages, 0) // 总结时不带 tools，不预留

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
		sendProgress("progress", "总结生成完成", nil)
		return result, nil
	}

	// English note.
	result.Response = fmt.Sprintf("已达到最大迭代次数（%d轮）。系统已执行了多轮测试，但由于达到迭代上限，无法继续自动执行。建议您查看已执行的工具结果，或提出新的测试请求以继续测试。", a.maxIterations)
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
			toolKey := mcpTool.Name // 内置工具使用工具名称作为key
			if !roleToolSet[toolKey] {
				continue // 不在角色工具列表中，跳过
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
				Description: description, // 使用简短描述减少token消耗
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
			a.logger.Warn("获取外部MCP工具失败", zap.Error(err))
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
						continue // 不在角色工具列表中，跳过
					}
				}

				// English note.
				var mcpName, actualToolName string
				if idx := strings.Index(externalTool.Name, "::"); idx > 0 {
					mcpName = externalTool.Name[:idx]
					actualToolName = externalTool.Name[idx+2:]
				} else {
					continue // 跳过格式不正确的工具
				}

				// English note.
				enabled := false
				if cfg, exists := externalMCPConfigs[mcpName]; exists {
					// English note.
					if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
						enabled = false // MCP未启用，所有工具都禁用
					} else {
						// English note.
						// English note.
						if cfg.ToolEnabled == nil {
							enabled = true // 未设置工具状态，默认为启用
						} else if toolEnabled, exists := cfg.ToolEnabled[actualToolName]; exists {
							enabled = toolEnabled // 使用配置的工具状态
						} else {
							enabled = true // 工具未在配置中，默认为启用
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
						Name:        openAIName, // 使用符合OpenAI规范的名称
						Description: description,
						Parameters:  convertedSchema,
					},
				})
			}
		}
	}

	a.logger.Debug("获取可用工具列表",
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
				a.logger.Info("OpenAI API调用重试成功",
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
				backoff = 30 * time.Second // 最大30秒
			}
			a.logger.Warn("OpenAI API调用失败，准备重试",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			// English note.
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			case <-time.After(backoff):
				// English note.
			}
		}
	}

	return nil, fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
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

	a.logger.Debug("准备发送OpenAI请求",
		zap.Int("messagesCount", len(messages)),
		zap.Int("toolsCount", len(tools)),
	)

	var response OpenAIResponse
	if a.openAIClient == nil {
		return nil, fmt.Errorf("OpenAI客户端未初始化")
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
		return "", fmt.Errorf("OpenAI客户端未初始化")
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
				a.logger.Info("OpenAI stream 调用重试成功",
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
			a.logger.Warn("OpenAI stream 调用失败，准备重试",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return "", fmt.Errorf("上下文已取消: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
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
		return nil, fmt.Errorf("OpenAI客户端未初始化")
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
				a.logger.Info("OpenAI stream 调用重试成功",
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
			a.logger.Warn("OpenAI stream 调用失败，准备重试",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("maxRetries", maxRetries),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("上下文已取消: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// English note.
type ToolExecutionResult struct {
	Result      string
	ExecutionID string
	IsError     bool // 标记是否为错误结果
}

// English note.
// English note.
func (a *Agent) executeToolViaMCP(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	a.logger.Info("通过MCP执行工具",
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
			a.logger.Debug("自动添加conversation_id到record_vulnerability工具",
				zap.String("conversation_id", conversationID),
			)
		} else {
			a.logger.Warn("record_vulnerability工具调用时conversation_id为空")
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
		a.logger.Debug("调用外部MCP工具",
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
			detail = fmt.Sprintf("工具执行超过 %d 分钟被自动终止（可在 config.yaml 的 agent.tool_timeout_minutes 中调整）", min)
		}
		errorMsg := fmt.Sprintf(`工具调用失败

工具名称: %s
错误类型: 系统错误
错误详情: %s

可能的原因：
- 工具 "%s" 不存在或未启用
- 单次执行超时（agent.tool_timeout_minutes）
- 系统配置问题
- 网络或权限问题

建议：
- 检查工具名称是否正确
- 若需更长执行时间，可适当增大 agent.tool_timeout_minutes
- 尝试使用其他替代工具
- 如果这是必需的工具，请向用户说明情况`, toolName, detail, toolName)

		return &ToolExecutionResult{
			Result:      errorMsg,
			ExecutionID: executionID,
			IsError:     true,
		}, nil // 返回 nil 错误，让调用者处理结果
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
				a.logger.Warn("保存大结果失败",
					zap.String("executionID", executionID),
					zap.String("toolName", toolName),
					zap.Error(err),
				)
			} else {
				a.logger.Info("大结果已保存",
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

	sb.WriteString(fmt.Sprintf("工具执行完成。结果已保存（ID: %s）。\n\n", executionID))
	sb.WriteString("结果信息：\n")
	sb.WriteString(fmt.Sprintf("  - 工具: %s\n", toolName))
	sb.WriteString(fmt.Sprintf("  - 大小: %d 字节 (%.2f KB)\n", size, float64(size)/1024))
	sb.WriteString(fmt.Sprintf("  - 行数: %d 行\n", lineCount))
	if filePath != "" {
		sb.WriteString(fmt.Sprintf("  - 文件路径: %s\n", filePath))
	}
	sb.WriteString("\n")
	sb.WriteString("推荐使用 query_execution_result 工具查询完整结果：\n")
	sb.WriteString(fmt.Sprintf("  - 查询第一页: query_execution_result(execution_id=\"%s\", page=1, limit=100)\n", executionID))
	sb.WriteString(fmt.Sprintf("  - 搜索关键词: query_execution_result(execution_id=\"%s\", search=\"关键词\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - 过滤条件: query_execution_result(execution_id=\"%s\", filter=\"error\")\n", executionID))
	sb.WriteString(fmt.Sprintf("  - 正则匹配: query_execution_result(execution_id=\"%s\", search=\"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", use_regex=true)\n", executionID))
	sb.WriteString("\n")
	if filePath != "" {
		sb.WriteString("如果 query_execution_result 工具不满足需求，也可以使用其他工具处理文件：\n")
		sb.WriteString("\n")
		sb.WriteString("**分段读取示例：**\n")
		sb.WriteString(fmt.Sprintf("  - 查看前100行: exec(command=\"head\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 查看后100行: exec(command=\"tail\", args=[\"-n\", \"100\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 查看第50-150行: exec(command=\"sed\", args=[\"-n\", \"50,150p\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**搜索和正则匹配示例：**\n")
		sb.WriteString(fmt.Sprintf("  - 搜索关键词: exec(command=\"grep\", args=[\"关键词\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 正则匹配IP地址: exec(command=\"grep\", args=[\"-E\", \"\\\\d+\\\\.\\\\d+\\\\.\\\\d+\\\\.\\\\d+\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 不区分大小写搜索: exec(command=\"grep\", args=[\"-i\", \"关键词\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 显示匹配行号: exec(command=\"grep\", args=[\"-n\", \"关键词\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**过滤和统计示例：**\n")
		sb.WriteString(fmt.Sprintf("  - 统计总行数: exec(command=\"wc\", args=[\"-l\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 过滤包含error的行: exec(command=\"grep\", args=[\"error\", \"%s\"])\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 排除空行: exec(command=\"grep\", args=[\"-v\", \"^$\", \"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**完整读取（不推荐大文件）：**\n")
		sb.WriteString(fmt.Sprintf("  - 使用 cat 工具: cat(file=\"%s\")\n", filePath))
		sb.WriteString(fmt.Sprintf("  - 使用 exec 工具: exec(command=\"cat\", args=[\"%s\"])\n", filePath))
		sb.WriteString("\n")
		sb.WriteString("**注意：**\n")
		sb.WriteString("  - 直接读取大文件可能会再次触发大结果保存机制\n")
		sb.WriteString("  - 建议优先使用分段读取和搜索功能，避免一次性加载整个文件\n")
		sb.WriteString("  - 正则表达式语法遵循标准 POSIX 正则表达式规范\n")
	}

	return sb.String()
}

// English note.
func (a *Agent) UpdateConfig(cfg *config.OpenAIConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg

	// English note.
	if a.memoryCompressor != nil {
		a.memoryCompressor.UpdateConfig(cfg)
	}

	a.logger.Info("Agent配置已更新",
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
		a.logger.Info("Agent最大迭代次数已更新", zap.Int("max_iterations", maxIterations))
	}
}

// English note.
func (a *Agent) formatToolError(toolName string, args map[string]interface{}, err error) string {
	errorMsg := fmt.Sprintf(`工具执行失败

工具名称: %s
调用参数: %v
错误信息: %v

请分析错误原因并采取以下行动之一：
1. 如果参数错误，请修正参数后重试
2. 如果工具不可用，请尝试使用替代工具
3. 如果这是系统问题，请向用户说明情况并提供建议
4. 如果错误信息中包含有用信息，可以基于这些信息继续分析`, toolName, args, err)

	return errorMsg
}

// English note.
func (a *Agent) applyMemoryCompression(ctx context.Context, messages []ChatMessage, reservedTokens int) []ChatMessage {
	if a.memoryCompressor == nil {
		return messages
	}

	compressed, changed, err := a.memoryCompressor.CompressHistory(ctx, messages, reservedTokens)
	if err != nil {
		a.logger.Warn("上下文压缩失败，将使用原始消息继续", zap.Error(err))
		return messages
	}
	if changed {
		a.logger.Info("历史上下文已压缩",
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
					a.logger.Info("移除了未完成的tool_calls，避免重新执行",
						zap.Int("removed_count", originalCount-len(validToolCalls)),
					)
				}
				break
			}
		}
	}

	if removed {
		a.logger.Warn("修复了对话历史中的tool消息和tool_calls",
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
