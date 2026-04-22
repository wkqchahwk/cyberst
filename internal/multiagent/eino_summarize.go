package multiagent

import (
	"context"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// English note.
const einoSummarizeUserInstruction = `。

：、、、、、、。
（URL、、、Payload、、）。
；。
****：、/（+）、/，「」。

。`

// English note.
// English note.
func newEinoSummarizationMiddleware(
	ctx context.Context,
	summaryModel model.BaseChatModel,
	appCfg *config.Config,
	logger *zap.Logger,
) (adk.ChatModelAgentMiddleware, error) {
	if summaryModel == nil || appCfg == nil {
		return nil, fmt.Errorf("multiagent: summarization  model ")
	}
	maxTotal := appCfg.OpenAI.MaxTotalTokens
	if maxTotal <= 0 {
		maxTotal = 120000
	}
	trigger := int(float64(maxTotal) * 0.9)
	if trigger < 4096 {
		trigger = maxTotal
		if trigger < 4096 {
			trigger = 4096
		}
	}
	preserveMax := trigger / 3
	if preserveMax < 2048 {
		preserveMax = 2048
	}

	modelName := strings.TrimSpace(appCfg.OpenAI.Model)
	if modelName == "" {
		modelName = "gpt-4o"
	}

	mw, err := summarization.New(ctx, &summarization.Config{
		Model: summaryModel,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: trigger,
		},
		TokenCounter:       einoSummarizationTokenCounter(modelName),
		UserInstruction:    einoSummarizeUserInstruction,
		EmitInternalEvents: false,
		PreserveUserMessages: &summarization.PreserveUserMessages{
			Enabled:   true,
			MaxTokens: preserveMax,
		},
		Callback: func(ctx context.Context, before, after adk.ChatModelAgentState) error {
			if logger == nil {
				return nil
			}
			logger.Info("eino summarization ",
				zap.Int("messages_before", len(before.Messages)),
				zap.Int("messages_after", len(after.Messages)),
				zap.Int("max_total_tokens", maxTotal),
				zap.Int("trigger_context_tokens", trigger),
			)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("summarization.New: %w", err)
	}
	return mw, nil
}

func einoSummarizationTokenCounter(openAIModel string) summarization.TokenCounterFunc {
	tc := agent.NewTikTokenCounter()
	return func(ctx context.Context, input *summarization.TokenCounterInput) (int, error) {
		var sb strings.Builder
		for _, msg := range input.Messages {
			if msg == nil {
				continue
			}
			sb.WriteString(string(msg.Role))
			sb.WriteByte('\n')
			if msg.Content != "" {
				sb.WriteString(msg.Content)
				sb.WriteByte('\n')
			}
			if msg.ReasoningContent != "" {
				sb.WriteString(msg.ReasoningContent)
				sb.WriteByte('\n')
			}
			if len(msg.ToolCalls) > 0 {
				if b, err := sonic.Marshal(msg.ToolCalls); err == nil {
					sb.Write(b)
					sb.WriteByte('\n')
				}
			}
			for _, part := range msg.UserInputMultiContent {
				if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
					sb.WriteString(part.Text)
					sb.WriteByte('\n')
				}
			}
		}
		for _, tl := range input.Tools {
			if tl == nil {
				continue
			}
			cp := *tl
			cp.Extra = nil
			if text, err := sonic.MarshalString(cp); err == nil {
				sb.WriteString(text)
				sb.WriteByte('\n')
			}
		}
		text := sb.String()
		n, err := tc.Count(openAIModel, text)
		if err != nil {
			return (len(text) + 3) / 4, nil
		}
		return n, nil
	}
}
