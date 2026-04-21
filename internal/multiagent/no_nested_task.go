package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

// English note.
// English note.
//
// English note.
// English note.
type noNestedTaskMiddleware struct {
	adk.BaseChatModelAgentMiddleware
}

type nestedTaskCtxKey struct{}

func newNoNestedTaskMiddleware() adk.ChatModelAgentMiddleware {
	return &noNestedTaskMiddleware{}
}

func (m *noNestedTaskMiddleware) WrapInvokableToolCall(
	ctx context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	if tCtx == nil || strings.TrimSpace(tCtx.Name) == "" {
		return endpoint, nil
	}
	// English note.
	if !strings.EqualFold(strings.TrimSpace(tCtx.Name), "task") {
		return endpoint, nil
	}

	// English note.
	if ctx != nil {
		if v, ok := ctx.Value(nestedTaskCtxKey{}).(bool); ok && v {
			return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
				// Important: return a tool result text (not an error) to avoid hard-stopping the whole multi-agent run.
				// The nested task is still prevented from spawning another sub-agent, so recursion is avoided.
				_ = argumentsInJSON
				_ = opts
				return "Nested task delegation is forbidden (already inside a sub-agent delegation chain) to avoid infinite delegation. Please continue the work using the current agent's tools.", nil
			}, nil
		}
	}

	// English note.
	return func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		ctx2 := ctx
		if ctx2 == nil {
			ctx2 = context.Background()
		}
		ctx2 = context.WithValue(ctx2, nestedTaskCtxKey{}, true)
		return endpoint(ctx2, argumentsInJSON, opts...)
	}, nil
}

