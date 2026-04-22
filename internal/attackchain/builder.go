package attackchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/openai"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// English note.
type Builder struct {
	db           *database.DB
	logger       *zap.Logger
	openAIClient *openai.Client
	openAIConfig *config.OpenAIConfig
	tokenCounter agent.TokenCounter
	maxTokens    int // tokens，100000
}

// English note.
type Node = database.AttackChainNode

// English note.
type Edge = database.AttackChainEdge

// English note.
type Chain struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// English note.
func NewBuilder(db *database.DB, openAIConfig *config.OpenAIConfig, logger *zap.Logger) *Builder {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient := &http.Client{Timeout: 5 * time.Minute, Transport: transport}

	// English note.
	maxTokens := 0
	if openAIConfig != nil && openAIConfig.MaxTotalTokens > 0 {
		maxTokens = openAIConfig.MaxTotalTokens
	} else if openAIConfig != nil {
		// English note.
		model := strings.ToLower(openAIConfig.Model)
		if strings.Contains(model, "gpt-4") {
			maxTokens = 128000 // gpt-4128k
		} else if strings.Contains(model, "gpt-3.5") {
			maxTokens = 16000 // gpt-3.5-turbo16k
		} else if strings.Contains(model, "deepseek") {
			maxTokens = 131072 // deepseek-chat131k
		} else {
			maxTokens = 100000 // 
		}
	} else {
		// English note.
		maxTokens = 100000
	}

	return &Builder{
		db:           db,
		logger:       logger,
		openAIClient: openai.NewClient(openAIConfig, httpClient, logger),
		openAIConfig: openAIConfig,
		tokenCounter: agent.NewTikTokenCounter(),
		maxTokens:    maxTokens,
	}
}

// English note.
func (b *Builder) BuildChainFromConversation(ctx context.Context, conversationID string) (*Chain, error) {
	b.logger.Info("（）", zap.String("conversationId", conversationID))

	// English note.
	messages, err := b.db.GetMessages(conversationID)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	if len(messages) == 0 {
		b.logger.Info("", zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// English note.
	// English note.
	hasToolExecutions := false
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			if len(messages[i].MCPExecutionIDs) > 0 {
				hasToolExecutions = true
				break
			}
		}
	}
	if !hasToolExecutions {
		if pdOK, err := b.db.ConversationHasToolProcessDetails(conversationID); err != nil {
			b.logger.Warn("", zap.Error(err))
		} else if pdOK {
			hasToolExecutions = true
		}
	}

	// English note.
	taskCancelled := false
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			content := strings.ToLower(messages[i].Content)
			if strings.Contains(content, "") || strings.Contains(content, "cancelled") {
				taskCancelled = true
			}
			break
		}
	}

	// English note.
	if taskCancelled && !hasToolExecutions {
		b.logger.Info("，",
			zap.String("conversationId", conversationID),
			zap.Bool("taskCancelled", taskCancelled),
			zap.Bool("hasToolExecutions", hasToolExecutions))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// English note.
	if !hasToolExecutions {
		b.logger.Info("，",
			zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// English note.
	reactInputJSON, modelOutput, err := b.db.GetReActData(conversationID)
	if err != nil {
		b.logger.Warn("ReAct，", zap.Error(err))
		// English note.
		reactInputJSON = ""
		modelOutput = ""
	}

	// var userInput string
	var reactInputFinal string
	var dataSource string // 

	// English note.
	if reactInputJSON != "" && modelOutput != "" {
		// English note.
		hash := sha256.Sum256([]byte(reactInputJSON))
		reactInputHash := hex.EncodeToString(hash[:])[:16] // 16

		// English note.
		var messageCount int
		var tempMessages []interface{}
		if json.Unmarshal([]byte(reactInputJSON), &tempMessages) == nil {
			messageCount = len(tempMessages)
		}

		dataSource = "database_last_react_input"
		b.logger.Info("ReAct",
			zap.String("conversationId", conversationID),
			zap.String("dataSource", dataSource),
			zap.Int("reactInputSize", len(reactInputJSON)),
			zap.Int("messageCount", messageCount),
			zap.String("reactInputHash", reactInputHash),
			zap.Int("modelOutputSize", len(modelOutput)))

		// English note.
		// userInput = b.extractUserInputFromReActInput(reactInputJSON)

		// English note.
		reactInputFinal = b.formatReActInputFromJSON(reactInputJSON)
	} else {
		// English note.
		dataSource = "messages_table"
		b.logger.Info("ReAct",
			zap.String("conversationId", conversationID),
			zap.String("dataSource", dataSource),
			zap.Int("messageCount", len(messages)))

		// English note.
		for i := len(messages) - 1; i >= 0; i-- {
			if strings.EqualFold(messages[i].Role, "user") {
				// userInput = messages[i].Content
				break
			}
		}

		// English note.
		reactInputFinal = b.buildReActInput(messages)

		// English note.
		for i := len(messages) - 1; i >= 0; i-- {
			if strings.EqualFold(messages[i].Role, "assistant") {
				modelOutput = messages[i].Content
				break
			}
		}
	}

	// English note.
	hasMCPOnAssistant := false
	var lastAssistantID string
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			lastAssistantID = messages[i].ID
			if len(messages[i].MCPExecutionIDs) > 0 {
				hasMCPOnAssistant = true
			}
			break
		}
	}
	if lastAssistantID != "" {
		pdHasTools, _ := b.db.ConversationHasToolProcessDetails(conversationID)
		if pdHasTools && !(hasMCPOnAssistant && reactInputContainsToolTrace(reactInputJSON)) {
			detailsMap, err := b.db.GetProcessDetailsByConversation(conversationID)
			if err != nil {
				b.logger.Warn("", zap.Error(err))
			} else if dets := detailsMap[lastAssistantID]; len(dets) > 0 {
				extra := b.formatProcessDetailsForAttackChain(dets)
				if strings.TrimSpace(extra) != "" {
					reactInputFinal = reactInputFinal + "\n\n## （）\n\n" + extra
					b.logger.Info("",
						zap.String("conversationId", conversationID),
						zap.String("messageId", lastAssistantID),
						zap.Int("detailEvents", len(dets)))
				}
			}
		}
	}

	// English note.
	prompt := b.buildSimplePrompt(reactInputFinal, modelOutput)
	// fmt.Println(prompt)
	// English note.
	chainJSON, err := b.callAIForChainGeneration(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI: %w", err)
	}

	// English note.
	chainData, err := b.parseChainJSON(chainJSON)
	if err != nil {
		// English note.
		b.logger.Warn("JSON", zap.Error(err), zap.String("raw_json", chainJSON))
		return &Chain{
			Nodes: []Node{},
			Edges: []Edge{},
		}, nil
	}

	b.logger.Info("",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("nodes", len(chainData.Nodes)),
		zap.Int("edges", len(chainData.Edges)))

	// English note.
	if err := b.saveChain(conversationID, chainData.Nodes, chainData.Edges); err != nil {
		b.logger.Warn("", zap.Error(err))
		// English note.
	}

	// English note.
	return chainData, nil
}

// English note.
func reactInputContainsToolTrace(reactInputJSON string) bool {
	s := strings.TrimSpace(reactInputJSON)
	if s == "" {
		return false
	}
	return strings.Contains(s, "tool_calls") ||
		strings.Contains(s, "tool_call_id") ||
		strings.Contains(s, `"role":"tool"`) ||
		strings.Contains(s, `"role": "tool"`)
}

// English note.
func (b *Builder) formatProcessDetailsForAttackChain(details []database.ProcessDetail) string {
	if len(details) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, d := range details {
		// English note.
		// English note.
		// English note.
		if d.EventType == "progress" || d.EventType == "thinking" || d.EventType == "planning" {
			continue
		}

		// English note.
		var dataMap map[string]interface{}
		if strings.TrimSpace(d.Data) != "" {
			_ = json.Unmarshal([]byte(d.Data), &dataMap)
		}
		einoRole := ""
		if v, ok := dataMap["einoRole"]; ok {
			einoRole = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		}
		toolName := ""
		if v, ok := dataMap["toolName"]; ok {
			toolName = strings.TrimSpace(fmt.Sprint(v))
		}

		// English note.
		if (d.EventType == "tool_call" || d.EventType == "tool_result" || d.EventType == "tool_calls_detected" || d.EventType == "iteration" || d.EventType == "eino_recovery") && einoRole == "orchestrator" {
			sb.WriteString("[")
			sb.WriteString(d.EventType)
			sb.WriteString("] ")
			sb.WriteString(strings.TrimSpace(d.Message))
			sb.WriteString("\n")
			if strings.TrimSpace(d.Data) != "" {
				sb.WriteString(d.Data)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
			continue
		}

		// English note.
		if d.EventType == "tool_call" && strings.EqualFold(toolName, "task") {
			sb.WriteString("[dispatch_subagent_task] ")
			sb.WriteString(strings.TrimSpace(d.Message))
			sb.WriteString("\n")
			if strings.TrimSpace(d.Data) != "" {
				sb.WriteString(d.Data)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
			continue
		}

		// English note.
		if d.EventType == "eino_agent_reply" && einoRole == "sub" {
			sb.WriteString("[subagent_final_reply] ")
			sb.WriteString(strings.TrimSpace(d.Message))
			sb.WriteString("\n")
			// English note.
			if strings.TrimSpace(d.Data) != "" {
				sb.WriteString(d.Data)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
			continue
		}

		// English note.
	}
	return strings.TrimSpace(sb.String())
}

// English note.
func (b *Builder) buildReActInput(messages []database.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
	}
	return builder.String()
}

// English note.
// func (b *Builder) extractUserInputFromReActInput(reactInputJSON string) string {
// English note.
// 	var messages []map[string]interface{}
// 	if err := json.Unmarshal([]byte(reactInputJSON), &messages); err != nil {
// English note.
// 		return ""
// 	}

// English note.
// 	for i := len(messages) - 1; i >= 0; i-- {
// 		if role, ok := messages[i]["role"].(string); ok && strings.EqualFold(role, "user") {
// 			if content, ok := messages[i]["content"].(string); ok {
// 				return content
// 			}
// 		}
// 	}

// 	return ""
// }

// English note.
func (b *Builder) formatReActInputFromJSON(reactInputJSON string) string {
	var messages []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messages); err != nil {
		b.logger.Warn("ReActJSON", zap.Error(err))
		return reactInputJSON // ，JSON
	}

	var builder strings.Builder
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// English note.
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
				// English note.
				if content != "" {
					builder.WriteString(fmt.Sprintf("[%s]: %s\n", role, content))
				}
				// English note.
				builder.WriteString(fmt.Sprintf("[%s]  (%d):\n", role, len(toolCalls)))
				for i, toolCall := range toolCalls {
					if tc, ok := toolCall.(map[string]interface{}); ok {
						toolCallID, _ := tc["id"].(string)
						if funcData, ok := tc["function"].(map[string]interface{}); ok {
							toolName, _ := funcData["name"].(string)
							arguments, _ := funcData["arguments"].(string)
							builder.WriteString(fmt.Sprintf("  [ %d]\n", i+1))
							builder.WriteString(fmt.Sprintf("    ID: %s\n", toolCallID))
							builder.WriteString(fmt.Sprintf("    : %s\n", toolName))
							builder.WriteString(fmt.Sprintf("    : %s\n", arguments))
						}
					}
				}
				builder.WriteString("\n")
				continue
			}
		}

		// English note.
		if role == "tool" {
			toolCallID, _ := msg["tool_call_id"].(string)
			if toolCallID != "" {
				builder.WriteString(fmt.Sprintf("[%s] (tool_call_id: %s):\n%s\n\n", role, toolCallID, content))
			} else {
				builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
			}
			continue
		}

		// English note.
		builder.WriteString(fmt.Sprintf("[%s]: %s\n\n", role, content))
	}

	return builder.String()
}

// English note.
func (b *Builder) buildSimplePrompt(reactInput, modelOutput string) string {
	return fmt.Sprintf(`。，、，。

# English note.

：
1. （）
2. 
3. 
4. 

* English note.

# English note.

# English note.
ReAct，：
- （IP、、URL）
- 
- （、、）
- AI

# English note.
，****：
- **target**：target
- **action**：action（、、）
- **vulnerability**：vulnerability
- ****：ReAct，

# English note.
* English note.
，（agent，）：
- ****：（：，）
- ****：（：）
- actionaction
- vulnerabilityaction
- 
- ****：，

# English note.
- ****：，
- ****：action（）
- ****：（、、）
- ****：，。
- ，

# English note.

# English note.
- ****：
- ****：（IP/）target
- ****：，
- **metadata.target**：（IP、、URL）

# English note.
- ****：AI
- ****：
  * English note.
  * English note.
  * English note.
- **ai_analysis**：
  * English note.
  * English note.
  * English note.
- **findings**：
  * English note.
  * English note.
  * English note.
  * English note.
- **status**：
  * English note.
  * English note.
- **risk_score**：0（action）

# English note.
- ****：
- ****：
  * English note.
  * English note.
- **risk_score**：
  * English note.
  * English note.
  * English note.
  * English note.
- **metadata**：
  * English note.
  * English note.
  * severity：critical/high/medium/low
  * English note.

# English note.

# English note.
，：
- （、、）
- （、）
- WAF/（403、406，）
- （）
- （DNS、）

# English note.
：
- 
- （，）
- （）

# English note.
：
- （nmap，""）
- （，""）

# English note.
- ****：，
- ****：8-15，，（20）
- ****：、、、
- ****：（nmap，""）
- ****：、、（）
- ****：，。

# English note.

# English note.
- **leads_to**：""""，action→action、target→action
  * English note.
- **discovers**：""，**action→vulnerability**
  * English note.
  * English note.
- **enables**：""""，**vulnerability→vulnerability、action→action（）**
  * English note.
  * English note.

# English note.
- **1-2**：（）
- **3-4**：（）
- **5-7**：（、）
- **8-10**：（、）

# English note.
* English note.

- ****：id"node_1"（node_1, node_2, node_3...）
- ****：sourceidtargetid（source < target），
  * English note.
  * English note.
  * English note.
- ****：JSON，，source >= target
- ****：（）
- **DAG**：
  * English note.
  * English note.
  * English note.
- ****：id，（），

# English note.

：
1. ****：？（target）
2. ****：？（action）
3. ****：？（failed_insight）
4. ****：？（actionfindings）
5. ****：？（action→vulnerability）
6. ****：？（targetvulnerability）

# English note.

%s

# English note.

%s

# English note.

JSON，：

* English note.

{
   "nodes": [
     {
       "id": "node_1",
       "type": "target",
       "label": ": example.com",
       "risk_score": 40,
       "metadata": {
         "target": "example.com"
       }
     },
     {
       "id": "node_2",
       "type": "action",
       "label": "80/443/8080",
       "risk_score": 0,
       "metadata": {
         "tool_name": "nmap",
         "tool_intent": "",
         "ai_analysis": "nmap，80、443、8080。80HTTP，443HTTPS，8080。Web。",
         "findings": ["80", "443", "8080", "HTTPApache 2.4"]
       }
     },
     {
       "id": "node_3",
       "type": "action",
       "label": "/admin",
       "risk_score": 0,
       "metadata": {
         "tool_name": "dirsearch",
         "tool_intent": "",
         "ai_analysis": "dirsearch，/admin。，。",
         "findings": ["/admin", "200", ""]
       }
     },
     {
       "id": "node_4",
       "type": "action",
       "label": "WebApache 2.4",
       "risk_score": 0,
       "metadata": {
         "tool_name": "whatweb",
         "tool_intent": "Web",
         "ai_analysis": "Apache 2.4，。",
         "findings": ["Apache 2.4", "PHP"]
       }
     },
     {
       "id": "node_5",
       "type": "action",
       "label": "SQL（WAF）",
       "risk_score": 0,
       "metadata": {
         "tool_name": "sqlmap",
         "tool_intent": "SQL",
         "ai_analysis": "/login.phpSQLWAF，403。Cloudflare。WAF，。",
         "findings": ["WAF", "403", "Cloudflare", "WAF"],
         "status": "failed_insight"
       }
     },
     {
       "id": "node_6",
       "type": "vulnerability",
       "label": "SQL",
       "risk_score": 85,
       "metadata": {
         "vulnerability_type": "SQL",
         "description": "/admin/login.phpusernameSQL，payload，。，。",
         "severity": "high",
         "location": "/admin/login.php?username="
       }
     }
   ],
   "edges": [
     {
       "source": "node_1",
       "target": "node_2",
       "type": "leads_to",
       "weight": 3
     },
     {
       "source": "node_2",
       "target": "node_3",
       "type": "leads_to",
       "weight": 4
     },
     {
       "source": "node_2",
       "target": "node_4",
       "type": "leads_to",
       "weight": 3
     },
     {
       "source": "node_3",
       "target": "node_5",
       "type": "leads_to",
       "weight": 4
     },
     {
       "source": "node_5",
       "target": "node_6",
       "type": "discovers",
       "weight": 7
     }
   ]
}

# English note.

1. ****：ReAct。，nodesedges。
2. **DAG**：DAG（），。sourceidtargetid（source < target）。
3. ****：，targetnode_1，action，vulnerability。
4. ****：，。。
5. ****：、，。
6. ****：，。
7. ****：，。
8. ****：metadata，sourcetarget，，。
9. ****：，（20），。
10. ****：JSON，source < target，DAG。

：`, reactInput, modelOutput)
}

// English note.
func (b *Builder) saveChain(conversationID string, nodes []Node, edges []Edge) error {
	// English note.
	if err := b.db.DeleteAttackChain(conversationID); err != nil {
		b.logger.Warn("", zap.Error(err))
	}

	for _, node := range nodes {
		metadataJSON, _ := json.Marshal(node.Metadata)
		if err := b.db.SaveAttackChainNode(conversationID, node.ID, node.Type, node.Label, "", string(metadataJSON), node.RiskScore); err != nil {
			b.logger.Warn("", zap.String("nodeId", node.ID), zap.Error(err))
		}
	}

	// English note.
	for _, edge := range edges {
		if err := b.db.SaveAttackChainEdge(conversationID, edge.ID, edge.Source, edge.Target, edge.Type, edge.Weight); err != nil {
			b.logger.Warn("", zap.String("edgeId", edge.ID), zap.Error(err))
		}
	}

	return nil
}

// English note.
func (b *Builder) LoadChainFromDatabase(conversationID string) (*Chain, error) {
	nodes, err := b.db.LoadAttackChainNodes(conversationID)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	edges, err := b.db.LoadAttackChainEdges(conversationID)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return &Chain{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// English note.
func (b *Builder) callAIForChainGeneration(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": b.openAIConfig.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "，。JSON。",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3,
		"max_tokens":  8000,
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if b.openAIClient == nil {
		return "", fmt.Errorf("OpenAI")
	}
	if err := b.openAIClient.ChatCompletion(ctx, requestBody, &apiResponse); err != nil {
		var apiErr *openai.APIError
		if errors.As(err, &apiErr) {
			bodyStr := strings.ToLower(apiErr.Body)
			if strings.Contains(bodyStr, "context") || strings.Contains(bodyStr, "length") || strings.Contains(bodyStr, "too long") {
				return "", fmt.Errorf("context length exceeded")
			}
		} else if strings.Contains(strings.ToLower(err.Error()), "context") || strings.Contains(strings.ToLower(err.Error()), "length") {
			return "", fmt.Errorf("context length exceeded")
		}
		return "", fmt.Errorf(": %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API")
	}

	content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
	// English note.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return content, nil
}

// English note.
type ChainJSON struct {
	Nodes []struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Label     string                 `json:"label"`
		RiskScore int                    `json:"risk_score"`
		Metadata  map[string]interface{} `json:"metadata"`
	} `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
		Type   string `json:"type"`
		Weight int    `json:"weight"`
	} `json:"edges"`
}

// English note.
func (b *Builder) parseChainJSON(chainJSON string) (*Chain, error) {
	var chainData ChainJSON
	if err := json.Unmarshal([]byte(chainJSON), &chainData); err != nil {
		return nil, fmt.Errorf("JSON: %w", err)
	}

	// English note.
	nodeIDMap := make(map[string]string)

	// English note.
	nodes := make([]Node, 0, len(chainData.Nodes))
	for _, n := range chainData.Nodes {
		// English note.
		newNodeID := fmt.Sprintf("node_%s", uuid.New().String())
		nodeIDMap[n.ID] = newNodeID

		node := Node{
			ID:        newNodeID,
			Type:      n.Type,
			Label:     n.Label,
			RiskScore: n.RiskScore,
			Metadata:  n.Metadata,
		}
		if node.Metadata == nil {
			node.Metadata = make(map[string]interface{})
		}
		nodes = append(nodes, node)
	}

	// English note.
	edges := make([]Edge, 0, len(chainData.Edges))
	for _, e := range chainData.Edges {
		sourceID, ok := nodeIDMap[e.Source]
		if !ok {
			continue
		}
		targetID, ok := nodeIDMap[e.Target]
		if !ok {
			continue
		}

		// English note.
		edgeID := fmt.Sprintf("edge_%s", uuid.New().String())

		edges = append(edges, Edge{
			ID:     edgeID,
			Source: sourceID,
			Target: targetID,
			Type:   e.Type,
			Weight: e.Weight,
		})
	}

	return &Chain{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// English note.
