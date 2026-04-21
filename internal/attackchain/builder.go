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
	maxTokens    int // 最大tokens限制，默认100000
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
			maxTokens = 128000 // gpt-4通常支持128k
		} else if strings.Contains(model, "gpt-3.5") {
			maxTokens = 16000 // gpt-3.5-turbo通常支持16k
		} else if strings.Contains(model, "deepseek") {
			maxTokens = 131072 // deepseek-chat通常支持131k
		} else {
			maxTokens = 100000 // 兜底默认值
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
	b.logger.Info("开始构建攻击链（简化版本）", zap.String("conversationId", conversationID))

	// English note.
	messages, err := b.db.GetMessages(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取对话消息失败: %w", err)
	}

	if len(messages) == 0 {
		b.logger.Info("对话中没有数据", zap.String("conversationId", conversationID))
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
			b.logger.Warn("查询过程详情判定工具执行失败", zap.Error(err))
		} else if pdOK {
			hasToolExecutions = true
		}
	}

	// English note.
	taskCancelled := false
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") {
			content := strings.ToLower(messages[i].Content)
			if strings.Contains(content, "取消") || strings.Contains(content, "cancelled") {
				taskCancelled = true
			}
			break
		}
	}

	// English note.
	if taskCancelled && !hasToolExecutions {
		b.logger.Info("任务已取消且没有实际工具执行，返回空攻击链",
			zap.String("conversationId", conversationID),
			zap.Bool("taskCancelled", taskCancelled),
			zap.Bool("hasToolExecutions", hasToolExecutions))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// English note.
	if !hasToolExecutions {
		b.logger.Info("没有实际工具执行记录，返回空攻击链",
			zap.String("conversationId", conversationID))
		return &Chain{Nodes: []Node{}, Edges: []Edge{}}, nil
	}

	// English note.
	reactInputJSON, modelOutput, err := b.db.GetReActData(conversationID)
	if err != nil {
		b.logger.Warn("获取保存的ReAct数据失败，将使用消息历史构建", zap.Error(err))
		// English note.
		reactInputJSON = ""
		modelOutput = ""
	}

	// var userInput string
	var reactInputFinal string
	var dataSource string // 记录数据来源

	// English note.
	if reactInputJSON != "" && modelOutput != "" {
		// English note.
		hash := sha256.Sum256([]byte(reactInputJSON))
		reactInputHash := hex.EncodeToString(hash[:])[:16] // 使用前16字符作为短标识

		// English note.
		var messageCount int
		var tempMessages []interface{}
		if json.Unmarshal([]byte(reactInputJSON), &tempMessages) == nil {
			messageCount = len(tempMessages)
		}

		dataSource = "database_last_react_input"
		b.logger.Info("使用保存的ReAct数据构建攻击链",
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
		b.logger.Info("从消息历史构建ReAct数据",
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
				b.logger.Warn("加载过程详情用于攻击链失败", zap.Error(err))
			} else if dets := detailsMap[lastAssistantID]; len(dets) > 0 {
				extra := b.formatProcessDetailsForAttackChain(dets)
				if strings.TrimSpace(extra) != "" {
					reactInputFinal = reactInputFinal + "\n\n## 执行过程与工具记录（含多代理编排与子任务）\n\n" + extra
					b.logger.Info("攻击链输入已补充过程详情",
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
		return nil, fmt.Errorf("AI生成失败: %w", err)
	}

	// English note.
	chainData, err := b.parseChainJSON(chainJSON)
	if err != nil {
		// English note.
		b.logger.Warn("解析攻击链JSON失败", zap.Error(err), zap.String("raw_json", chainJSON))
		return &Chain{
			Nodes: []Node{},
			Edges: []Edge{},
		}, nil
	}

	b.logger.Info("攻击链构建完成",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("nodes", len(chainData.Nodes)),
		zap.Int("edges", len(chainData.Edges)))

	// English note.
	if err := b.saveChain(conversationID, chainData.Nodes, chainData.Edges); err != nil {
		b.logger.Warn("保存攻击链到数据库失败", zap.Error(err))
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
		b.logger.Warn("解析ReAct输入JSON失败", zap.Error(err))
		return reactInputJSON // 如果解析失败，返回原始JSON
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
				builder.WriteString(fmt.Sprintf("[%s] 工具调用 (%d个):\n", role, len(toolCalls)))
				for i, toolCall := range toolCalls {
					if tc, ok := toolCall.(map[string]interface{}); ok {
						toolCallID, _ := tc["id"].(string)
						if funcData, ok := tc["function"].(map[string]interface{}); ok {
							toolName, _ := funcData["name"].(string)
							arguments, _ := funcData["arguments"].(string)
							builder.WriteString(fmt.Sprintf("  [工具调用 %d]\n", i+1))
							builder.WriteString(fmt.Sprintf("    ID: %s\n", toolCallID))
							builder.WriteString(fmt.Sprintf("    工具名称: %s\n", toolName))
							builder.WriteString(fmt.Sprintf("    参数: %s\n", arguments))
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
	return fmt.Sprintf(`你是专业的安全测试分析师和攻击链构建专家。你的任务是根据对话记录和工具执行结果，构建一个逻辑清晰、有教育意义的攻击链图，完整展现渗透测试的思维过程和执行路径。

# English note.

构建一个能够讲述完整攻击故事的攻击链让学习者能够：
1. 理解渗透测试的完整流程和思维逻辑（从目标识别到漏洞发现的每一步）
2. 学习如何从失败中获取线索并调整策略
3. 掌握工具使用的实际效果和局限性
4. 理解漏洞发现和利用的因果关系

* English note.

# English note.

# English note.
仔细分析ReAct输入中的工具调用序列和大模型输出，识别：
- 测试目标（IP、域名、URL等）
- 实际执行的工具和参数
- 工具返回的关键信息（成功结果、错误信息、超时等）
- AI的分析和决策过程

# English note.
从工具执行记录中提取有意义的节点，**确保不遗漏任何关键步骤**：
- **target节点**：每个独立的测试目标创建一个target节点
- **action节点**：每个有意义的工具执行创建一个action节点（包括提供线索的失败、成功的信息收集、漏洞验证等）
- **vulnerability节点**：每个真实确认的漏洞创建一个vulnerability节点
- **完整性检查**：对照ReAct输入中的工具调用序列，确保每个有意义的工具执行都被包含在攻击链中

# English note.
* English note.
按照因果关系连接节点，形成树状图（因为是单agent执行，所以可以不按照时间顺序）：
- **分支结构**：一个节点可以有多个后续节点（例如：端口扫描发现多个端口后，可以同时进行多个不同的测试）
- **汇聚结构**：多个节点可以指向同一个节点（例如：多个不同的测试都发现了同一个漏洞）
- 识别哪些action是基于前面action的结果而执行的
- 识别哪些vulnerability是由哪些action发现的
- 识别失败节点如何为后续成功提供线索
- **避免线性链**：不要将所有节点连成一条线，应该根据实际的并行测试和分支探索构建树状结构

# English note.
- **完整性检查**：确保所有有意义的工具执行都被包含，不要遗漏关键步骤
- **合并规则**：只合并真正相似或重复的action节点（如多次相同工具的相似调用）
- **删除规则**：只删除完全无价值的失败节点（完全无输出、纯系统错误、重复的相同失败）
- **重要提醒**：宁可保留更多节点，也不要遗漏关键步骤。攻击链必须完整展现渗透测试过程
- 确保攻击链逻辑连贯，能够讲述完整故事

# English note.

# English note.
- **用途**：标识测试目标
- **创建规则**：每个独立目标（不同IP/域名）创建一个target节点
- **多目标处理**：不同目标的节点不相互连接，各自形成独立的子图
- **metadata.target**：精确记录目标标识（IP地址、域名、URL等）

# English note.
- **用途**：记录工具执行和AI分析结果
- **标签规则**：
  * English note.
  * English note.
  * English note.
- **ai_analysis要求**：
  * English note.
  * English note.
  * English note.
- **findings要求**：
  * English note.
  * English note.
  * English note.
  * English note.
- **status标记**：
  * English note.
  * English note.
- **risk_score**：始终为0（action节点不评估风险）

# English note.
- **用途**：记录真实确认的安全漏洞
- **创建规则**：
  * English note.
  * English note.
- **risk_score规则**：
  * English note.
  * English note.
  * English note.
  * English note.
- **metadata要求**：
  * English note.
  * English note.
  * severity：critical/high/medium/low
  * English note.

# English note.

# English note.
以下失败情况必须创建节点，因为它们提供了有价值的线索：
- 工具返回明确的错误信息（权限错误、连接拒绝、认证失败等）
- 超时或连接失败（可能表明防火墙、网络隔离等）
- WAF/防火墙拦截（返回403、406等，表明存在防护机制）
- 工具未安装或配置错误（但执行了调用）
- 目标不可达（DNS解析失败、网络不通等）

# English note.
以下情况不应创建节点：
- 完全无输出的工具调用
- 纯系统错误（与目标无关，如本地环境问题）
- 重复的相同失败（多次相同错误只保留第一次）

# English note.
以下情况应合并节点：
- 同一工具的多次相似调用（如多次nmap扫描不同端口范围，合并为一个"端口扫描"节点）
- 同一目标的多个相似探测（如多个目录扫描工具，合并为一个"目录扫描"节点）

# English note.
- **完整性优先**：必须包含所有有意义的工具执行和关键步骤，不要为了控制数量而删除重要节点
- **建议范围**：单目标通常8-15个节点，但如果实际执行步骤较多，可以适当增加（最多20个节点）
- **优先保留**：关键成功步骤、提供线索的失败、发现的漏洞、重要的信息收集步骤
- **可以合并**：同一工具的多次相似调用（如多次nmap扫描不同端口范围，合并为一个"端口扫描"节点）
- **可以删除**：完全无输出的工具调用、纯系统错误、重复的相同失败（多次相同错误只保留第一次）
- **重要原则**：宁可节点稍多，也不要遗漏关键步骤。攻击链必须能够完整展现渗透测试的完整过程

# English note.

# English note.
- **leads_to**：表示"导致"或"引导到"，用于action→action、target→action
  * English note.
- **discovers**：表示"发现"，**专门用于action→vulnerability**
  * English note.
  * English note.
- **enables**：表示"使能"或"促成"，**仅用于vulnerability→vulnerability、action→action（当后续行动依赖前面结果时）**
  * English note.
  * English note.

# English note.
- **权重1-2**：弱关联（如初步探测到进一步探测）
- **权重3-4**：中等关联（如发现端口到服务识别）
- **权重5-7**：强关联（如发现漏洞、关键信息泄露）
- **权重8-10**：极强关联（如漏洞利用成功、权限提升）

# English note.
* English note.

- **节点编号规则**：节点id从"node_1"开始递增（node_1, node_2, node_3...）
- **边的方向规则**：所有边的source节点id必须严格小于target节点id（source < target），这是确保无环的关键
  * English note.
  * English note.
  * English note.
- **无环验证**：在输出JSON前，必须检查所有边，确保没有任何一条边的source >= target
- **无孤立节点**：确保每个节点至少有一条边连接（除了可能的根节点）
- **DAG结构特点**：
  * English note.
  * English note.
  * English note.
- **拓扑排序验证**：如果按照节点id从小到大排序，所有边都应该从左指向右（从上指向下），这样就能保证无环

# English note.

构建的攻击链应该能够回答以下问题：
1. **起点**：测试从哪里开始？（target节点）
2. **探索过程**：如何逐步收集信息？（action节点序列）
3. **失败与调整**：遇到障碍时如何调整策略？（failed_insight节点）
4. **关键发现**：发现了哪些重要信息？（action的findings）
5. **漏洞确认**：如何确认漏洞存在？（action→vulnerability）
6. **攻击路径**：完整的攻击路径是什么？（从target到vulnerability的路径）

# English note.

%s

# English note.

%s

# English note.

严格按照以下JSON格式输出，不要添加任何其他文字：

* English note.

{
   "nodes": [
     {
       "id": "node_1",
       "type": "target",
       "label": "测试目标: example.com",
       "risk_score": 40,
       "metadata": {
         "target": "example.com"
       }
     },
     {
       "id": "node_2",
       "type": "action",
       "label": "扫描端口发现80/443/8080",
       "risk_score": 0,
       "metadata": {
         "tool_name": "nmap",
         "tool_intent": "端口扫描",
         "ai_analysis": "使用nmap对目标进行端口扫描，发现80、443、8080端口开放。80端口运行HTTP服务，443端口运行HTTPS服务，8080端口可能为管理后台。这些开放端口为后续Web应用测试提供了入口。",
         "findings": ["80端口开放", "443端口开放", "8080端口开放", "HTTP服务为Apache 2.4"]
       }
     },
     {
       "id": "node_3",
       "type": "action",
       "label": "目录扫描发现/admin后台",
       "risk_score": 0,
       "metadata": {
         "tool_name": "dirsearch",
         "tool_intent": "目录扫描",
         "ai_analysis": "使用dirsearch对目标进行目录扫描，发现/admin目录存在且可访问。该目录可能为管理后台，是重要的测试目标。",
         "findings": ["/admin目录存在", "返回200状态码", "疑似管理后台"]
       }
     },
     {
       "id": "node_4",
       "type": "action",
       "label": "识别Web服务为Apache 2.4",
       "risk_score": 0,
       "metadata": {
         "tool_name": "whatweb",
         "tool_intent": "Web服务识别",
         "ai_analysis": "识别出目标运行Apache 2.4服务器，这为后续的漏洞测试提供了重要信息。",
         "findings": ["Apache 2.4", "PHP版本信息"]
       }
     },
     {
       "id": "node_5",
       "type": "action",
       "label": "尝试SQL注入（被WAF拦截）",
       "risk_score": 0,
       "metadata": {
         "tool_name": "sqlmap",
         "tool_intent": "SQL注入检测",
         "ai_analysis": "对/login.php进行SQL注入测试时被WAF拦截，返回403错误。错误信息显示检测到Cloudflare防护。这表明目标部署了WAF，需要调整测试策略。",
         "findings": ["WAF拦截", "返回403", "检测到Cloudflare", "目标部署WAF"],
         "status": "failed_insight"
       }
     },
     {
       "id": "node_6",
       "type": "vulnerability",
       "label": "SQL注入漏洞",
       "risk_score": 85,
       "metadata": {
         "vulnerability_type": "SQL注入",
         "description": "在/admin/login.php的username参数发现SQL注入漏洞，可通过注入payload绕过登录验证，直接获取管理员权限。漏洞返回数据库错误信息，确认存在注入点。",
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

1. **严禁杜撰**：只使用ReAct输入中实际执行的工具和实际返回的结果。如无实际数据，返回空的nodes和edges数组。
2. **DAG结构必须**：必须构建真正的DAG（有向无环图），不能有任何循环。所有边的source节点id必须严格小于target节点id（source < target）。
3. **拓扑顺序**：节点应该按照逻辑顺序编号，target节点通常是node_1，后续的action节点按执行顺序递增，vulnerability节点在最后。
4. **完整性优先**：必须包含所有有意义的工具执行和关键步骤，不要为了控制节点数量而删除重要节点。攻击链必须能够完整展现从目标识别到漏洞发现的完整过程。
5. **逻辑连贯**：确保攻击链能够讲述一个完整、连贯的渗透测试故事，包括所有关键步骤和决策点。
6. **教育价值**：优先保留有教育意义的节点，帮助学习者理解渗透测试思维和完整流程。
7. **准确性**：所有节点信息必须基于实际数据，不要推测或假设。
8. **完整性检查**：确保每个节点都有必要的metadata字段，每条边都有正确的source和target，没有孤立节点，没有循环。
9. **不要过度精简**：如果实际执行步骤较多，可以适当增加节点数量（最多20个），确保不遗漏关键步骤。
10. **输出前验证**：在输出JSON前，必须验证所有边都满足source < target的条件，确保DAG结构正确。

现在开始分析并构建攻击链：`, reactInput, modelOutput)
}

// English note.
func (b *Builder) saveChain(conversationID string, nodes []Node, edges []Edge) error {
	// English note.
	if err := b.db.DeleteAttackChain(conversationID); err != nil {
		b.logger.Warn("删除旧攻击链失败", zap.Error(err))
	}

	for _, node := range nodes {
		metadataJSON, _ := json.Marshal(node.Metadata)
		if err := b.db.SaveAttackChainNode(conversationID, node.ID, node.Type, node.Label, "", string(metadataJSON), node.RiskScore); err != nil {
			b.logger.Warn("保存攻击链节点失败", zap.String("nodeId", node.ID), zap.Error(err))
		}
	}

	// English note.
	for _, edge := range edges {
		if err := b.db.SaveAttackChainEdge(conversationID, edge.ID, edge.Source, edge.Target, edge.Type, edge.Weight); err != nil {
			b.logger.Warn("保存攻击链边失败", zap.String("edgeId", edge.ID), zap.Error(err))
		}
	}

	return nil
}

// English note.
func (b *Builder) LoadChainFromDatabase(conversationID string) (*Chain, error) {
	nodes, err := b.db.LoadAttackChainNodes(conversationID)
	if err != nil {
		return nil, fmt.Errorf("加载攻击链节点失败: %w", err)
	}

	edges, err := b.db.LoadAttackChainEdges(conversationID)
	if err != nil {
		return nil, fmt.Errorf("加载攻击链边失败: %w", err)
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
				"content": "你是一个专业的安全测试分析师，擅长构建攻击链图。请严格按照JSON格式返回攻击链数据。",
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
		return "", fmt.Errorf("OpenAI客户端未初始化")
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
		return "", fmt.Errorf("请求失败: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("API未返回有效响应")
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
		return nil, fmt.Errorf("解析JSON失败: %w", err)
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
