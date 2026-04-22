package einomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/security"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

// English note.
type ExecutionRecorder func(executionID string)

// English note.
// English note.
const ToolErrorPrefix = "__CYBERSTRIKE_AI_TOOL_ERROR__\n"

// English note.
func ToolsFromDefinitions(
	ag *agent.Agent,
	holder *ConversationHolder,
	defs []agent.Tool,
	rec ExecutionRecorder,
	toolOutputChunk func(toolName, toolCallID, chunk string),
) ([]tool.BaseTool, error) {
	out := make([]tool.BaseTool, 0, len(defs))
	for _, d := range defs {
		if d.Type != "function" || d.Function.Name == "" {
			continue
		}
		info, err := toolInfoFromDefinition(d)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", d.Function.Name, err)
		}
		out = append(out, &mcpBridgeTool{
			info:   info,
			name:   d.Function.Name,
			agent:  ag,
			holder: holder,
			record: rec,
			chunk:  toolOutputChunk,
		})
	}
	return out, nil
}

func toolInfoFromDefinition(d agent.Tool) (*schema.ToolInfo, error) {
	fn := d.Function
	raw, err := json.Marshal(fn.Parameters)
	if err != nil {
		return nil, err
	}
	var js jsonschema.Schema
	if len(raw) > 0 && string(raw) != "null" && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &js); err != nil {
			return nil, err
		}
	}
	if js.Type == "" {
		js.Type = string(schema.Object)
	}
	if js.Properties == nil && js.Type == string(schema.Object) {
		// English note.
	}
	return &schema.ToolInfo{
		Name:        fn.Name,
		Desc:        fn.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&js),
	}, nil
}

type mcpBridgeTool struct {
	info   *schema.ToolInfo
	name   string
	agent  *agent.Agent
	holder *ConversationHolder
	record ExecutionRecorder
	chunk  func(toolName, toolCallID, chunk string)
}

func (m *mcpBridgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	_ = ctx
	return m.info, nil
}

func (m *mcpBridgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	_ = opts
	return runMCPToolInvocation(ctx, m.agent, m.holder, m.name, argumentsInJSON, m.record, m.chunk)
}

// English note.
func runMCPToolInvocation(
	ctx context.Context,
	ag *agent.Agent,
	holder *ConversationHolder,
	toolName string,
	argumentsInJSON string,
	record ExecutionRecorder,
	chunk func(toolName, toolCallID, chunk string),
) (string, error) {
	var args map[string]interface{}
	if argumentsInJSON != "" && argumentsInJSON != "null" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			// Return soft error (nil error) so the eino graph continues and the LLM can self-correct,
			// instead of a hard error that terminates the iteration loop.
			return ToolErrorPrefix + fmt.Sprintf(
				"Invalid tool arguments JSON: %s\n\nPlease ensure the arguments are a valid JSON object "+
					"(double-quoted keys, matched braces, no trailing commas) and retry.\n\n"+
					"（ JSON ：%s。 arguments  JSON 。）",
				err.Error(), err.Error()), nil
		}
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	if chunk != nil {
		toolCallID := compose.GetToolCallID(ctx)
		if toolCallID != "" {
			if existing, ok := ctx.Value(security.ToolOutputCallbackCtxKey).(security.ToolOutputCallback); ok && existing != nil {
				ctx = context.WithValue(ctx, security.ToolOutputCallbackCtxKey, security.ToolOutputCallback(func(c string) {
					existing(c)
					if strings.TrimSpace(c) == "" {
						return
					}
					chunk(toolName, toolCallID, c)
				}))
			} else {
				ctx = context.WithValue(ctx, security.ToolOutputCallbackCtxKey, security.ToolOutputCallback(func(c string) {
					if strings.TrimSpace(c) == "" {
						return
					}
					chunk(toolName, toolCallID, c)
				}))
			}
		}
	}

	res, err := ag.ExecuteMCPToolForConversation(ctx, holder.Get(), toolName, args)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", nil
	}
	if res.ExecutionID != "" && record != nil {
		record(res.ExecutionID)
	}
	if res.IsError {
		return ToolErrorPrefix + res.Result, nil
	}
	return res.Result, nil
}

// English note.
// English note.
// English note.
// English note.
func UnknownToolReminderHandler() func(ctx context.Context, name, input string) (string, error) {
	return func(ctx context.Context, name, input string) (string, error) {
		_ = ctx
		_ = input
		requested := strings.TrimSpace(name)
		// Return a recoverable error that still carries a friendly, bilingual hint.
		// This will be caught by multiagent runner as "tool not found" and trigger a retry.
		return "", fmt.Errorf("tool %q not found: %s", requested, unknownToolReminderText(requested))
	}
}

func unknownToolReminderText(requested string) string {
	if requested == "" {
		requested = "(empty)"
	}
	return fmt.Sprintf(`The tool name %q is not registered for this agent.

Please retry using only names that appear in the tool definitions for this turn (exact match, case-sensitive). Do not invent or rename tools; adjust your plan and continue.

（ %q ：，；，。）`, requested, requested)
}
