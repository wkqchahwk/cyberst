package multiagent

import (
	"context"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/config"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// English note.
type PlanExecuteRootArgs struct {
	MainToolCallingModel *openai.ChatModel
	ExecModel            *openai.ChatModel
	OrchInstruction      string
	ToolsCfg             adk.ToolsConfig
	ExecMaxIter          int
	LoopMaxIter          int
	// English note.
	AppCfg *config.Config
	Logger *zap.Logger
	// English note.
	// English note.
	ExecPreMiddlewares []adk.ChatModelAgentMiddleware
	// English note.
	SkillMiddleware adk.ChatModelAgentMiddleware
	// English note.
	FilesystemMiddleware adk.ChatModelAgentMiddleware
}

// English note.
func NewPlanExecuteRoot(ctx context.Context, a *PlanExecuteRootArgs) (adk.ResumableAgent, error) {
	if a == nil {
		return nil, fmt.Errorf("plan_execute: args 为空")
	}
	if a.MainToolCallingModel == nil || a.ExecModel == nil {
		return nil, fmt.Errorf("plan_execute: 模型为空")
	}
	tcm, ok := interface{}(a.MainToolCallingModel).(model.ToolCallingChatModel)
	if !ok {
		return nil, fmt.Errorf("plan_execute: 主模型需实现 ToolCallingChatModel")
	}
	plannerCfg := &planexecute.PlannerConfig{
		ToolCallingChatModel: tcm,
	}
	if fn := planExecutePlannerGenInput(a.OrchInstruction); fn != nil {
		plannerCfg.GenInputFn = fn
	}
	planner, err := planexecute.NewPlanner(ctx, plannerCfg)
	if err != nil {
		return nil, fmt.Errorf("plan_execute planner: %w", err)
	}
	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel:  tcm,
		GenInputFn: planExecuteReplannerGenInput(a.OrchInstruction),
	})
	if err != nil {
		return nil, fmt.Errorf("plan_execute replanner: %w", err)
	}

	// English note.
	var execHandlers []adk.ChatModelAgentMiddleware
	// English note.
	if len(a.ExecPreMiddlewares) > 0 {
		execHandlers = append(execHandlers, a.ExecPreMiddlewares...)
	}
	// English note.
	if a.FilesystemMiddleware != nil {
		execHandlers = append(execHandlers, a.FilesystemMiddleware)
	}
	// English note.
	if a.SkillMiddleware != nil {
		execHandlers = append(execHandlers, a.SkillMiddleware)
	}
	// English note.
	if a.AppCfg != nil {
		sumMw, sumErr := newEinoSummarizationMiddleware(ctx, a.ExecModel, a.AppCfg, a.Logger)
		if sumErr != nil {
			return nil, fmt.Errorf("plan_execute executor summarization: %w", sumErr)
		}
		execHandlers = append(execHandlers, sumMw)
	}
	executor, err := newPlanExecuteExecutor(ctx, &planexecute.ExecutorConfig{
		Model:         a.ExecModel,
		ToolsConfig:   a.ToolsCfg,
		MaxIterations: a.ExecMaxIter,
		GenInputFn:    planExecuteExecutorGenInput(a.OrchInstruction),
	}, execHandlers)
	if err != nil {
		return nil, fmt.Errorf("plan_execute executor: %w", err)
	}
	loopMax := a.LoopMaxIter
	if loopMax <= 0 {
		loopMax = 10
	}
	return planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: loopMax,
	})
}

// English note.
// English note.
func planExecutePlannerGenInput(orchInstruction string) planexecute.GenPlannerModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	if oi == "" {
		return nil
	}
	return func(ctx context.Context, userInput []adk.Message) ([]adk.Message, error) {
		msgs := make([]adk.Message, 0, 1+len(userInput))
		msgs = append(msgs, schema.SystemMessage(oi))
		msgs = append(msgs, userInput...)
		return msgs, nil
	}
}

func planExecuteExecutorGenInput(orchInstruction string) planexecute.GenModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		planContent, err := in.Plan.MarshalJSON()
		if err != nil {
			return nil, err
		}
		userMsgs, err := planexecute.ExecutorPrompt.Format(ctx, map[string]any{
			"input":          planExecuteFormatInput(in.UserInput),
			"plan":           string(planContent),
			"executed_steps": planExecuteFormatExecutedSteps(in.ExecutedSteps),
			"step":           in.Plan.FirstStep(),
		})
		if err != nil {
			return nil, err
		}
		if oi != "" {
			userMsgs = append([]adk.Message{schema.SystemMessage(oi)}, userMsgs...)
		}
		return userMsgs, nil
	}
}

func planExecuteFormatInput(input []adk.Message) string {
	var sb strings.Builder
	for _, msg := range input {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

func planExecuteFormatExecutedSteps(results []planexecute.ExecutedStep) string {
	capped := capPlanExecuteExecutedSteps(results)
	var sb strings.Builder
	for _, result := range capped {
		sb.WriteString(fmt.Sprintf("Step: %s\nResult: %s\n\n", result.Step, result.Result))
	}
	return sb.String()
}

// English note.
// English note.
func planExecuteReplannerGenInput(orchInstruction string) planexecute.GenModelInputFn {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
		planContent, err := in.Plan.MarshalJSON()
		if err != nil {
			return nil, err
		}
		msgs, err := planexecute.ReplannerPrompt.Format(ctx, map[string]any{
			"plan":           string(planContent),
			"input":          planExecuteFormatInput(in.UserInput),
			"executed_steps": planExecuteFormatExecutedSteps(in.ExecutedSteps),
			"plan_tool":      planexecute.PlanToolInfo.Name,
			"respond_tool":   planexecute.RespondToolInfo.Name,
		})
		if err != nil {
			return nil, err
		}
		if oi != "" {
			msgs = append([]adk.Message{schema.SystemMessage(oi)}, msgs...)
		}
		return msgs, nil
	}
}

// English note.
func planExecuteStreamsMainAssistant(agent string) bool {
	if agent == "" {
		return true
	}
	switch agent {
	case "planner", "executor", "replanner", "execute_replan", "plan_execute_replan":
		return true
	default:
		return false
	}
}

func planExecuteEinoRoleTag(agent string) string {
	_ = agent
	return "orchestrator"
}
