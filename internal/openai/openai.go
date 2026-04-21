package openai

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

// English note.
type Client struct {
	httpClient *http.Client
	config     *config.OpenAIConfig
	logger     *zap.Logger
}

// English note.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openai api error: status=%d body=%s", e.StatusCode, e.Body)
}

// English note.
func NewClient(cfg *config.OpenAIConfig, httpClient *http.Client, logger *zap.Logger) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Client{
		httpClient: httpClient,
		config:     cfg,
		logger:     logger,
	}
}

// English note.
func (c *Client) UpdateConfig(cfg *config.OpenAIConfig) {
	c.config = cfg
}

// English note.
func (c *Client) ChatCompletion(ctx context.Context, payload interface{}, out interface{}) error {
	if c == nil {
		return fmt.Errorf("openai client is not initialized")
	}
	if c.config == nil {
		return fmt.Errorf("openai config is nil")
	}
	if ProviderRequiresAPIKey(c.config.Provider) && strings.TrimSpace(c.config.APIKey) == "" {
		return fmt.Errorf("openai api key is empty")
	}
	if c.isClaude() {
		return c.claudeChatCompletion(ctx, payload, out)
	}

	baseURL := ResolveBaseURL(c.config)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openai payload: %w", err)
	}

	c.logger.Debug("sending OpenAI chat completion request",
		zap.Int("payloadSizeKB", len(body)/1024))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build openai request: %w", err)
	}
	ApplyOpenAICompatibleHeaders(req, c.config)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call openai api: %w", err)
	}
	defer resp.Body.Close()

	bodyChan := make(chan []byte, 1)
	errChan := make(chan error, 1)
	go func() {
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}
		bodyChan <- responseBody
	}()

	var respBody []byte
	select {
	case respBody = <-bodyChan:
	case err := <-errChan:
		return fmt.Errorf("read openai response: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("read openai response timeout: %w", ctx.Err())
	case <-time.After(25 * time.Minute):
		return fmt.Errorf("read openai response timeout (25m)")
	}

	c.logger.Debug("received OpenAI response",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("responseSizeKB", len(respBody)/1024),
	)

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("OpenAI chat completion returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			c.logger.Error("failed to unmarshal OpenAI response",
				zap.Error(err),
				zap.String("body", string(respBody)),
			)
			return fmt.Errorf("unmarshal openai response: %w", err)
		}
	}

	return nil
}

// English note.
// English note.
func (c *Client) ChatCompletionStream(ctx context.Context, payload interface{}, onDelta func(delta string) error) (string, error) {
	if c == nil {
		return "", fmt.Errorf("openai client is not initialized")
	}
	if c.config == nil {
		return "", fmt.Errorf("openai config is nil")
	}
	if ProviderRequiresAPIKey(c.config.Provider) && strings.TrimSpace(c.config.APIKey) == "" {
		return "", fmt.Errorf("openai api key is empty")
	}
	if c.isClaude() {
		return c.claudeChatCompletionStream(ctx, payload, onDelta)
	}

	baseURL := ResolveBaseURL(c.config)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal openai payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build openai request: %w", err)
	}
	ApplyOpenAICompatibleHeaders(req, c.config)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call openai api: %w", err)
	}
	defer resp.Body.Close()

	// English note.
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	type streamDelta struct {
		// English note.
		Content string `json:"content,omitempty"`
		Text    string `json:"text,omitempty"`
	}
	type streamChoice struct {
		Delta        streamDelta `json:"delta"`
		FinishReason *string     `json:"finish_reason,omitempty"`
	}
	type streamResponse struct {
		ID      string         `json:"id,omitempty"`
		Choices []streamChoice `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error,omitempty"`
	}

	reader := bufio.NewReader(resp.Body)
	var full strings.Builder

	// English note.
	// data: {...}\n\n
	// data: [DONE]\n\n
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return full.String(), fmt.Errorf("read openai stream: %w", readErr)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		var chunk streamResponse
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			// English note.
			continue
		}
		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return full.String(), fmt.Errorf("openai stream error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			delta = chunk.Choices[0].Delta.Text
		}
		if delta == "" {
			continue
		}

		full.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return full.String(), err
			}
		}
	}

	c.logger.Debug("received OpenAI stream completion",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("contentLen", full.Len()),
	)

	return full.String(), nil
}

// English note.
type StreamToolCall struct {
	Index            int
	ID               string
	Type             string
	FunctionName    string
	FunctionArgsStr string
}

// English note.
func (c *Client) ChatCompletionStreamWithToolCalls(
	ctx context.Context,
	payload interface{},
	onContentDelta func(delta string) error,
) (string, []StreamToolCall, string, error) {
	if c == nil {
		return "", nil, "", fmt.Errorf("openai client is not initialized")
	}
	if c.config == nil {
		return "", nil, "", fmt.Errorf("openai config is nil")
	}
	if ProviderRequiresAPIKey(c.config.Provider) && strings.TrimSpace(c.config.APIKey) == "" {
		return "", nil, "", fmt.Errorf("openai api key is empty")
	}
	if c.isClaude() {
		return c.claudeChatCompletionStreamWithToolCalls(ctx, payload, onContentDelta)
	}

	baseURL := ResolveBaseURL(c.config)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, "", fmt.Errorf("marshal openai payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", nil, "", fmt.Errorf("build openai request: %w", err)
	}
	ApplyOpenAICompatibleHeaders(req, c.config)

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("call openai api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// English note.
	type toolCallFunctionDelta struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	}
	type toolCallDelta struct {
		Index    int                     `json:"index,omitempty"`
		ID       string                  `json:"id,omitempty"`
		Type     string                  `json:"type,omitempty"`
		Function toolCallFunctionDelta  `json:"function,omitempty"`
	}
	type streamDelta2 struct {
		Content   string          `json:"content,omitempty"`
		Text      string          `json:"text,omitempty"`
		ToolCalls []toolCallDelta `json:"tool_calls,omitempty"`
	}
	type streamChoice2 struct {
		Delta        streamDelta2 `json:"delta"`
		FinishReason *string      `json:"finish_reason,omitempty"`
	}
	type streamResponse2 struct {
		Choices []streamChoice2 `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error,omitempty"`
	}

	type toolCallAccum struct {
		id    string
		typ   string
		name  string
		args  strings.Builder
	}
	toolCallAccums := make(map[int]*toolCallAccum)

	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	finishReason := ""

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return full.String(), nil, finishReason, fmt.Errorf("read openai stream: %w", readErr)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		var chunk streamResponse2
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			// English note.
			continue
		}
		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return full.String(), nil, finishReason, fmt.Errorf("openai stream error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			finishReason = strings.TrimSpace(*choice.FinishReason)
		}

		delta := choice.Delta

		content := delta.Content
		if content == "" {
			content = delta.Text
		}
		if content != "" {
			full.WriteString(content)
			if onContentDelta != nil {
				if err := onContentDelta(content); err != nil {
					return full.String(), nil, finishReason, err
				}
			}
		}

		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				acc, ok := toolCallAccums[tc.Index]
				if !ok {
					acc = &toolCallAccum{}
					toolCallAccums[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Type != "" {
					acc.typ = tc.Type
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)
				}
			}
		}
	}

	// English note.
	indices := make([]int, 0, len(toolCallAccums))
	for idx := range toolCallAccums {
		indices = append(indices, idx)
	}
	// English note.
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	toolCalls := make([]StreamToolCall, 0, len(indices))
	for _, idx := range indices {
		acc := toolCallAccums[idx]
		tc := StreamToolCall{
			Index:            idx,
			ID:               acc.id,
			Type:             acc.typ,
			FunctionName:    acc.name,
			FunctionArgsStr: acc.args.String(),
		}
		toolCalls = append(toolCalls, tc)
	}

	c.logger.Debug("received OpenAI stream completion (tool_calls)",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("contentLen", full.Len()),
		zap.Int("toolCalls", len(toolCalls)),
		zap.String("finishReason", finishReason),
	)

	if strings.TrimSpace(finishReason) == "" {
		finishReason = "stop"
	}

	return full.String(), toolCalls, finishReason, nil
}
