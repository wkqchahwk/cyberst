package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
)

type ollamaCloudRoundTripper struct {
	base   http.RoundTripper
	config *config.OpenAIConfig
}

func isOllamaCloudProvider(cfg *config.OpenAIConfig) bool {
	if cfg == nil {
		return false
	}
	return NormalizeProvider(cfg.Provider) == ProviderOllamaCloud
}

func (rt *ollamaCloudRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return rt.base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: read request body: %w", err)
	}
	_ = req.Body.Close()

	var oaiReq openAIChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: unmarshal request: %w", err)
	}

	ollamaReq := convertOpenAIToOllamaChat(oaiReq)
	// Return OpenAI-compatible SSE to Eino even when the upstream request asked
	// for streaming. A single completed chunk is enough for the OpenAI client
	// while keeping the Ollama direct API path simple and reliable.
	ollamaReq.Stream = false

	ollamaBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: marshal request: %w", err)
	}

	baseURL := normalizeOllamaCloudBaseURL(rt.config)
	newReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, baseURL+"/chat", bytes.NewReader(ollamaBody))
	if err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: build request: %w", err)
	}
	newReq.Header.Set("Content-Type", "application/json")
	if rt.config != nil && strings.TrimSpace(rt.config.APIKey) != "" {
		newReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(rt.config.APIKey))
	}

	resp, err := rt.base.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		converted := ollamaErrorToOpenAIError(respBody, resp.StatusCode)
		return &http.Response{
			StatusCode:    resp.StatusCode,
			Header:        http.Header{"Content-Type": []string{"application/json"}},
			Body:          io.NopCloser(bytes.NewReader(converted)),
			ContentLength: int64(len(converted)),
			Request:       req,
		}, nil
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: unmarshal response: %w", err)
	}

	if oaiReq.Stream {
		sse := ollamaChatToOpenAIStreamJSON(ollamaResp)
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:          io.NopCloser(bytes.NewReader(sse)),
			ContentLength: int64(len(sse)),
			Request:       req,
		}, nil
	}

	converted, err := json.Marshal(ollamaChatToOpenAIResponse(ollamaResp))
	if err != nil {
		return nil, fmt.Errorf("ollama cloud bridge: marshal response: %w", err)
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(converted)),
		ContentLength: int64(len(converted)),
		Request:       req,
	}, nil
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Tools       []json.RawMessage   `json:"tools,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Tools    []json.RawMessage   `json:"tools,omitempty"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type ollamaChatResponse struct {
	Model            string            `json:"model"`
	CreatedAt        string            `json:"created_at,omitempty"`
	Message          ollamaChatMessage `json:"message"`
	Done             bool              `json:"done"`
	DoneReason       string            `json:"done_reason,omitempty"`
	PromptEvalCount  int               `json:"prompt_eval_count,omitempty"`
	EvalCount        int               `json:"eval_count,omitempty"`
	TotalDuration    int64             `json:"total_duration,omitempty"`
	PromptEvalDur    int64             `json:"prompt_eval_duration,omitempty"`
	EvalDuration      int64             `json:"eval_duration,omitempty"`
}

func normalizeOllamaCloudBaseURL(cfg *config.OpenAIConfig) string {
	baseURL := ""
	if cfg != nil {
		baseURL = strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	}
	if baseURL == "" {
		return "https://ollama.com/api"
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return strings.TrimSuffix(baseURL, "/v1") + "/api"
	}
	if !strings.HasSuffix(baseURL, "/api") && strings.Contains(baseURL, "ollama.com") {
		return strings.TrimSuffix(baseURL, "/") + "/api"
	}
	return baseURL
}

func convertOpenAIToOllamaChat(req openAIChatRequest) ollamaChatRequest {
	out := ollamaChatRequest{
		Model:    req.Model,
		Messages: make([]ollamaChatMessage, 0, len(req.Messages)),
		Tools:    req.Tools,
		Stream:   req.Stream,
	}

	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "user"
		}
		outMsg := ollamaChatMessage{
			Role:    role,
			Content: openAIContentToText(msg.Content),
		}
		if len(msg.ToolCalls) > 0 {
			outMsg.ToolCalls = normalizeOllamaRequestToolCalls(msg.ToolCalls)
		}
		out.Messages = append(out.Messages, outMsg)
	}

	options := map[string]any{}
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		options["top_p"] = *req.TopP
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		options["num_predict"] = *req.MaxTokens
	}
	if len(options) > 0 {
		out.Options = options
	}
	return out
}

func normalizeOllamaRequestToolCalls(calls []ollamaToolCall) []ollamaToolCall {
	out := make([]ollamaToolCall, 0, len(calls))
	for _, call := range calls {
		if call.Function.Arguments == nil || len(call.Function.Arguments) == 0 {
			call.Function.Arguments = json.RawMessage(`{}`)
		} else {
			var encoded string
			if err := json.Unmarshal(call.Function.Arguments, &encoded); err == nil {
				encoded = strings.TrimSpace(encoded)
				if encoded == "" {
					encoded = "{}"
				}
				if !json.Valid([]byte(encoded)) {
					encoded = "{}"
				}
				call.Function.Arguments = json.RawMessage(encoded)
			}
		}
		out = append(out, call)
	}
	return out
}

func openAIContentToText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			if part.Type == "" || part.Type == "text" {
				b.WriteString(part.Text)
			}
		}
		return b.String()
	}
	return string(raw)
}

func ollamaChatToOpenAIResponse(resp ollamaChatResponse) map[string]any {
	created := time.Now().Unix()
	if resp.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, resp.CreatedAt); err == nil {
			created = t.Unix()
		}
	}

	message := map[string]any{
		"role":    "assistant",
		"content": resp.Message.Content,
	}
	if len(resp.Message.ToolCalls) > 0 {
		message["tool_calls"] = normalizeOpenAIToolCalls(resp.Message.ToolCalls)
	}

	finishReason := normalizeOllamaFinishReason(resp.DoneReason, len(resp.Message.ToolCalls) > 0)
	return map[string]any{
		"id":      "chatcmpl-ollama-" + strconv.FormatInt(created, 10),
		"object":  "chat.completion",
		"created": created,
		"model":   resp.Model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     resp.PromptEvalCount,
			"completion_tokens": resp.EvalCount,
			"total_tokens":      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}

func ollamaChatToOpenAIStreamJSON(resp ollamaChatResponse) []byte {
	created := time.Now().Unix()
	if resp.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, resp.CreatedAt); err == nil {
			created = t.Unix()
		}
	}
	id := "chatcmpl-ollama-" + strconv.FormatInt(created, 10)
	model := resp.Model
	finishReason := normalizeOllamaFinishReason(resp.DoneReason, len(resp.Message.ToolCalls) > 0)

	var b strings.Builder
	delta := map[string]any{
		"role":    "assistant",
		"content": resp.Message.Content,
	}
	if len(resp.Message.ToolCalls) > 0 {
		delta["tool_calls"] = normalizeOpenAIStreamToolCalls(resp.Message.ToolCalls)
	}
	writeOpenAISSEChunk(&b, id, model, created, delta, nil)
	writeOpenAISSEChunk(&b, id, model, created, map[string]any{}, finishReason)
	b.WriteString("data: [DONE]\n\n")
	return []byte(b.String())
}

func writeOpenAISSEChunk(b *strings.Builder, id, model string, created int64, delta map[string]any, finishReason any) {
	chunk := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         delta,
				"finish_reason": finishReason,
			},
		},
	}
	encoded, _ := json.Marshal(chunk)
	b.WriteString("data: ")
	b.Write(encoded)
	b.WriteString("\n\n")
}

func normalizeOpenAIToolCalls(calls []ollamaToolCall) []map[string]any {
	out := make([]map[string]any, 0, len(calls))
	for i, call := range calls {
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("call_ollama_%d", i)
		}
		args := "{}"
		if len(call.Function.Arguments) > 0 {
			args = string(call.Function.Arguments)
		}
		out = append(out, map[string]any{
			"id":   id,
			"type": "function",
			"function": map[string]any{
				"name":      call.Function.Name,
				"arguments": args,
			},
		})
	}
	return out
}

func normalizeOpenAIStreamToolCalls(calls []ollamaToolCall) []map[string]any {
	out := make([]map[string]any, 0, len(calls))
	for i, call := range calls {
		id := strings.TrimSpace(call.ID)
		if id == "" {
			id = fmt.Sprintf("call_ollama_%d", i)
		}
		args := "{}"
		if len(call.Function.Arguments) > 0 {
			args = string(call.Function.Arguments)
		}
		out = append(out, map[string]any{
			"index": i,
			"id":    id,
			"type":  "function",
			"function": map[string]any{
				"name":      call.Function.Name,
				"arguments": args,
			},
		})
	}
	return out
}

func normalizeOllamaFinishReason(reason string, hasToolCalls bool) string {
	if hasToolCalls {
		return "tool_calls"
	}
	switch strings.TrimSpace(strings.ToLower(reason)) {
	case "", "stop", "unload":
		return "stop"
	case "length":
		return "length"
	default:
		return reason
	}
}

func ollamaErrorToOpenAIError(body []byte, status int) []byte {
	msg := strings.TrimSpace(string(body))
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && strings.TrimSpace(parsed.Error) != "" {
		msg = parsed.Error
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	out, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "ollama_cloud_error",
			"code":    status,
		},
	})
	return out
}
