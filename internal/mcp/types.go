package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// English note.
type ExternalMCPClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
	Close() error
	IsConnected() bool
	GetStatus() string
}

// English note.
const (
	MessageTypeRequest  = "request"
	MessageTypeResponse = "response"
	MessageTypeError    = "error"
	MessageTypeNotify   = "notify"
)

// English note.
const ProtocolVersion = "2024-11-05"

// English note.
type MessageID struct {
	value interface{}
}

// English note.
func (m *MessageID) UnmarshalJSON(data []byte) error {
	// English note.
	if string(data) == "null" {
		m.value = nil
		return nil
	}

	// English note.
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		m.value = str
		return nil
	}

	// English note.
	var num json.Number
	if err := json.Unmarshal(data, &num); err == nil {
		m.value = num
		return nil
	}

	return fmt.Errorf("invalid id type")
}

// English note.
func (m MessageID) MarshalJSON() ([]byte, error) {
	if m.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(m.value)
}

// English note.
func (m MessageID) String() string {
	if m.value == nil {
		return ""
	}
	return fmt.Sprintf("%v", m.value)
}

// English note.
func (m MessageID) Value() interface{} {
	return m.value
}

// English note.
type Message struct {
	ID      MessageID       `json:"id,omitempty"`
	Type    string          `json:"-"` // 内部使用，不序列化到JSON
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	Version string          `json:"jsonrpc,omitempty"` // JSON-RPC 2.0 版本标识
}

// English note.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// English note.
type Tool struct {
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`                // 详细描述
	ShortDescription string                 `json:"shortDescription,omitempty"` // 简短描述（用于工具列表，减少token消耗）
	InputSchema      map[string]interface{} `json:"inputSchema"`
}

// English note.
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// English note.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// English note.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// English note.
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// English note.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// English note.
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// English note.
type ServerCapabilities struct {
	Tools     map[string]interface{} `json:"tools,omitempty"`
	Prompts   map[string]interface{} `json:"prompts,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty"`
	Sampling  map[string]interface{} `json:"sampling,omitempty"`
}

// English note.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// English note.
type ListToolsRequest struct{}

// English note.
type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

// English note.
type ListPromptsResponse struct {
	Prompts []Prompt `json:"prompts"`
}

// English note.
type ListResourcesResponse struct {
	Resources []Resource `json:"resources"`
}

// English note.
type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// English note.
type CallToolResponse struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// English note.
type ToolExecution struct {
	ID        string                 `json:"id"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
	Status    string                 `json:"status"` // pending, running, completed, failed
	Result    *ToolResult            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"startTime"`
	EndTime   *time.Time             `json:"endTime,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
}

// English note.
type ToolStats struct {
	ToolName     string     `json:"toolName"`
	TotalCalls   int        `json:"totalCalls"`
	SuccessCalls int        `json:"successCalls"`
	FailedCalls  int        `json:"failedCalls"`
	LastCallTime *time.Time `json:"lastCallTime,omitempty"`
}

// English note.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// English note.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// English note.
type GetPromptRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// English note.
type GetPromptResponse struct {
	Messages []PromptMessage `json:"messages"`
}

// English note.
type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// English note.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// English note.
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// English note.
type ReadResourceResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// English note.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// English note.
type SamplingRequest struct {
	Messages    []SamplingMessage `json:"messages"`
	Model       string            `json:"model,omitempty"`
	MaxTokens   int               `json:"maxTokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	TopP        float64           `json:"topP,omitempty"`
}

// English note.
type SamplingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// English note.
type SamplingResponse struct {
	Content    []SamplingContent `json:"content"`
	Model      string            `json:"model,omitempty"`
	StopReason string            `json:"stopReason,omitempty"`
}

// English note.
type SamplingContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
