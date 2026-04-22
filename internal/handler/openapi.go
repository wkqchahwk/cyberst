package handler

import (
	"net/http"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// English note.
type OpenAPIHandler struct {
	db               *database.DB
	logger           *zap.Logger
	resultStorage    storage.ResultStorage
	conversationHdlr *ConversationHandler
	agentHdlr        *AgentHandler
}

// English note.
func NewOpenAPIHandler(db *database.DB, logger *zap.Logger, resultStorage storage.ResultStorage, conversationHdlr *ConversationHandler, agentHdlr *AgentHandler) *OpenAPIHandler {
	return &OpenAPIHandler{
		db:               db,
		logger:           logger,
		resultStorage:    resultStorage,
		conversationHdlr: conversationHdlr,
		agentHdlr:        agentHdlr,
	}
}

// English note.
func (h *OpenAPIHandler) GetOpenAPISpec(c *gin.Context) {
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "CyberStrikeAI API",
			"description": "AIAPI",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"name": "CyberStrikeAI",
			},
		},
		"servers": []map[string]interface{}{
			{
				"url":         scheme + "://" + host,
				"description": "",
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "Bearer Token。Token /api/auth/login 。",
				},
			},
			"schemas": map[string]interface{}{
				"CreateConversationRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
							"example":     "Web",
						},
					},
				},
				"Conversation": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
							"example":     "550e8400-e29b-41d4-a716-446655440000",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
							"example":     "Web",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"ConversationDetail": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "：active（）、completed（）、failed（）",
							"enum":        []string{"active", "completed", "failed"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"messageCount": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
					},
				},
				"Message": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "：user（）、assistant（）",
							"enum":        []string{"user", "assistant"},
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"ConversationResults": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"messages": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Message",
							},
						},
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"executionResults": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/ExecutionResult",
							},
						},
					},
				},
				"Vulnerability": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"ExecutionResult": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"success", "failed", "running"},
						},
						"result": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"Error": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"LoginRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"password"},
					"properties": map[string]interface{}{
						"password": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"LoginResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"token": map[string]interface{}{
							"type":        "string",
							"description": "Token",
						},
						"expires_at": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "Token",
						},
						"session_duration_hr": map[string]interface{}{
							"type":        "integer",
							"description": "（）",
						},
					},
				},
				"ChangePasswordRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"oldPassword", "newPassword"},
					"properties": map[string]interface{}{
						"oldPassword": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"newPassword": map[string]interface{}{
							"type":        "string",
							"description": "（8）",
						},
					},
				},
				"UpdateConversationRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"title"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"Group": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
						"updatedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"CreateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "（）",
						},
					},
				},
				"UpdateGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"icon": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"AddConversationToGroupRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId", "groupId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"groupId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
					},
				},
				"BatchTaskRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"tasks"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "（）",
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "，",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"role": map[string]interface{}{
							"type":        "string",
							"description": "（）",
						},
						"agentMode": map[string]interface{}{
							"type":        "string",
							"description": "：single（ ReAct）| eino_single（Eino ADK ）| deep | plan_execute | supervisor；react  single； multi  deep",
							"enum":        []string{"single", "eino_single", "deep", "plan_execute", "supervisor", "multi", "react"},
						},
						"scheduleMode": map[string]interface{}{
							"type":        "string",
							"description": "（manual | cron）",
							"enum":        []string{"manual", "cron"},
						},
						"cronExpr": map[string]interface{}{
							"type":        "string",
							"description": "Cron （scheduleMode=cron ）",
						},
						"executeNow": map[string]interface{}{
							"type":        "boolean",
							"description": "（ false）",
						},
					},
				},
				"BatchQueue": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"pending", "running", "paused", "completed", "failed"},
						},
						"tasks": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"type": "object",
							},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"CancelAgentLoopRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversationId"},
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
					},
				},
				"AgentTask": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"conversationId": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"running", "completed", "failed", "cancelled", "timeout"},
						},
						"startedAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"CreateVulnerabilityRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"conversation_id", "title", "severity"},
					"properties": map[string]interface{}{
						"conversation_id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"UpdateVulnerabilityRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"severity": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"critical", "high", "medium", "low", "info"},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"open", "closed", "fixed"},
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"proof": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"impact": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"recommendation": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"ListVulnerabilitiesResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Vulnerability",
							},
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
					},
				},
				"VulnerabilityStats": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"by_severity": map[string]interface{}{
							"type":        "object",
							"description": "",
						},
						"by_status": map[string]interface{}{
							"type":        "object",
							"description": "",
						},
					},
				},
				"RoleConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "",
						},
						"systemPrompt": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"userPrompt": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"tools": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"skills": map[string]interface{}{
							"type":        "array",
							"description": "Skills",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"Skill": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
					},
				},
				"CreateSkillRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "description"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
					},
				},
				"UpdateSkillRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Skill",
						},
					},
				},
				"ToolExecution": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "ID",
						},
						"toolName": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"success", "failed", "running"},
						},
						"createdAt": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
					},
				},
				"MonitorResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"executions": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/ToolExecution",
							},
						},
						"stats": map[string]interface{}{
							"type":        "object",
							"description": "",
						},
						"timestamp": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "",
						},
						"total": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"page": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"page_size": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"total_pages": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
					},
				},
				"ConfigResponse": map[string]interface{}{
					"type":        "object",
					"description": "",
				},
				"UpdateConfigRequest": map[string]interface{}{
					"type":        "object",
					"description": "",
				},
				"ExternalMCPConfig": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
						"args": map[string]interface{}{
							"type":        "array",
							"description": "",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"ExternalMCPResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"config": map[string]interface{}{
							"$ref": "#/components/schemas/ExternalMCPConfig",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "",
							"enum":        []string{"connected", "disconnected", "error", "disabled"},
						},
						"toolCount": map[string]interface{}{
							"type":        "integer",
							"description": "",
						},
						"error": map[string]interface{}{
							"type":        "string",
							"description": "",
						},
					},
				},
				"AddOrUpdateExternalMCPRequest": map[string]interface{}{
					"type":     "object",
					"required": []string{"config"},
					"properties": map[string]interface{}{
						"config": map[string]interface{}{
							"$ref": "#/components/schemas/ExternalMCPConfig",
						},
					},
				},
				"AttackChain": map[string]interface{}{
					"type":        "object",
					"description": "",
				},
				"MCPMessage": map[string]interface{}{
					"type":        "object",
					"description": "MCP（JSON-RPC 2.0）",
					"required":    []string{"jsonrpc"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"description": "ID，、null。，；，",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
							"example": "550e8400-e29b-41d4-a716-446655440000",
						},
						"method": map[string]interface{}{
							"type":        "string",
							"description": "。：\n- `initialize`: MCP\n- `tools/list`: \n- `tools/call`: \n- `prompts/list`: \n- `prompts/get`: \n- `resources/list`: \n- `resources/read`: \n- `sampling/request`: ",
							"enum": []string{
								"initialize",
								"tools/list",
								"tools/call",
								"prompts/list",
								"prompts/get",
								"resources/list",
								"resources/read",
								"sampling/request",
							},
							"example": "tools/list",
						},
						"params": map[string]interface{}{
							"description": "（JSON），method",
							"type":        "object",
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC，\"2.0\"",
							"enum":        []string{"2.0"},
							"example":     "2.0",
						},
					},
				},
				"MCPInitializeParams": map[string]interface{}{
					"type":     "object",
					"required": []string{"protocolVersion", "capabilities", "clientInfo"},
					"properties": map[string]interface{}{
						"protocolVersion": map[string]interface{}{
							"type":        "string",
							"description": "",
							"example":     "2024-11-05",
						},
						"capabilities": map[string]interface{}{
							"type":        "object",
							"description": "",
						},
						"clientInfo": map[string]interface{}{
							"type":     "object",
							"required": []string{"name", "version"},
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "",
									"example":     "MyClient",
								},
								"version": map[string]interface{}{
									"type":        "string",
									"description": "",
									"example":     "1.0.0",
								},
							},
						},
					},
				},
				"MCPCallToolParams": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "arguments"},
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "",
							"example":     "nmap",
						},
						"arguments": map[string]interface{}{
							"type":        "object",
							"description": "（），",
							"example": map[string]interface{}{
								"target": "192.168.1.1",
								"ports":  "80,443",
							},
						},
					},
				},
				"MCPResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"description": "ID（id）",
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "number"},
								{"type": "null"},
							},
						},
						"result": map[string]interface{}{
							"description": "（JSON），",
							"type":        "object",
						},
						"error": map[string]interface{}{
							"type":        "object",
							"description": "（）",
							"properties": map[string]interface{}{
								"code": map[string]interface{}{
									"type":        "integer",
									"description": "",
									"example":     -32600,
								},
								"message": map[string]interface{}{
									"type":        "string",
									"description": "",
									"example":     "Invalid Request",
								},
								"data": map[string]interface{}{
									"description": "（）",
								},
							},
						},
						"jsonrpc": map[string]interface{}{
							"type":        "string",
							"description": "JSON-RPC",
							"example":     "2.0",
						},
					},
				},
			},
		},
		"security": []map[string]interface{}{
			{
				"bearerAuth": []string{},
			},
		},
		"paths": map[string]interface{}{
			"/api/auth/login": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Token",
					"operationId": "login",
					"security":    []map[string]interface{}{},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/LoginRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/LoginResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/auth/logout": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，Token",
					"operationId": "logout",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/auth/change-password": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "changePassword",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ChangePasswordRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "，",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/auth/validate": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "Token",
					"description": "Token",
					"operationId": "validateToken",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Token",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"token": map[string]interface{}{
												"type":        "string",
												"description": "Token",
											},
											"expires_at": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "Token",
						},
					},
				},
			},
			"/api/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。\n****：\n- ✅ ****\n- ✅ ****\n- ✅ ****\n****：\n**1（）：**  `/api/agent-loop` ，**** `conversationId` ，。，。\n**2：** ， `conversationId`  `/api/agent-loop` 。，。\n****：\n```json\n{\n  \"title\": \"Web\"\n}\n```",
					"operationId": "createConversation",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateConversationRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
						"500": map[string]interface{}{
							"description": "",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "listConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 50,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
								"minimum": 0,
							},
						},
						{
							"name":        "search",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Conversation",
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
					},
				},
			},
			"/api/conversations/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "getConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationDetail",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateConversationRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Conversation",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "（、）。****。",
					"operationId": "deleteConversation",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":        "string",
												"description": "",
												"example":     "",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
						"500": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/conversations/{id}/results": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，、",
					"operationId": "getConversationResults",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConversationResults",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
					},
				},
			},
			"/api/agent-loop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "AI（）",
					"description": "AI（）。**AI**，。\n****：\n- ✅ API/****\n- ✅ ****\n- ✅ ****，\n- ✅ ，\n****：\n1. ****： `POST /api/conversations` ， `conversationId`\n2. ****： `conversationId` \n****：\n**1 - ：**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web\"\n}\n```\n**2 - ：**\n```json\nPOST /api/agent-loop\n{\n  \"conversationId\": \"ID\",\n  \"message\": \" http://example.com SQL\",\n  \"role\": \"\"\n}\n```\n****：\n `conversationId`，。****，。\n****：AI、IDMCPID。。",
					"operationId": "sendMessage",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "（）",
											"example":     " http://example.com SQL",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "ID（）。\n- ****：（）\n- ****：（）",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "（），：、、Web",
											"example":     "",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "，AI",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"response": map[string]interface{}{
												"type":        "string",
												"description": "AI",
											},
											"conversationId": map[string]interface{}{
												"type":        "string",
												"description": "ID",
											},
											"mcpExecutionIds": map[string]interface{}{
												"type":        "array",
												"description": "MCPID",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"time": map[string]interface{}{
												"type":        "string",
												"format":      "date-time",
												"description": "",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
						"500": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/agent-loop/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "AI（）",
					"description": "AI（Server-Sent Events）。**AI**，。\n****：\n- ✅ API/****\n- ✅ ****\n- ✅ ****，\n- ✅ ，\n- ✅ ，AI\n****：\n1. ****： `POST /api/conversations` ， `conversationId`\n2. ****： `conversationId` \n****：\n**1 - ：**\n```json\nPOST /api/conversations\n{\n  \"title\": \"Web\"\n}\n```\n**2 - （）：**\n```json\nPOST /api/agent-loop/stream\n{\n  \"conversationId\": \"ID\",\n  \"message\": \" http://example.com SQL\",\n  \"role\": \"\"\n}\n```\n****：Server-Sent Events (SSE)，：\n- `message`: \n- `response`: AI\n- `progress`: \n- `done`: \n- `error`: \n- `cancelled`: ",
					"operationId": "sendMessageStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "（）",
											"example":     " http://example.com SQL",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": "ID（）。\n- ****：（）\n- ****：（）",
											"example":     "550e8400-e29b-41d4-a716-446655440000",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "（），：、、Web",
											"example":     "",
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "（Server-Sent Events）",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
						"500": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/eino-agent": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     " AI （Eino ADK ，）",
					"description": " `POST /api/agent-loop` ， **CloudWeGo Eino** `adk.NewChatModelAgent` + `adk.NewRunner.Run` （ MCP ）。**** `multi_agent.enabled`；`multi_agent.eino_skills` / `eino_middleware` 。 `webshellConnectionId`。",
					"operationId": "sendMessageEinoSingleAgent",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message":              map[string]interface{}{"type": "string"},
										"conversationId":       map[string]interface{}{"type": "string"},
										"role":                 map[string]interface{}{"type": "string"},
										"webshellConnectionId": map[string]interface{}{"type": "string"},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "， /api/agent-loop"},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"500": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/eino-agent/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     " AI （Eino ADK ，SSE）",
					"description": " `POST /api/agent-loop/stream` ； Eino **** ADK 。（ `tool_call` / `response_delta` ）。**** `multi_agent.enabled`。",
					"operationId": "sendMessageEinoSingleAgentStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message":              map[string]interface{}{"type": "string"},
										"conversationId":       map[string]interface{}{"type": "string"},
										"role":                 map[string]interface{}{"type": "string"},
										"webshellConnectionId": map[string]interface{}{"type": "string"},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "text/event-stream（SSE）",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE ",
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/multi-agent": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     " AI （Eino ，）",
					"description": " `POST /api/agent-loop` ， **CloudWeGo Eino** 。 `orchestration`（`deep` | `plan_execute` | `supervisor`）， `deep`。****：`multi_agent.enabled: true`； 404 JSON。 `webshellConnectionId`。",
					"operationId": "sendMessageMultiAgent",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type":        "string",
											"description": "（）",
										},
										"conversationId": map[string]interface{}{
											"type":        "string",
											"description": " ID（，）",
										},
										"role": map[string]interface{}{
											"type":        "string",
											"description": "（）",
										},
										"webshellConnectionId": map[string]interface{}{
											"type":        "string",
											"description": "WebShell  ID（， agent-loop ）",
										},
										"orchestration": map[string]interface{}{
											"type":        "string",
											"description": "Eino ：deep | plan_execute | supervisor； deep",
											"enum":        []string{"deep", "plan_execute", "supervisor"},
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "， /api/agent-loop",
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
						"500": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/multi-agent/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     " AI （Eino ，SSE）",
					"description": " `POST /api/agent-loop/stream` ； Eino 。`orchestration`  deep / plan_execute / supervisor， deep。****：`multi_agent.enabled: true`； SSE  `type: error`  `done`。 `webshellConnectionId`。",
					"operationId": "sendMessageMultiAgentStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message":              map[string]interface{}{"type": "string"},
										"conversationId":       map[string]interface{}{"type": "string"},
										"role":                 map[string]interface{}{"type": "string"},
										"webshellConnectionId": map[string]interface{}{"type": "string"},
										"orchestration": map[string]interface{}{
											"type":        "string",
											"description": "deep | plan_execute | supervisor； deep",
											"enum":        []string{"deep", "plan_execute", "supervisor"},
										},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "text/event-stream（SSE）",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "SSE ",
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/agent-loop/cancel": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Agent Loop",
					"operationId": "cancelAgentLoop",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CancelAgentLoopRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"status": map[string]interface{}{
												"type":    "string",
												"example": "cancelling",
											},
											"conversationId": map[string]interface{}{
												"type":        "string",
												"description": "ID",
											},
											"message": map[string]interface{}{
												"type":    "string",
												"example": "，。",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/agent-loop/tasks": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Agent Loop",
					"operationId": "listAgentTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/AgentTask",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/agent-loop/tasks/completed": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Agent Loop",
					"operationId": "listCompletedTasks",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"tasks": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/AgentTask",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "createBatchQueue",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/BatchTaskRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queueId": map[string]interface{}{
												"type":        "string",
												"description": "ID",
											},
											"queue": map[string]interface{}{
												"$ref": "#/components/schemas/BatchQueue",
											},
											"started": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "listBatchQueues",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queues": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/BatchQueue",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/BatchQueue",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "startBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/pause": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "pauseBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。，。，。\n****：\n，。：\n- \" http://example.com SQL\"\n- \" 192.168.1.1 \"\n- \" https://target.com XSS\"\n****：\n```json\n{\n  \"task\": \" http://example.com SQL\"\n}\n```",
					"operationId": "addBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"task"},
									"properties": map[string]interface{}{
										"task": map[string]interface{}{
											"type":        "string",
											"description": "，（）",
											"example":     " http://example.com SQL",
										},
									},
								},
								"examples": map[string]interface{}{
									"sqlInjection": map[string]interface{}{
										"summary":     "SQL",
										"description": "SQL",
										"value": map[string]interface{}{
											"task": " http://example.com SQL",
										},
									},
									"portScan": map[string]interface{}{
										"summary":     "",
										"description": "IP",
										"value": map[string]interface{}{
											"task": " 192.168.1.1 ",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"taskId": map[string]interface{}{
												"type":        "string",
												"description": "ID",
											},
											"message": map[string]interface{}{
												"type":        "string",
												"description": "",
												"example":     "",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "（task）",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/batch-tasks/{queueId}/tasks/{taskId}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"task": map[string]interface{}{
											"type":        "string",
											"description": "",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteBatchTask",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "taskId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "createGroup",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "listGroups",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Group",
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Group",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/{id}/conversations": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getGroupConversations",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"$ref": "#/components/schemas/Conversation",
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/conversations": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "addConversationToGroup",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AddConversationToGroupRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "removeConversationFromGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/vulnerabilities": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "listVulnerabilities",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
								"minimum": 0,
							},
						},
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "（offset）",
							"schema": map[string]interface{}{
								"type":    "integer",
								"minimum": 1,
							},
						},
						{
							"name":        "id",
							"in":          "query",
							"required":    false,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversation_id",
							"in":          "query",
							"required":    false,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "severity",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"critical", "high", "medium", "low", "info"},
							},
						},
						{
							"name":        "status",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"open", "closed", "fixed"},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ListVulnerabilitiesResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "createVulnerability",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateVulnerabilityRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/vulnerabilities/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getVulnerabilityStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/VulnerabilityStats",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/vulnerabilities/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateVulnerabilityRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Vulnerability",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteVulnerability",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getRoles",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/RoleConfig",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "createRole",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/RoleConfig",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/roles/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"role": map[string]interface{}{
												"$ref": "#/components/schemas/RoleConfig",
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/RoleConfig",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteRole",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/roles/skills/list": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "Skills",
					"description": "Skills，",
					"operationId": "getSkills",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/skills": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skills",
					"description": "Skills，",
					"operationId": "getSkills",
					"parameters": []map[string]interface{}{
						{
							"name":        "limit",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
							},
						},
						{
							"name":        "offset",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 0,
							},
						},
						{
							"name":        "search",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"skills": map[string]interface{}{
												"type":        "array",
												"description": "Skills",
												"items": map[string]interface{}{
													"$ref": "#/components/schemas/Skill",
												},
											},
											"total": map[string]interface{}{
												"type":        "integer",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "createSkill",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CreateSkillRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/skills/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "getSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "clearSkillStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/skills/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "getSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Skill",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Skill",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "updateSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateSkillRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "Skill",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "deleteSkill",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "Skill",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/skills/{name}/bound-roles": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "",
					"description": "Skill",
					"operationId": "getSkillBoundRoles",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"roles": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "Skill",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/skills/{name}/stats": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "Skill",
					"description": "Skill",
					"operationId": "clearSkillStatsByName",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "Skill",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "Skill",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/monitor": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "monitor",
					"parameters": []map[string]interface{}{
						{
							"name":        "page",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 1,
								"minimum": 1,
							},
						},
						{
							"name":        "page_size",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type":    "integer",
								"default": 20,
								"minimum": 1,
								"maximum": 100,
							},
						},
						{
							"name":        "status",
							"in":          "query",
							"required":    false,
							"description": "",
							"schema": map[string]interface{}{
								"type": "string",
								"enum": []string{"success", "failed", "running"},
							},
						},
						{
							"name":        "tool",
							"in":          "query",
							"required":    false,
							"description": "（）",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MonitorResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/monitor/execution/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ToolExecution",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteExecution",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/monitor/executions": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteExecutions",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/monitor/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/config": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ConfigResponse",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateConfig",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/UpdateConfigRequest",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/config/tools": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getTools",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "array",
										"description": "",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/config/apply": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "applyConfig",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/external-mcp": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "getExternalMCPs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"servers": map[string]interface{}{
												"type":        "object",
												"description": "MCP",
												"additionalProperties": map[string]interface{}{
													"$ref": "#/components/schemas/ExternalMCPResponse",
												},
											},
											"stats": map[string]interface{}{
												"type":        "object",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/external-mcp/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "getExternalMCPStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "object",
										"description": "",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/external-mcp/{name}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "getExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ExternalMCPResponse",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "MCP",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP。\n****：\n：\n**1. stdio（）**：\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"command\": \"node\",\n    \"args\": [\"/path/to/mcp-server.js\"],\n    \"env\": {}\n  }\n}\n```\n**2. sse（Server-Sent Events）**：\n```json\n{\n  \"config\": {\n    \"enabled\": true,\n    \"transport\": \"sse\",\n    \"url\": \"http://127.0.0.1:8082/sse\",\n    \"timeout\": 30\n  }\n}\n```\n****：\n- `enabled`: （boolean，）\n- `command`: （stdio，：\"node\", \"python\"）\n- `args`: （stdio）\n- `env`: （object，）\n- `transport`: （\"stdio\"  \"sse\"，sse）\n- `url`: SSEURL（sse）\n- `timeout`: （，，30）\n- `description`: （）",
					"operationId": "addOrUpdateExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP（）",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/AddOrUpdateExternalMCPRequest",
								},
								"examples": map[string]interface{}{
									"stdio": map[string]interface{}{
										"summary":     "stdio",
										"description": "MCP",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"command":     "node",
												"args":        []string{"/path/to/mcp-server.js"},
												"env":         map[string]interface{}{},
												"timeout":     30,
												"description": "Node.js MCP",
											},
										},
									},
									"sse": map[string]interface{}{
										"summary":     "SSE",
										"description": "Server-Sent EventsMCP",
										"value": map[string]interface{}{
											"config": map[string]interface{}{
												"enabled":     true,
												"transport":   "sse",
												"url":         "http://127.0.0.1:8082/sse",
												"timeout":     30,
												"description": "SSE MCP",
											},
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "MCP",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "（、）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "stdiocommandargs",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "deleteExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "MCP",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/external-mcp/{name}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "startExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "MCP",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/external-mcp/{name}/stop": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP",
					"operationId": "stopExternalMCP",
					"parameters": []map[string]interface{}{
						{
							"name":        "name",
							"in":          "path",
							"required":    true,
							"description": "MCP",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "MCP",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/attack-chain/{conversationId}/regenerate": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "regenerateAttackChain",
					"parameters": []map[string]interface{}{
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/AttackChain",
									},
								},
							},
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/conversations/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateConversationPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/{id}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateGroupPinned",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/groups/{id}/conversations/{conversationId}/pinned": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateConversationPinnedInGroup",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
						{
							"name":        "conversationId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"pinned"},
									"properties": map[string]interface{}{
										"pinned": map[string]interface{}{
											"type":        "boolean",
											"description": "",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/categories": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getKnowledgeCategories",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"categories": map[string]interface{}{
												"type":        "array",
												"description": "",
												"items": map[string]interface{}{
													"type": "string",
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/items": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getKnowledgeItems",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"items": map[string]interface{}{
												"type":        "array",
												"description": "",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "createKnowledgeItem",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":        "object",
									"description": "",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"400": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/items/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "updateKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":        "object",
									"description": "",
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteKnowledgeItem",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/index-status": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getIndexStatus",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
											"total_items": map[string]interface{}{
												"type":        "integer",
												"description": "",
											},
											"indexed_items": map[string]interface{}{
												"type":        "integer",
												"description": "",
											},
											"progress_percent": map[string]interface{}{
												"type":        "number",
												"description": "",
											},
											"is_complete": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/index": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "rebuildIndex",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/scan": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，",
					"operationId": "scanKnowledgeBase",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/search": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。，（）。\n****：\n- ： + ， TopK\n- （：SQL、XSS、）\n-  `/api/knowledge/categories` \n****：\n```json\n{\n  \"query\": \"SQL\",\n  \"riskType\": \"SQL\",\n  \"topK\": 5,\n  \"threshold\": 0.7\n}\n```",
					"operationId": "searchKnowledge",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type":     "object",
									"required": []string{"query"},
									"properties": map[string]interface{}{
										"query": map[string]interface{}{
											"type":        "string",
											"description": "，（）",
											"example":     "SQL",
										},
										"riskType": map[string]interface{}{
											"type":        "string",
											"description": "：（：SQL、XSS、）。 `/api/knowledge/categories` ，，。。",
											"example":     "SQL",
										},
										"topK": map[string]interface{}{
											"type":        "integer",
											"description": "：Top-K，5",
											"default":     5,
											"minimum":     1,
											"maximum":     50,
											"example":     5,
										},
										"threshold": map[string]interface{}{
											"type":        "number",
											"format":      "float",
											"description": "：（0-1），0.7。",
											"default":     0.7,
											"minimum":     0,
											"maximum":     1,
											"example":     0.7,
										},
									},
								},
								"examples": map[string]interface{}{
									"basic": map[string]interface{}{
										"summary":     "",
										"description": "，",
										"value": map[string]interface{}{
											"query": "SQL",
										},
									},
									"withRiskType": map[string]interface{}{
										"summary":     "",
										"description": "",
										"value": map[string]interface{}{
											"query":     "SQL",
											"riskType":  "SQL",
											"topK":      5,
											"threshold": 0.7,
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"results": map[string]interface{}{
												"type":        "array",
												"description": "，：item（）、chunks（）、score（）",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"item": map[string]interface{}{
															"type":        "object",
															"description": "",
														},
														"chunks": map[string]interface{}{
															"type":        "array",
															"description": "",
														},
														"score": map[string]interface{}{
															"type":        "number",
															"description": "（0-1）",
														},
													},
												},
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
									"example": map[string]interface{}{
										"results": []map[string]interface{}{
											{
												"item": map[string]interface{}{
													"id":       "item-1",
													"title":    "SQL",
													"category": "SQL",
												},
												"chunks": []map[string]interface{}{
													{
														"text": "SQL...",
													},
												},
												"score": 0.85,
											},
										},
										"enabled": true,
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "（query）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/Error",
									},
									"example": map[string]interface{}{
										"error": "",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
						"500": map[string]interface{}{
							"description": "（）",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "getRetrievalLogs",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"logs": map[string]interface{}{
												"type":        "array",
												"description": "",
											},
											"enabled": map[string]interface{}{
												"type":        "boolean",
												"description": "",
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			"/api/knowledge/retrieval-logs/{id}": map[string]interface{}{
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "",
					"operationId": "deleteRetrievalLog",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
						},
						"404": map[string]interface{}{
							"description": "",
						},
						"401": map[string]interface{}{
							"description": "",
						},
					},
				},
			},
			// English note.
			"/api/conversations/{id}/delete-turn": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "（ user  user ）， last_react 。",
					"operationId": "deleteConversationTurn",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"messageId"},
									"properties": map[string]interface{}{
										"messageId": map[string]interface{}{
											"type":        "string",
											"description": "ID，",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"deletedMessageIds": map[string]interface{}{
												"type":        "array",
												"items":       map[string]interface{}{"type": "string"},
												"description": "ID",
											},
											"message": map[string]interface{}{
												"type":    "string",
												"example": "ok",
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/messages/{id}/process-details": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，、。",
					"operationId": "getMessageProcessDetails",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"processDetails": map[string]interface{}{
												"type": "array",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"id":             map[string]interface{}{"type": "string", "description": "ID"},
														"messageId":      map[string]interface{}{"type": "string", "description": "ID"},
														"conversationId": map[string]interface{}{"type": "string", "description": "ID"},
														"eventType":      map[string]interface{}{"type": "string", "description": "（tool_call, thinking）"},
														"message":        map[string]interface{}{"type": "string", "description": ""},
														"data":           map[string]interface{}{"description": "（JSON）"},
														"createdAt":      map[string]interface{}{"type": "string", "format": "date-time", "description": ""},
													},
												},
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/batch-tasks/{queueId}/rerun": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，。",
					"operationId": "rerunBatchQueue",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string", "example": ""},
											"queueId": map[string]interface{}{"type": "string", "description": "ID"},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/batch-tasks/{queueId}/metadata": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "、。",
					"operationId": "updateBatchQueueMetadata",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"title":     map[string]interface{}{"type": "string", "description": ""},
										"role":      map[string]interface{}{"type": "string", "description": ""},
										"agentMode": map[string]interface{}{"type": "string", "description": "", "enum": []string{"single", "eino_single", "deep", "plan_execute", "supervisor"}},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queue": map[string]interface{}{"$ref": "#/components/schemas/BatchQueue"},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/batch-tasks/{queueId}/schedule": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Cron。。",
					"operationId": "updateBatchQueueSchedule",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"scheduleMode": map[string]interface{}{"type": "string", "description": "", "enum": []string{"manual", "cron"}},
										"cronExpr":     map[string]interface{}{"type": "string", "description": "Cron（scheduleModecron）", "example": "0 2 * * *"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queue": map[string]interface{}{"$ref": "#/components/schemas/BatchQueue"},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/batch-tasks/{queueId}/schedule-enabled": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "Cron",
					"description": "Cron，。",
					"operationId": "setBatchQueueScheduleEnabled",
					"parameters": []map[string]interface{}{
						{
							"name":        "queueId",
							"in":          "path",
							"required":    true,
							"description": "ID",
							"schema":      map[string]interface{}{"type": "string"},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"scheduleEnabled"},
									"properties": map[string]interface{}{
										"scheduleEnabled": map[string]interface{}{"type": "boolean", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"queue": map[string]interface{}{"$ref": "#/components/schemas/BatchQueue"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/groups/mappings": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "getAllGroupMappings",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"conversation_id": map[string]interface{}{"type": "string", "description": "ID"},
												"group_id":        map[string]interface{}{"type": "string", "description": "ID"},
												"pinned":          map[string]interface{}{"type": "boolean", "description": ""},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/fofa/search": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"FOFA"},
					"summary":     "FOFA",
					"description": "FOFA，。",
					"operationId": "fofaSearch",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"query"},
									"properties": map[string]interface{}{
										"query":  map[string]interface{}{"type": "string", "description": "FOFA", "example": "domain=\"example.com\""},
										"size":   map[string]interface{}{"type": "integer", "description": "（100，10000）", "default": 100},
										"page":   map[string]interface{}{"type": "integer", "description": "（1）", "default": 1},
										"fields": map[string]interface{}{"type": "string", "description": "，", "example": "host,ip,port,title"},
										"full":   map[string]interface{}{"type": "boolean", "description": "", "default": false},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"query":         map[string]interface{}{"type": "string", "description": ""},
											"size":          map[string]interface{}{"type": "integer"},
											"page":          map[string]interface{}{"type": "integer"},
											"total":         map[string]interface{}{"type": "integer", "description": ""},
											"fields":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
											"results_count": map[string]interface{}{"type": "integer"},
											"results":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": ""},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/fofa/parse": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"FOFA"},
					"summary":     "FOFA",
					"description": "AIFOFA，。",
					"operationId": "fofaParse",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"text"},
									"properties": map[string]interface{}{
										"text": map[string]interface{}{"type": "string", "description": "", "example": "WordPress"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"query":       map[string]interface{}{"type": "string", "description": "FOFA"},
											"explanation": map[string]interface{}{"type": "string", "description": ""},
											"warnings":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": ""},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/config/test-openai": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "OpenAI API",
					"description": "OpenAI/Claude API，。",
					"operationId": "testOpenAI",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"model"},
									"properties": map[string]interface{}{
										"provider": map[string]interface{}{"type": "string", "description": "LLM（openai/claude）", "example": "openai"},
										"base_url": map[string]interface{}{"type": "string", "description": "API（，provider）"},
										"api_key":  map[string]interface{}{"type": "string", "description": "API"},
										"model":    map[string]interface{}{"type": "string", "description": "", "example": "gpt-4"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"success":    map[string]interface{}{"type": "boolean", "description": ""},
											"error":      map[string]interface{}{"type": "string", "description": "（success=false）"},
											"model":      map[string]interface{}{"type": "string", "description": "（success=true）"},
											"latency_ms": map[string]interface{}{"type": "number", "description": "（success=true）"},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/terminal/run": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "Shell。",
					"operationId": "terminalRun",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"command"},
									"properties": map[string]interface{}{
										"command": map[string]interface{}{"type": "string", "description": ""},
										"shell":   map[string]interface{}{"type": "string", "description": "Shell（sh/cmd）"},
										"cwd":     map[string]interface{}{"type": "string", "description": "（）"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"stdout":    map[string]interface{}{"type": "string", "description": ""},
											"stderr":    map[string]interface{}{"type": "string", "description": ""},
											"exit_code": map[string]interface{}{"type": "integer", "description": ""},
											"error":     map[string]interface{}{"type": "string", "description": "（）"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/terminal/run/stream": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "SSEShell，。 JSON: {\"t\": \"out\"|\"err\"|\"exit\", \"d\": \"\", \"c\": }",
					"operationId": "terminalRunStream",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"command"},
									"properties": map[string]interface{}{
										"command": map[string]interface{}{"type": "string", "description": ""},
										"shell":   map[string]interface{}{"type": "string", "description": "Shell（sh/cmd）"},
										"cwd":     map[string]interface{}{"type": "string", "description": "（）"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "SSE",
							"content": map[string]interface{}{
								"text/event-stream": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":        "string",
										"description": "Server-Sent Events，JSON: {\"t\":\"out|err|exit\",\"d\":\"data\",\"c\":exitCode}",
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/terminal/ws": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "WebSocket",
					"description": "WebSocket，PTY。/，JSON: {\"type\":\"resize\",\"cols\":80,\"rows\":24} 。PTY。",
					"operationId": "terminalWS",
					"responses": map[string]interface{}{
						"101": map[string]interface{}{"description": "WebSocket"},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/webshell/connections": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell。",
					"operationId": "listWebshellConnections",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"id":         map[string]interface{}{"type": "string", "description": "ID"},
												"url":        map[string]interface{}{"type": "string", "description": "WebShell URL"},
												"password":   map[string]interface{}{"type": "string", "description": ""},
												"type":       map[string]interface{}{"type": "string", "description": "Shell", "enum": []string{"php", "asp", "aspx", "jsp", "custom"}},
												"method":     map[string]interface{}{"type": "string", "description": "", "enum": []string{"get", "post"}},
												"cmd_param":  map[string]interface{}{"type": "string", "description": ""},
												"remark":     map[string]interface{}{"type": "string", "description": ""},
												"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell。",
					"operationId": "createWebshellConnection",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"url"},
									"properties": map[string]interface{}{
										"url":       map[string]interface{}{"type": "string", "description": "WebShell URL"},
										"password":  map[string]interface{}{"type": "string", "description": ""},
										"type":      map[string]interface{}{"type": "string", "description": "Shell", "enum": []string{"php", "asp", "aspx", "jsp", "custom"}},
										"method":    map[string]interface{}{"type": "string", "description": "", "enum": []string{"get", "post"}},
										"cmd_param": map[string]interface{}{"type": "string", "description": ""},
										"remark":    map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/connections/{id}": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell。",
					"operationId": "updateWebshellConnection",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"url":       map[string]interface{}{"type": "string"},
										"password":  map[string]interface{}{"type": "string"},
										"type":      map[string]interface{}{"type": "string", "enum": []string{"php", "asp", "aspx", "jsp", "custom"}},
										"method":    map[string]interface{}{"type": "string", "enum": []string{"get", "post"}},
										"cmd_param": map[string]interface{}{"type": "string"},
										"remark":    map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell。",
					"operationId": "deleteWebshellConnection",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/connections/{id}/state": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "",
					"description": "WebShell。",
					"operationId": "getWebshellConnectionState",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"state": map[string]interface{}{"type": "object", "description": "（JSON）"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "",
					"description": "WebShell。",
					"operationId": "saveWebshellConnectionState",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"state": map[string]interface{}{"type": "object", "description": "（JSON）"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/connections/{id}/ai-history": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "AI",
					"description": "WebShellAI。",
					"operationId": "getWebshellAIHistory",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"conversationId": map[string]interface{}{"type": "string"},
											"messages": map[string]interface{}{
												"type": "array",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"id":        map[string]interface{}{"type": "string"},
														"role":      map[string]interface{}{"type": "string"},
														"content":   map[string]interface{}{"type": "string"},
														"createdAt": map[string]interface{}{"type": "string", "format": "date-time"},
													},
												},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/connections/{id}/ai-conversations": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "AI",
					"description": "WebShellAI。",
					"operationId": "listWebshellAIConversations",
					"parameters": []map[string]interface{}{
						{"name": "id", "in": "path", "required": true, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"id":        map[string]interface{}{"type": "string"},
												"title":     map[string]interface{}{"type": "string"},
												"createdAt": map[string]interface{}{"type": "string", "format": "date-time"},
											},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/exec": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell。",
					"operationId": "webshellExec",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"url", "command"},
									"properties": map[string]interface{}{
										"url":       map[string]interface{}{"type": "string", "description": "WebShell URL"},
										"password":  map[string]interface{}{"type": "string"},
										"type":      map[string]interface{}{"type": "string", "enum": []string{"php", "asp", "aspx", "jsp", "custom"}},
										"method":    map[string]interface{}{"type": "string", "enum": []string{"get", "post"}},
										"cmd_param": map[string]interface{}{"type": "string"},
										"command":   map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"ok":        map[string]interface{}{"type": "boolean"},
											"output":    map[string]interface{}{"type": "string", "description": ""},
											"error":     map[string]interface{}{"type": "string", "description": ""},
											"http_code": map[string]interface{}{"type": "integer", "description": "HTTP"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/webshell/file": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"WebShell"},
					"summary":     "WebShell",
					"description": "WebShell（、、、、、）。",
					"operationId": "webshellFileOp",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"url", "action", "path"},
									"properties": map[string]interface{}{
										"url":         map[string]interface{}{"type": "string", "description": "WebShell URL"},
										"password":    map[string]interface{}{"type": "string"},
										"type":        map[string]interface{}{"type": "string", "enum": []string{"php", "asp", "aspx", "jsp", "custom"}},
										"method":      map[string]interface{}{"type": "string", "enum": []string{"get", "post"}},
										"cmd_param":   map[string]interface{}{"type": "string"},
										"action":      map[string]interface{}{"type": "string", "description": "", "enum": []string{"list", "read", "delete", "write", "mkdir", "rename", "upload", "upload_chunk"}},
										"path":        map[string]interface{}{"type": "string", "description": "/"},
										"target_path": map[string]interface{}{"type": "string", "description": "（rename）"},
										"content":     map[string]interface{}{"type": "string", "description": "（write/upload）"},
										"chunk_index": map[string]interface{}{"type": "integer", "description": "（upload_chunk）"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"ok":     map[string]interface{}{"type": "boolean"},
											"output": map[string]interface{}{"type": "string"},
											"error":  map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/chat-uploads": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，ID。",
					"operationId": "listChatUploads",
					"parameters": []map[string]interface{}{
						{"name": "conversation", "in": "query", "required": false, "description": "ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"files": map[string]interface{}{
												"type": "array",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"relativePath":  map[string]interface{}{"type": "string"},
														"absolutePath":  map[string]interface{}{"type": "string"},
														"name":          map[string]interface{}{"type": "string"},
														"size":          map[string]interface{}{"type": "integer"},
														"modifiedUnix":  map[string]interface{}{"type": "integer"},
														"date":          map[string]interface{}{"type": "string"},
														"conversationId": map[string]interface{}{"type": "string"},
														"subPath":       map[string]interface{}{"type": "string"},
													},
												},
											},
											"folders": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "（multipart/form-data）。",
					"operationId": "uploadChatFile",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"file"},
									"properties": map[string]interface{}{
										"file":           map[string]interface{}{"type": "string", "format": "binary", "description": ""},
										"conversationId": map[string]interface{}{"type": "string", "description": "ID（）"},
										"relativeDir":    map[string]interface{}{"type": "string", "description": "（）"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"ok":           map[string]interface{}{"type": "boolean"},
											"relativePath": map[string]interface{}{"type": "string"},
											"absolutePath": map[string]interface{}{"type": "string"},
											"name":         map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "deleteChatUpload",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"path"},
									"properties": map[string]interface{}{
										"path": map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/chat-uploads/download": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "downloadChatUpload",
					"parameters": []map[string]interface{}{
						{"name": "path", "in": "query", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/octet-stream": map[string]interface{}{
									"schema": map[string]interface{}{"type": "string", "format": "binary"},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/chat-uploads/content": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "getChatUploadContent",
					"parameters": []map[string]interface{}{
						{"name": "path", "in": "query", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"content": map[string]interface{}{"type": "string", "description": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "putChatUploadContent",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"path", "content"},
									"properties": map[string]interface{}{
										"path":    map[string]interface{}{"type": "string", "description": ""},
										"content": map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/chat-uploads/mkdir": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "mkdirChatUpload",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name"},
									"properties": map[string]interface{}{
										"parent": map[string]interface{}{"type": "string", "description": ""},
										"name":   map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"ok":           map[string]interface{}{"type": "boolean"},
											"relativePath": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/chat-uploads/rename": map[string]interface{}{
				"put": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。",
					"operationId": "renameChatUpload",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"path", "newName"},
									"properties": map[string]interface{}{
										"path":    map[string]interface{}{"type": "string", "description": ""},
										"newName": map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"ok":           map[string]interface{}{"type": "boolean"},
											"relativePath": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/robot/wecom": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "URL（）。。",
					"operationId": "wecomCallbackVerify",
					"security":    []map[string]interface{}{},
					"parameters": []map[string]interface{}{
						{"name": "msg_signature", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
						{"name": "timestamp", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
						{"name": "nonce", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
						{"name": "echostr", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "，echostr"},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。，。",
					"operationId": "wecomCallbackMessage",
					"security":    []map[string]interface{}{},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/robot/dingtalk": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。，。",
					"operationId": "dingtalkCallback",
					"security":    []map[string]interface{}{},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/robot/lark": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "。，。",
					"operationId": "larkCallback",
					"security":    []map[string]interface{}{},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/robot/test": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，。。",
					"operationId": "testRobot",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"platform", "text"},
									"properties": map[string]interface{}{
										"platform": map[string]interface{}{"type": "string", "description": "", "enum": []string{"dingtalk", "lark", "wecom"}},
										"user_id":  map[string]interface{}{"type": "string", "description": "ID", "example": "test"},
										"text":     map[string]interface{}{"type": "string", "description": "", "example": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/multi-agent/markdown-agents": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Markdown"},
					"summary":     "Markdown",
					"description": "Markdown。",
					"operationId": "listMarkdownAgents",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"agents": map[string]interface{}{
												"type": "array",
												"items": map[string]interface{}{
													"type": "object",
													"properties": map[string]interface{}{
														"filename":        map[string]interface{}{"type": "string", "description": ""},
														"id":              map[string]interface{}{"type": "string", "description": "ID"},
														"name":            map[string]interface{}{"type": "string", "description": ""},
														"description":     map[string]interface{}{"type": "string", "description": ""},
														"is_orchestrator": map[string]interface{}{"type": "boolean", "description": ""},
														"kind":            map[string]interface{}{"type": "string", "description": ""},
													},
												},
											},
											"dir": map[string]interface{}{"type": "string", "description": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
				"post": map[string]interface{}{
					"tags":        []string{"Markdown"},
					"summary":     "Markdown",
					"description": "Markdown。",
					"operationId": "createMarkdownAgent",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"name"},
									"properties": map[string]interface{}{
										"filename":       map[string]interface{}{"type": "string", "description": "（，）"},
										"id":             map[string]interface{}{"type": "string", "description": "ID"},
										"name":           map[string]interface{}{"type": "string", "description": ""},
										"description":    map[string]interface{}{"type": "string", "description": ""},
										"tools":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": ""},
										"instruction":    map[string]interface{}{"type": "string", "description": ""},
										"bind_role":      map[string]interface{}{"type": "string", "description": ""},
										"max_iterations": map[string]interface{}{"type": "integer", "description": ""},
										"kind":           map[string]interface{}{"type": "string", "description": ""},
										"raw":            map[string]interface{}{"type": "string", "description": "Markdown"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"filename": map[string]interface{}{"type": "string"},
											"message":  map[string]interface{}{"type": "string", "example": ""},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/multi-agent/markdown-agents/{filename}": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Markdown"},
					"summary":     "Markdown",
					"description": "Markdown。",
					"operationId": "getMarkdownAgent",
					"parameters": []map[string]interface{}{
						{"name": "filename", "in": "path", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"filename":        map[string]interface{}{"type": "string"},
											"raw":             map[string]interface{}{"type": "string", "description": "Markdown"},
											"id":              map[string]interface{}{"type": "string"},
											"name":            map[string]interface{}{"type": "string"},
											"description":     map[string]interface{}{"type": "string"},
											"tools":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
											"instruction":     map[string]interface{}{"type": "string"},
											"bind_role":       map[string]interface{}{"type": "string"},
											"max_iterations":  map[string]interface{}{"type": "integer"},
											"kind":            map[string]interface{}{"type": "string"},
											"is_orchestrator": map[string]interface{}{"type": "boolean"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Markdown"},
					"summary":     "Markdown",
					"description": "Markdown。",
					"operationId": "updateMarkdownAgent",
					"parameters": []map[string]interface{}{
						{"name": "filename", "in": "path", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name":           map[string]interface{}{"type": "string"},
										"description":    map[string]interface{}{"type": "string"},
										"tools":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
										"instruction":    map[string]interface{}{"type": "string"},
										"bind_role":      map[string]interface{}{"type": "string"},
										"max_iterations": map[string]interface{}{"type": "integer"},
										"kind":           map[string]interface{}{"type": "string"},
										"raw":            map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string", "example": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
				"delete": map[string]interface{}{
					"tags":        []string{"Markdown"},
					"summary":     "Markdown",
					"description": "Markdown。",
					"operationId": "deleteMarkdownAgent",
					"parameters": []map[string]interface{}{
						{"name": "filename", "in": "path", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string", "example": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/skills/{name}/files": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "",
					"description": "。",
					"operationId": "listSkillPackageFiles",
					"parameters": []map[string]interface{}{
						{"name": "name", "in": "path", "required": true, "description": "/ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"files": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
			},
			"/api/skills/{name}/file": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "",
					"description": "。",
					"operationId": "getSkillPackageFile",
					"parameters": []map[string]interface{}{
						{"name": "name", "in": "path", "required": true, "description": "/ID", "schema": map[string]interface{}{"type": "string"}},
						{"name": "path", "in": "query", "required": true, "description": "", "schema": map[string]interface{}{"type": "string"}},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"path":    map[string]interface{}{"type": "string", "description": ""},
											"content": map[string]interface{}{"type": "string", "description": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
						"404": map[string]interface{}{"description": ""},
					},
				},
				"put": map[string]interface{}{
					"tags":        []string{"Skills"},
					"summary":     "",
					"description": "。",
					"operationId": "putSkillPackageFile",
					"parameters": []map[string]interface{}{
						{"name": "name", "in": "path", "required": true, "description": "/ID", "schema": map[string]interface{}{"type": "string"}},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"path"},
									"properties": map[string]interface{}{
										"path":    map[string]interface{}{"type": "string", "description": ""},
										"content": map[string]interface{}{"type": "string", "description": ""},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{"type": "string", "example": "saved"},
											"path":    map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/monitor/executions/names": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "ID，N+1。",
					"operationId": "batchGetToolNames",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"required": []string{"ids"},
									"properties": map[string]interface{}{
										"ids": map[string]interface{}{
											"type":        "array",
											"items":       map[string]interface{}{"type": "string"},
											"description": "ID",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "，ID",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type":                 "object",
										"additionalProperties": map[string]interface{}{"type": "string"},
										"description":          "ID，",
										"example":              map[string]interface{}{"exec-001": "nmap", "exec-002": "sqlmap"},
									},
								},
							},
						},
						"400": map[string]interface{}{"description": ""},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			// English note.
			"/api/knowledge/stats": map[string]interface{}{
				"get": map[string]interface{}{
					"tags":        []string{""},
					"summary":     "",
					"description": "，。",
					"operationId": "getKnowledgeStats",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"enabled":          map[string]interface{}{"type": "boolean", "description": ""},
											"total_categories": map[string]interface{}{"type": "integer", "description": ""},
											"total_items":      map[string]interface{}{"type": "integer", "description": ""},
										},
									},
								},
							},
						},
						"401": map[string]interface{}{"description": ""},
					},
				},
			},

			"/api/mcp": map[string]interface{}{
				"post": map[string]interface{}{
					"tags":        []string{"MCP"},
					"summary":     "MCP",
					"description": "MCP (Model Context Protocol) ，MCP。\n****：\n JSON-RPC 2.0 ，：\n**1. initialize** - MCP\n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"init-1\",\n  \"method\": \"initialize\",\n  \"params\": {\n    \"protocolVersion\": \"2024-11-05\",\n    \"capabilities\": {},\n    \"clientInfo\": {\n      \"name\": \"MyClient\",\n      \"version\": \"1.0.0\"\n    }\n  }\n}\n```\n**2. tools/list** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"list-1\",\n  \"method\": \"tools/list\",\n  \"params\": {}\n}\n```\n**3. tools/call** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"call-1\",\n  \"method\": \"tools/call\",\n  \"params\": {\n    \"name\": \"nmap\",\n    \"arguments\": {\n      \"target\": \"192.168.1.1\",\n      \"ports\": \"80,443\"\n    }\n  }\n}\n```\n**4. prompts/list** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompts-list-1\",\n  \"method\": \"prompts/list\",\n  \"params\": {}\n}\n```\n**5. prompts/get** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"prompt-get-1\",\n  \"method\": \"prompts/get\",\n  \"params\": {\n    \"name\": \"prompt-name\",\n    \"arguments\": {}\n  }\n}\n```\n**6. resources/list** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resources-list-1\",\n  \"method\": \"resources/list\",\n  \"params\": {}\n}\n```\n**7. resources/read** - \n```json\n{\n  \"jsonrpc\": \"2.0\",\n  \"id\": \"resource-read-1\",\n  \"method\": \"resources/read\",\n  \"params\": {\n    \"uri\": \"resource://example\"\n  }\n}\n```\n****：\n- `-32700`: Parse error - JSON\n- `-32600`: Invalid Request - \n- `-32601`: Method not found - \n- `-32602`: Invalid params - \n- `-32603`: Internal error - ",
					"operationId": "mcpEndpoint",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/MCPMessage",
								},
								"examples": map[string]interface{}{
									"listTools": map[string]interface{}{
										"summary":     "",
										"description": "MCP",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "list-tools-1",
											"method":  "tools/list",
											"params":  map[string]interface{}{},
										},
									},
									"callTool": map[string]interface{}{
										"summary":     "",
										"description": "MCP",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "call-tool-1",
											"method":  "tools/call",
											"params": map[string]interface{}{
												"name": "nmap",
												"arguments": map[string]interface{}{
													"target": "192.168.1.1",
													"ports":  "80,443",
												},
											},
										},
									},
									"initialize": map[string]interface{}{
										"summary":     "",
										"description": "MCP，",
										"value": map[string]interface{}{
											"jsonrpc": "2.0",
											"id":      "init-1",
											"method":  "initialize",
											"params": map[string]interface{}{
												"protocolVersion": "2024-11-05",
												"capabilities":    map[string]interface{}{},
												"clientInfo": map[string]interface{}{
													"name":    "MyClient",
													"version": "1.0.0",
												},
											},
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "MCP（JSON-RPC 2.0）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MCPResponse",
									},
									"examples": map[string]interface{}{
										"success": map[string]interface{}{
											"summary":     "",
											"description": "",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"result": map[string]interface{}{
													"content": []map[string]interface{}{
														{
															"type": "text",
															"text": "...",
														},
													},
													"isError": false,
												},
											},
										},
										"error": map[string]interface{}{
											"summary":     "",
											"description": "",
											"value": map[string]interface{}{
												"jsonrpc": "2.0",
												"id":      "call-tool-1",
												"error": map[string]interface{}{
													"code":    -32601,
													"message": "Tool not found",
													"data":    " 'unknown-tool' ",
												},
											},
										},
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "（JSON）",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/MCPResponse",
									},
									"example": map[string]interface{}{
										"id": nil,
										"error": map[string]interface{}{
											"code":    -32700,
											"message": "Parse error",
											"data":    "unexpected end of JSON input",
										},
										"jsonrpc": "2.0",
									},
								},
							},
						},
						"401": map[string]interface{}{
							"description": "，Token",
						},
						"405": map[string]interface{}{
							"description": "（POST）",
						},
					},
				},
			},
		},
	}

	enrichSpecWithI18nKeys(spec)
	c.JSON(http.StatusOK, spec)
}

// English note.
// English note.
// English note.
func (h *OpenAPIHandler) GetConversationResults(c *gin.Context) {
	conversationID := c.Param("id")

	// English note.
	conv, err := h.db.GetConversation(conversationID)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	// English note.
	messages, err := h.db.GetMessages(conversationID)
	if err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// English note.
	vulnList, err := h.db.ListVulnerabilities(1000, 0, "", conversationID, "", "")
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		vulnList = []*database.Vulnerability{}
	}
	vulnerabilities := make([]database.Vulnerability, len(vulnList))
	for i, v := range vulnList {
		vulnerabilities[i] = *v
	}

	// English note.
	executionResults := []map[string]interface{}{}
	for _, msg := range messages {
		if len(msg.MCPExecutionIDs) > 0 {
			for _, execID := range msg.MCPExecutionIDs {
				// English note.
				if h.resultStorage != nil {
					result, err := h.resultStorage.GetResult(execID)
					if err == nil && result != "" {
						// English note.
						metadata, err := h.resultStorage.GetResultMetadata(execID)
						toolName := "unknown"
						createdAt := time.Now()
						if err == nil && metadata != nil {
							toolName = metadata.ToolName
							createdAt = metadata.CreatedAt
						}
						executionResults = append(executionResults, map[string]interface{}{
							"id":        execID,
							"toolName":  toolName,
							"status":    "success",
							"result":    result,
							"createdAt": createdAt.Format(time.RFC3339),
						})
					}
				}
			}
		}
	}

	response := map[string]interface{}{
		"conversationId":   conv.ID,
		"messages":         messages,
		"vulnerabilities":  vulnerabilities,
		"executionResults": executionResults,
	}

	c.JSON(http.StatusOK, response)
}
