package handler

import (
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

// English note.
type multiAgentPrepared struct {
	ConversationID     string
	CreatedNew         bool
	History            []agent.ChatMessage
	FinalMessage       string
	RoleTools          []string
	RoleSkills         []string
	AssistantMessageID string
	UserMessageID      string
}

func (h *AgentHandler) prepareMultiAgentSession(req *ChatRequest) (*multiAgentPrepared, error) {
	if len(req.Attachments) > maxAttachments {
		return nil, fmt.Errorf(" %d ", maxAttachments)
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	createdNew := false
	if conversationID == "" {
		title := safeTruncateString(req.Message, 50)
		var conv *database.Conversation
		var err error
		if strings.TrimSpace(req.WebShellConnectionID) != "" {
			conv, err = h.db.CreateConversationWithWebshell(strings.TrimSpace(req.WebShellConnectionID), title)
		} else {
			conv, err = h.db.CreateConversation(title)
		}
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}
		conversationID = conv.ID
		createdNew = true
	} else {
		if _, err := h.db.GetConversation(conversationID); err != nil {
			return nil, fmt.Errorf("")
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
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}
	}

	finalMessage := req.Message
	var roleTools []string
	var roleSkills []string
	if req.WebShellConnectionID != "" {
		conn, errConn := h.db.GetWebshellConnection(strings.TrimSpace(req.WebShellConnectionID))
		if errConn != nil || conn == nil {
			h.logger.Warn("WebShell AI ：", zap.String("id", req.WebShellConnectionID), zap.Error(errConn))
			return nil, fmt.Errorf(" WebShell ")
		}
		remark := conn.Remark
		if remark == "" {
			remark = conn.URL
		}
		webshellContext := fmt.Sprintf("[WebShell ]  ID：%s，：%s。（，connection_id  \"%s\"）：webshell_exec、webshell_file_list、webshell_file_read、webshell_file_write、record_vulnerability、list_knowledge_risk_types、search_knowledge_base。Skills  Eino  `skill` 。\n\n：%s",
			conn.ID, remark, conn.ID, req.Message)
		// English note.
		if req.Role != "" && req.Role != "" && h.config != nil && h.config.Roles != nil {
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
	} else if req.Role != "" && req.Role != "" && h.config != nil && h.config.Roles != nil {
		if role, exists := h.config.Roles[req.Role]; exists && role.Enabled {
			if role.UserPrompt != "" {
				finalMessage = role.UserPrompt + "\n\n" + req.Message
			}
			roleTools = role.Tools
			roleSkills = role.Skills
		}
	}

	var savedPaths []string
	if len(req.Attachments) > 0 {
		var aerr error
		savedPaths, aerr = saveAttachmentsToDateAndConversationDir(req.Attachments, conversationID, h.logger)
		if aerr != nil {
			return nil, fmt.Errorf(": %w", aerr)
		}
	}
	finalMessage = appendAttachmentsToMessage(finalMessage, req.Attachments, savedPaths)

	userContent := userMessageContentForStorage(req.Message, req.Attachments, savedPaths)
	userMsgRow, uerr := h.db.AddMessage(conversationID, "user", userContent, nil)
	if uerr != nil {
		h.logger.Error("", zap.Error(uerr))
		return nil, fmt.Errorf(": %w", uerr)
	}
	userMessageID := ""
	if userMsgRow != nil {
		userMessageID = userMsgRow.ID
	}

	assistantMsg, aerr := h.db.AddMessage(conversationID, "assistant", "...", nil)
	var assistantMessageID string
	if aerr != nil {
		h.logger.Warn("", zap.Error(aerr))
	} else if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	return &multiAgentPrepared{
		ConversationID:     conversationID,
		CreatedNew:         createdNew,
		History:            agentHistoryMessages,
		FinalMessage:       finalMessage,
		RoleTools:          roleTools,
		RoleSkills:         roleSkills,
		AssistantMessageID: assistantMessageID,
		UserMessageID:      userMessageID,
	}, nil
}
