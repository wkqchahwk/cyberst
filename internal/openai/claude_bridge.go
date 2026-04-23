package openai

// English note.
// English note.
//
// English note.
//   Request:  OpenAI /chat/completions  → Claude /v1/messages
// English note.
// English note.
//   Auth:     Bearer → x-api-key
//   Tools:    OpenAI tools[] → Claude tools[] (input_schema)

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

// ============================================================
// Claude Request Types
// ============================================================

// English note.
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
	Tools     []claudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string               `json:"role"`
	Content claudeMessageContent `json:"content"`
}

// English note.
// English note.
type claudeMessageContent struct {
	Text   string               // （）
	Blocks []claudeContentBlock //  block （tool_use / tool_result ）
}

func (c claudeMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) > 0 {
		return json.Marshal(c.Blocks)
	}
	return json.Marshal(c.Text)
}

func (c *claudeMessageContent) UnmarshalJSON(data []byte) error {
	// English note.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	// English note.
	return json.Unmarshal(data, &c.Blocks)
}

type claudeContentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// English note.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// English note.
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type claudeTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ============================================================
// Claude Response Types
// ============================================================

type claudeResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []claudeContentBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   string               `json:"stop_reason"`
	StopSequence *string              `json:"stop_sequence"`
	Usage        *claudeUsage         `json:"usage,omitempty"`
	Error        *claudeError         `json:"error,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ============================================================
// Conversion: OpenAI Request → Claude Request
// ============================================================

// English note.
func convertOpenAIToClaude(payload interface{}) (*claudeRequest, error) {
	// English note.
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("claude bridge: marshal payload: %w", err)
	}

	var oai map[string]interface{}
	if err := json.Unmarshal(raw, &oai); err != nil {
		return nil, fmt.Errorf("claude bridge: unmarshal payload: %w", err)
	}

	req := &claudeRequest{}

	// model
	if m, ok := oai["model"].(string); ok {
		req.Model = m
	}

	// English note.
	if mt, ok := oai["max_tokens"].(float64); ok && mt > 0 {
		req.MaxTokens = int(mt)
	} else {
		req.MaxTokens = 8192 // Claude （ Haiku/Sonnet/Opus）
	}

	// stream
	if s, ok := oai["stream"].(bool); ok {
		req.Stream = s
	}

	// messages
	msgs, _ := oai["messages"].([]interface{})
	for i := 0; i < len(msgs); i++ {
		mm, ok := msgs[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := mm["role"].(string)
		content, _ := mm["content"].(string)

		// English note.
		if role == "system" {
			if req.System != "" {
				req.System += "\n\n"
			}
			req.System += content
			continue
		}

		// English note.
		if role == "assistant" {
			var blocks []claudeContentBlock
			if content != "" {
				blocks = append(blocks, claudeContentBlock{Type: "text", Text: content})
			}

			if tcs, ok := mm["tool_calls"].([]interface{}); ok {
				for _, tc := range tcs {
					tcMap, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					tcID, _ := tcMap["id"].(string)
					fn, _ := tcMap["function"].(map[string]interface{})
					fnName, _ := fn["name"].(string)
					fnArgs, _ := fn["arguments"]

						// English note.
						if strings.TrimSpace(fnName) == "" {
							fnName = "unknown_function"
						}
						if strings.TrimSpace(tcID) == "" {
							tcID = fmt.Sprintf("call_%d", time.Now().UnixNano())
						}

					var inputRaw json.RawMessage
					switch v := fnArgs.(type) {
					case string:
						inputRaw = json.RawMessage(v)
					default:
						inputRaw, _ = json.Marshal(v)
					}
					// English note.
					if len(inputRaw) == 0 || !json.Valid(inputRaw) {
						inputRaw = json.RawMessage("{}")
					}
					blocks = append(blocks, claudeContentBlock{
						Type:  "tool_use",
						ID:    tcID,
						Name:  fnName,
						Input: inputRaw,
					})
				}
			}

			if len(blocks) > 0 {
				req.Messages = append(req.Messages, claudeMessage{
					Role:    "assistant",
					Content: claudeMessageContent{Blocks: blocks},
				})
			}
			continue
		}

		// tool result (role == "tool" in OpenAI)
		// English note.
		// English note.
		if role == "tool" {
			var toolBlocks []claudeContentBlock
			// English note.
			for ; i < len(msgs); i++ {
				tmm, ok := msgs[i].(map[string]interface{})
				if !ok {
					break
				}
				tr, _ := tmm["role"].(string)
				if tr != "tool" {
					break
				}
				tcID, _ := tmm["tool_call_id"].(string)
				tcContent, _ := tmm["content"].(string)
				toolBlocks = append(toolBlocks, claudeContentBlock{
					Type:      "tool_result",
					ToolUseID: tcID,
					Content:   tcContent,
				})
			}
			i-- //  for  i++，
			req.Messages = append(req.Messages, claudeMessage{
				Role:    "user",
				Content: claudeMessageContent{Blocks: toolBlocks},
			})
			continue
		}

		// English note.
		req.Messages = append(req.Messages, claudeMessage{
			Role:    role,
			Content: claudeMessageContent{Text: content},
		})
	}

	// tools
	if tools, ok := oai["tools"].([]interface{}); ok {
		for _, t := range tools {
			tMap, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			fn, ok := tMap["function"].(map[string]interface{})
			if !ok {
				continue
			}
			ct := claudeTool{}
			ct.Name, _ = fn["name"].(string)
			ct.Description, _ = fn["description"].(string)
			if params, ok := fn["parameters"].(map[string]interface{}); ok {
				ct.InputSchema = params
			} else {
				ct.InputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			req.Tools = append(req.Tools, ct)
		}
	}

	return req, nil
}

// ============================================================
// Conversion: Claude Response → OpenAI Response (non-streaming)
// ============================================================

// English note.
func claudeToOpenAIResponseJSON(claudeBody []byte) ([]byte, error) {
	var cr claudeResponse
	if err := json.Unmarshal(claudeBody, &cr); err != nil {
		return nil, fmt.Errorf("claude bridge: unmarshal response: %w", err)
	}

	if cr.Error != nil {
		return nil, fmt.Errorf("claude api error: [%s] %s", cr.Error.Type, cr.Error.Message)
	}

	// English note.
	oaiResp := map[string]interface{}{
		"id":      cr.ID,
		"object":  "chat.completion",
		"model":   cr.Model,
		"choices": []interface{}{},
	}

	var textContent string
	var toolCalls []interface{}

	for _, block := range cr.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			argsStr := string(block.Input)
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   block.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      block.Name,
					"arguments": argsStr,
				},
			})
		}
	}

	finishReason := claudeStopReasonToOpenAI(cr.StopReason)
	message := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	choice := map[string]interface{}{
		"index":         0,
		"message":       message,
		"finish_reason": finishReason,
	}

	oaiResp["choices"] = []interface{}{choice}

	if cr.Usage != nil {
		oaiResp["usage"] = map[string]interface{}{
			"prompt_tokens":     cr.Usage.InputTokens,
			"completion_tokens": cr.Usage.OutputTokens,
			"total_tokens":      cr.Usage.InputTokens + cr.Usage.OutputTokens,
		}
	}

	return json.Marshal(oaiResp)
}

func claudeStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// ============================================================
// Claude HTTP Calls (non-streaming & streaming)
// ============================================================

// English note.
func (c *Client) claudeChatCompletion(ctx context.Context, payload interface{}, out interface{}) error {
	claudeReq, err := convertOpenAIToClaude(payload)
	if err != nil {
		return err
	}
	claudeReq.Stream = false

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return fmt.Errorf("claude bridge: marshal: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	c.logger.Debug("sending Claude chat completion request",
		zap.String("model", claudeReq.Model),
		zap.Int("payloadSizeKB", len(body)/1024))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("claude bridge: build request: %w", err)
	}
	c.setClaudeHeaders(req)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("claude bridge: call api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("claude bridge: read response: %w", err)
	}

	c.logger.Debug("received Claude response",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("responseSizeKB", len(respBody)/1024),
	)

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Claude chat completion returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// English note.
	oaiJSON, err := claudeToOpenAIResponseJSON(respBody)
	if err != nil {
		return err
	}

	if out != nil {
		if err := json.Unmarshal(oaiJSON, out); err != nil {
			return fmt.Errorf("claude bridge: unmarshal converted response: %w", err)
		}
	}

	return nil
}

// English note.
func (c *Client) claudeChatCompletionStream(ctx context.Context, payload interface{}, onDelta func(delta string) error) (string, error) {
	claudeReq, err := convertOpenAIToClaude(payload)
	if err != nil {
		return "", err
	}
	claudeReq.Stream = true

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return "", fmt.Errorf("claude bridge: marshal: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("claude bridge: build request: %w", err)
	}
	c.setClaudeHeaders(req)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude bridge: call api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	reader := bufio.NewReader(resp.Body)
	var full strings.Builder

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return full.String(), fmt.Errorf("claude bridge: read stream: %w", readErr)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "content_block_delta":
			delta, _ := event["delta"].(map[string]interface{})
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				if text != "" {
					full.WriteString(text)
					if onDelta != nil {
						if err := onDelta(text); err != nil {
							return full.String(), err
						}
					}
				}
			}
		case "error":
			errData, _ := event["error"].(map[string]interface{})
			msg, _ := errData["message"].(string)
			return full.String(), fmt.Errorf("claude stream error: %s", msg)
		}
	}

	c.logger.Debug("received Claude stream completion",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("contentLen", full.Len()),
	)

	return full.String(), nil
}

// English note.
// English note.
func (c *Client) claudeChatCompletionStreamWithToolCalls(
	ctx context.Context,
	payload interface{},
	onContentDelta func(delta string) error,
) (string, []StreamToolCall, string, error) {
	claudeReq, err := convertOpenAIToClaude(payload)
	if err != nil {
		return "", nil, "", err
	}
	claudeReq.Stream = true

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return "", nil, "", fmt.Errorf("claude bridge: marshal: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", nil, "", fmt.Errorf("claude bridge: build request: %w", err)
	}
	c.setClaudeHeaders(req)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("claude bridge: call api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	finishReason := ""

	// English note.
	type toolAccum struct {
		id    string
		name  string
		args  strings.Builder
		index int
	}
	var currentToolCalls []toolAccum
	currentBlockIndex := -1
	currentBlockType := ""

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return full.String(), nil, finishReason, fmt.Errorf("claude bridge: read stream: %w", readErr)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "content_block_start":
			idx, _ := event["index"].(float64)
			currentBlockIndex = int(idx)
			cb, _ := event["content_block"].(map[string]interface{})
			blockType, _ := cb["type"].(string)
			currentBlockType = blockType

			if blockType == "tool_use" {
				id, _ := cb["id"].(string)
				name, _ := cb["name"].(string)
				currentToolCalls = append(currentToolCalls, toolAccum{
					id:    id,
					name:  name,
					index: currentBlockIndex,
				})
			}

		case "content_block_delta":
			delta, _ := event["delta"].(map[string]interface{})
			deltaType, _ := delta["type"].(string)

			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				if text != "" {
					full.WriteString(text)
					if onContentDelta != nil {
						if err := onContentDelta(text); err != nil {
							return full.String(), nil, finishReason, err
						}
					}
				}
			} else if deltaType == "input_json_delta" {
				partialJSON, _ := delta["partial_json"].(string)
				if partialJSON != "" && currentBlockType == "tool_use" && len(currentToolCalls) > 0 {
					currentToolCalls[len(currentToolCalls)-1].args.WriteString(partialJSON)
				}
			}

		case "content_block_stop":
			// English note.

		case "message_delta":
			delta, _ := event["delta"].(map[string]interface{})
			if sr, ok := delta["stop_reason"].(string); ok {
				finishReason = claudeStopReasonToOpenAI(sr)
			}

		case "message_stop":
			// English note.

		case "error":
			errData, _ := event["error"].(map[string]interface{})
			msg, _ := errData["message"].(string)
			return full.String(), nil, finishReason, fmt.Errorf("claude stream error: %s", msg)
		}
	}

	// English note.
	var toolCalls []StreamToolCall
	for i, tc := range currentToolCalls {
		toolCalls = append(toolCalls, StreamToolCall{
			Index:           i,
			ID:              tc.id,
			Type:            "function",
			FunctionName:    tc.name,
			FunctionArgsStr: tc.args.String(),
		})
	}

	if finishReason == "" {
		finishReason = "stop"
	}

	c.logger.Debug("received Claude stream completion (tool_calls)",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("contentLen", full.Len()),
		zap.Int("toolCalls", len(toolCalls)),
		zap.String("finishReason", finishReason),
	)

	return full.String(), toolCalls, finishReason, nil
}

// ============================================================
// Helpers
// ============================================================

// English note.
func (c *Client) setClaudeHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

// English note.
func (c *Client) isClaude() bool {
	return isClaudeProvider(c.config)
}

func isClaudeProvider(cfg *config.OpenAIConfig) bool {
	if cfg == nil {
		return false
	}
	return NormalizeProvider(cfg.Provider) == ProviderAnthropic
}

// ============================================================
// Eino HTTP Client Bridge
// ============================================================

// English note.
// English note.
func NewEinoHTTPClient(cfg *config.OpenAIConfig, base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	if isOllamaCloudProvider(cfg) {
		cloned := *base
		transport := base.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		cloned.Transport = &ollamaCloudRoundTripper{
			base:   transport,
			config: cfg,
		}
		return &cloned
	}
	if !isClaudeProvider(cfg) {
		return base
	}

	cloned := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	cloned.Transport = &claudeRoundTripper{
		base:   transport,
		config: cfg,
	}
	return &cloned
}

// English note.
type claudeRoundTripper struct {
	base   http.RoundTripper
	config *config.OpenAIConfig
}

func (rt *claudeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// English note.
	if !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return rt.base.RoundTrip(req)
	}

	// English note.
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("claude bridge: read request body: %w", err)
	}
	_ = req.Body.Close()

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("claude bridge: unmarshal request: %w", err)
	}

	// English note.
	claudeReq, err := convertOpenAIToClaude(payload)
	if err != nil {
		return nil, err
	}

	// English note.
	baseURL := strings.TrimSuffix(rt.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	claudeBody, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude bridge: marshal claude request: %w", err)
	}

	newReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(claudeBody))
	if err != nil {
		return nil, fmt.Errorf("claude bridge: build request: %w", err)
	}
	newReq.Header.Set("Content-Type", "application/json")
	newReq.Header.Set("x-api-key", rt.config.APIKey)
	newReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := rt.base.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}

	// English note.
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		converted := rt.tryConvertClaudeErrorToOpenAI(bodyBytes)
		return &http.Response{
			StatusCode:    resp.StatusCode,
			Header:        resp.Header.Clone(),
			Body:          io.NopCloser(bytes.NewReader(converted)),
			ContentLength: int64(len(converted)),
			Request:       req,
		}, nil
	}

	// English note.
	if !claudeReq.Stream {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		oaiJSON, err := claudeToOpenAIResponseJSON(respBody)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"application/json"}},
			Body:          io.NopCloser(bytes.NewReader(oaiJSON)),
			ContentLength: int64(len(oaiJSON)),
			Request:       req,
		}, nil
	}

	// English note.
	pr, pw := io.Pipe()

	// English note.
	writeLine := func(data string) bool {
		_, err := pw.Write([]byte(data))
		return err == nil
	}

	go func() {
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		blockToToolIndex := make(map[int]int)
		nextToolIndex := 0

		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				if readErr == io.EOF {
					writeLine("data: [DONE]\n\n")
				} else {
					// English note.
					oaiErr := map[string]interface{}{
						"error": map[string]interface{}{
							"message": readErr.Error(),
							"type":    "claude_stream_read_error",
						},
					}
					b, _ := json.Marshal(oaiErr)
					writeLine("data: " + string(b) + "\n\n")
					writeLine("data: [DONE]\n\n")
				}
				pw.Close()
				return
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
				continue
			}
			dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if dataStr == "[DONE]" {
				writeLine("data: [DONE]\n\n")
				pw.Close()
				return
			}

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)

			switch eventType {
			case "content_block_start":
				blockIdxFlt, _ := event["index"].(float64)
				blockIdx := int(blockIdxFlt)
				cb, _ := event["content_block"].(map[string]interface{})
				bt, _ := cb["type"].(string)

				if bt == "tool_use" {
					id, _ := cb["id"].(string)
					name, _ := cb["name"].(string)
					blockToToolIndex[blockIdx] = nextToolIndex
					toolIdx := nextToolIndex
					nextToolIndex++

					oaiChunk := map[string]interface{}{
						"choices": []map[string]interface{}{
							{
								"delta": map[string]interface{}{
									"tool_calls": []map[string]interface{}{
										{
											"index": toolIdx,
											"id":    id,
											"type":  "function",
											"function": map[string]interface{}{
												"name": name,
											},
										},
									},
								},
							},
						},
					}
					b, _ := json.Marshal(oaiChunk)
					if !writeLine("data: " + string(b) + "\n\n") {
						pw.Close()
						return
					}
				}

			case "content_block_delta":
				blockIdxFlt, _ := event["index"].(float64)
				blockIdx := int(blockIdxFlt)
				delta, _ := event["delta"].(map[string]interface{})
				dt, _ := delta["type"].(string)

				if dt == "text_delta" {
					text, _ := delta["text"].(string)
					oaiChunk := map[string]interface{}{
						"choices": []map[string]interface{}{
							{
								"delta": map[string]interface{}{
									"content": text,
								},
							},
						},
					}
					b, _ := json.Marshal(oaiChunk)
					if !writeLine("data: " + string(b) + "\n\n") {
						pw.Close()
						return
					}
				} else if dt == "input_json_delta" {
					partial, _ := delta["partial_json"].(string)
					if partial != "" {
						if toolIdx, ok := blockToToolIndex[blockIdx]; ok {
							oaiChunk := map[string]interface{}{
								"choices": []map[string]interface{}{
									{
										"delta": map[string]interface{}{
											"tool_calls": []map[string]interface{}{
												{
													"index": toolIdx,
													"function": map[string]interface{}{
														"arguments": partial,
													},
												},
											},
										},
									},
								},
							}
							b, _ := json.Marshal(oaiChunk)
							if !writeLine("data: " + string(b) + "\n\n") {
								pw.Close()
								return
							}
						}
					}
				}

			case "message_delta":
				d, _ := event["delta"].(map[string]interface{})
				if sr, ok := d["stop_reason"].(string); ok {
					finishReason := claudeStopReasonToOpenAI(sr)
					oaiChunk := map[string]interface{}{
						"choices": []map[string]interface{}{
							{
								"delta":         map[string]interface{}{},
								"finish_reason": finishReason,
							},
						},
					}
					b, _ := json.Marshal(oaiChunk)
					if !writeLine("data: " + string(b) + "\n\n") {
						pw.Close()
						return
					}
				}

			case "message_stop":
				writeLine("data: [DONE]\n\n")
				pw.Close()
				return

			case "error":
				errData, _ := event["error"].(map[string]interface{})
				msg, _ := errData["message"].(string)
				oaiChunk := map[string]interface{}{
					"error": map[string]interface{}{
						"message": msg,
						"type":    "claude_stream_error",
					},
				}
				b, _ := json.Marshal(oaiChunk)
				writeLine("data: " + string(b) + "\n\n")
				writeLine("data: [DONE]\n\n")
				pw.Close()
				return
			}
		}
	}()

	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body:    pr,
		Request: req,
	}, nil
}

// English note.
func (rt *claudeRoundTripper) tryConvertClaudeErrorToOpenAI(body []byte) []byte {
	var ce struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &ce); err != nil || ce.Error.Message == "" {
		return body
	}
	oaiErr := map[string]interface{}{
		"error": map[string]interface{}{
			"message": ce.Error.Message,
			"type":    ce.Error.Type,
			"code":    ce.Type,
		},
	}
	b, _ := json.Marshal(oaiErr)
	return b
}
