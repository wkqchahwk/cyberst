package multiagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

// English note.
func newPlanExecuteExecutor(ctx context.Context, cfg *planexecute.ExecutorConfig, handlers []adk.ChatModelAgentMiddleware) (adk.Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("plan_execute: ExecutorConfig ")
	}
	if cfg.Model == nil {
		return nil, fmt.Errorf("plan_execute: Executor Model ")
	}
	genInputFn := cfg.GenInputFn
	if genInputFn == nil {
		genInputFn = planExecuteDefaultGenExecutorInput
	}
	genInput := func(ctx context.Context, instruction string, _ *adk.AgentInput) ([]adk.Message, error) {
		plan, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
		if !ok {
			return nil, fmt.Errorf("plan_execute executor: session value %q missing (possible session corruption)", planexecute.PlanSessionKey)
		}
		plan_ := plan.(planexecute.Plan)

		userInput, ok := adk.GetSessionValue(ctx, planexecute.UserInputSessionKey)
		if !ok {
			return nil, fmt.Errorf("plan_execute executor: session value %q missing (possible session corruption)", planexecute.UserInputSessionKey)
		}
		userInput_ := userInput.([]adk.Message)

		var executedSteps_ []planexecute.ExecutedStep
		executedStep, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey)
		if ok {
			executedSteps_ = executedStep.([]planexecute.ExecutedStep)
		}

		in := &planexecute.ExecutionContext{
			UserInput:     userInput_,
			Plan:          plan_,
			ExecutedSteps: executedSteps_,
		}
		return genInputFn(ctx, in)
	}

	agentCfg := &adk.ChatModelAgentConfig{
		Name:          "executor",
		Description:   "an executor agent",
		Model:         cfg.Model,
		ToolsConfig:   cfg.ToolsConfig,
		GenModelInput: genInput,
		MaxIterations: cfg.MaxIterations,
		OutputKey:     planexecute.ExecutedStepSessionKey,
	}
	if len(handlers) > 0 {
		agentCfg.Handlers = handlers
	}
	return adk.NewChatModelAgent(ctx, agentCfg)
}

// English note.
func planExecuteDefaultGenExecutorInput(ctx context.Context, in *planexecute.ExecutionContext) ([]adk.Message, error) {
	planContent, err := in.Plan.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return planexecute.ExecutorPrompt.Format(ctx, map[string]any{
		"input":          planExecuteFormatInput(in.UserInput),
		"plan":           string(planContent),
		"executed_steps": planExecuteFormatExecutedSteps(in.ExecutedSteps),
		"step":           in.Plan.FirstStep(),
	})
}
