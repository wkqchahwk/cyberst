package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"github.com/creack/pty"
	"go.uber.org/zap"
)

// English note.
// English note.
type ToolOutputCallback func(chunk string)

type toolOutputCallbackCtxKey struct{}

// English note.
var ToolOutputCallbackCtxKey = toolOutputCallbackCtxKey{}

// English note.
type Executor struct {
	config        *config.SecurityConfig
	toolIndex     map[string]*config.ToolConfig // 工具索引，用于 O(1) 查找
	mcpServer     *mcp.Server
	logger        *zap.Logger
	resultStorage ResultStorage // 结果存储（用于查询工具）
}

// English note.
type ResultStorage interface {
	SaveResult(executionID string, toolName string, result string) error
	GetResult(executionID string) (string, error)
	GetResultPage(executionID string, page int, limit int) (*storage.ResultPage, error)
	SearchResult(executionID string, keyword string, useRegex bool) ([]string, error)
	FilterResult(executionID string, filter string, useRegex bool) ([]string, error)
	GetResultMetadata(executionID string) (*storage.ResultMetadata, error)
	GetResultPath(executionID string) string
	DeleteResult(executionID string) error
}

// English note.
func NewExecutor(cfg *config.SecurityConfig, mcpServer *mcp.Server, logger *zap.Logger) *Executor {
	executor := &Executor{
		config:        cfg,
		toolIndex:     make(map[string]*config.ToolConfig),
		mcpServer:     mcpServer,
		logger:        logger,
		resultStorage: nil, // 稍后通过 SetResultStorage 设置
	}
	// English note.
	executor.buildToolIndex()
	return executor
}

// English note.
func (e *Executor) SetResultStorage(storage ResultStorage) {
	e.resultStorage = storage
}

func (e *Executor) actionEnabled() bool {
	if e == nil || e.config == nil {
		return false
	}
	return e.config.ActionEnabled
}

func (e *Executor) blockedByActionModeResult(toolName string, reason string) *mcp.ToolResult {
	msg := fmt.Sprintf(
		"Action Execution is OFF. Tool `%s` was not executed.\n\nReason: %s\n\nUse passive validation output and collected evidence to prepare the red-team report. If you need a controlled execution window, turn Action Execution ON in Settings > Security.",
		toolName,
		reason,
	)
	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}

func (e *Executor) enforceActionMode(toolName string, toolConfig *config.ToolConfig) *mcp.ToolResult {
	if e.actionEnabled() {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(toolName), "exec") {
		return e.blockedByActionModeResult(toolName, "Arbitrary command execution is blocked in report-only mode.")
	}
	if toolConfig != nil && toolConfig.RequiresActionEnabled {
		return e.blockedByActionModeResult(toolName, "This tool is marked as requiring Action Execution to be enabled.")
	}
	return nil
}

// English note.
func (e *Executor) buildToolIndex() {
	e.toolIndex = make(map[string]*config.ToolConfig)
	for i := range e.config.Tools {
		if e.config.Tools[i].Enabled {
			e.toolIndex[e.config.Tools[i].Name] = &e.config.Tools[i]
		}
	}
	e.logger.Info("工具索引构建完成",
		zap.Int("totalTools", len(e.config.Tools)),
		zap.Int("enabledTools", len(e.toolIndex)),
	)
}

// English note.
func (e *Executor) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
	e.logger.Info("ExecuteTool被调用",
		zap.String("toolName", toolName),
		zap.Any("args", args),
	)

	// English note.
	if toolName == "exec" && !e.actionEnabled() {
		e.logger.Warn("blocked tool execution because Action Execution is OFF", zap.String("tool", toolName))
		return e.blockedByActionModeResult(toolName, "Arbitrary command execution is blocked in report-only mode."), nil
	}
	if toolName == "exec" {
		e.logger.Info("执行exec工具")
		return e.executeSystemCommand(ctx, args)
	}

	// English note.
	toolConfig, exists := e.toolIndex[toolName]
	if exists {
		if blocked := e.enforceActionMode(toolName, toolConfig); blocked != nil {
			e.logger.Warn("blocked tool execution because Action Execution is OFF",
				zap.String("tool", toolName),
				zap.String("command", toolConfig.Command),
			)
			return blocked, nil
		}
	}
	if !exists {
		e.logger.Error("工具未找到或未启用",
			zap.String("toolName", toolName),
			zap.Int("totalTools", len(e.config.Tools)),
			zap.Int("enabledTools", len(e.toolIndex)),
		)
		return nil, fmt.Errorf("工具 %s 未找到或未启用", toolName)
	}

	e.logger.Info("找到工具配置",
		zap.String("toolName", toolName),
		zap.String("command", toolConfig.Command),
		zap.Strings("args", toolConfig.Args),
	)

	// English note.
	if strings.HasPrefix(toolConfig.Command, "internal:") {
		e.logger.Info("执行内部工具",
			zap.String("toolName", toolName),
			zap.String("command", toolConfig.Command),
		)
		return e.executeInternalTool(ctx, toolName, toolConfig.Command, args)
	}

	// English note.
	cmdArgs := e.buildCommandArgs(toolName, toolConfig, args)

	e.logger.Info("构建命令参数完成",
		zap.String("toolName", toolName),
		zap.Strings("cmdArgs", cmdArgs),
		zap.Int("argsCount", len(cmdArgs)),
	)

	// English note.
	if len(cmdArgs) == 0 {
		e.logger.Warn("命令参数为空",
			zap.String("toolName", toolName),
			zap.Any("inputArgs", args),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("错误: 工具 %s 缺少必需的参数。接收到的参数: %v", toolName, args),
				},
			},
			IsError: true,
		}, nil
	}

	// English note.
	cmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)
	applyDefaultTerminalEnv(cmd)

	e.logger.Info("执行安全工具",
		zap.String("tool", toolName),
		zap.Strings("args", cmdArgs),
	)

	var output string
	var err error
	// English note.
	if cb, ok := ctx.Value(ToolOutputCallbackCtxKey).(ToolOutputCallback); ok && cb != nil {
		output, err = streamCommandOutput(cmd, cb)
		if err != nil && shouldRetryWithPTY(output) {
			e.logger.Info("检测到工具需要 TTY，使用 PTY 重试",
				zap.String("tool", toolName),
			)
			cmd2 := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)
			applyDefaultTerminalEnv(cmd2)
			output, err = runCommandWithPTY(ctx, cmd2, cb)
		}
	} else {
		outputBytes, err2 := cmd.CombinedOutput()
		output = string(outputBytes)
		err = err2
		if err != nil && shouldRetryWithPTY(output) {
			e.logger.Info("检测到工具需要 TTY，使用 PTY 重试",
				zap.String("tool", toolName),
			)
			cmd2 := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)
			applyDefaultTerminalEnv(cmd2)
			output, err = runCommandWithPTY(ctx, cmd2, nil)
		}
	}
	if err != nil {
		// English note.
		exitCode := getExitCode(err)
		if exitCode != nil && toolConfig.AllowedExitCodes != nil {
			for _, allowedCode := range toolConfig.AllowedExitCodes {
				if *exitCode == allowedCode {
					e.logger.Info("工具执行完成（退出码在允许列表中）",
						zap.String("tool", toolName),
						zap.Int("exitCode", *exitCode),
						zap.String("output", string(output)),
					)
					return &mcp.ToolResult{
						Content: []mcp.Content{
							{
								Type: "text",
								Text: string(output),
							},
						},
						IsError: false,
					}, nil
				}
			}
		}

		e.logger.Error("工具执行失败",
			zap.String("tool", toolName),
			zap.Error(err),
			zap.Int("exitCode", getExitCodeValue(err)),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("工具执行失败: %v\n输出: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("工具执行成功",
		zap.String("tool", toolName),
		zap.String("output", string(output)),
	)

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: string(output),
			},
		},
		IsError: false,
	}, nil
}

// English note.
func (e *Executor) RegisterTools(mcpServer *mcp.Server) {
	e.logger.Info("开始注册工具",
		zap.Int("totalTools", len(e.config.Tools)),
		zap.Int("enabledTools", len(e.toolIndex)),
	)

	// English note.
	e.buildToolIndex()

	for i, toolConfig := range e.config.Tools {
		if !toolConfig.Enabled {
			e.logger.Debug("跳过未启用的工具",
				zap.String("tool", toolConfig.Name),
			)
			continue
		}

		// English note.
		toolName := toolConfig.Name
		toolConfigCopy := toolConfig

		// English note.
		useFullDescription := strings.TrimSpace(strings.ToLower(e.config.ToolDescriptionMode)) == "full"
		shortDesc := toolConfigCopy.ShortDescription
		if shortDesc == "" {
			// English note.
			desc := toolConfigCopy.Description
			if len(desc) > 10000 {
				if idx := strings.Index(desc, "\n"); idx > 0 && idx < 10000 {
					shortDesc = strings.TrimSpace(desc[:idx])
				} else {
					shortDesc = desc[:10000] + "..."
				}
			} else {
				shortDesc = desc
			}
		}
		if useFullDescription {
			shortDesc = "" // 使用 description 时清空 ShortDescription，下游会回退到 Description
		}

		tool := mcp.Tool{
			Name:             toolConfigCopy.Name,
			Description:      toolConfigCopy.Description,
			ShortDescription: shortDesc,
			InputSchema:      e.buildInputSchema(&toolConfigCopy),
		}

		handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
			e.logger.Info("工具handler被调用",
				zap.String("toolName", toolName),
				zap.Any("args", args),
			)
			return e.ExecuteTool(ctx, toolName, args)
		}

		mcpServer.RegisterTool(tool, handler)
		e.logger.Info("注册安全工具成功",
			zap.String("tool", toolConfigCopy.Name),
			zap.String("command", toolConfigCopy.Command),
			zap.Int("index", i),
		)
	}

	e.logger.Info("工具注册完成",
		zap.Int("registeredCount", len(e.config.Tools)),
	)
}

// English note.
func (e *Executor) buildCommandArgs(toolName string, toolConfig *config.ToolConfig, args map[string]interface{}) []string {
	cmdArgs := make([]string, 0)

	// English note.
	if len(toolConfig.Parameters) > 0 {
		// English note.
		hasScanType := false
		var scanTypeValue string
		if scanType, ok := args["scan_type"].(string); ok && scanType != "" {
			hasScanType = true
			scanTypeValue = scanType
		}

		// English note.
		if hasScanType && toolName == "nmap" {
			// English note.
			// English note.
		} else {
			cmdArgs = append(cmdArgs, toolConfig.Args...)
		}

		// English note.
		positionalParams := make([]config.ParameterConfig, 0)
		flagParams := make([]config.ParameterConfig, 0)

		for _, param := range toolConfig.Parameters {
			if param.Position != nil {
				positionalParams = append(positionalParams, param)
			} else {
				flagParams = append(flagParams, param)
			}
		}

		// English note.
		for _, param := range positionalParams {
			if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
				continue
			}
			if param.Position != nil && *param.Position == 0 {
				value := e.getParamValue(args, param)
				if value == nil && param.Default != nil {
					value = param.Default
				}
				if value != nil {
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
				break
			}
		}

		// English note.
		for _, param := range flagParams {
			// English note.
			// English note.
			if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
				continue
			}

			value := e.getParamValue(args, param)
			if value == nil {
				if param.Required {
					// English note.
					e.logger.Warn("缺少必需的标志参数",
						zap.String("tool", toolName),
						zap.String("param", param.Name),
					)
					return []string{}
				}
				continue
			}

			// English note.
			if param.Type == "bool" {
				var boolVal bool
				var ok bool

				// English note.
				if boolVal, ok = value.(bool); ok {
					// English note.
				} else if numVal, ok := value.(float64); ok {
					// English note.
					boolVal = numVal != 0
					ok = true
				} else if numVal, ok := value.(int); ok {
					// English note.
					boolVal = numVal != 0
					ok = true
				} else if strVal, ok := value.(string); ok {
					// English note.
					boolVal = strVal == "true" || strVal == "1" || strVal == "yes"
					ok = true
				}

				if ok {
					if !boolVal {
						continue // false 时不添加任何参数
					}
					// English note.
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					continue
				}
			}

			format := param.Format
			if format == "" {
				format = "flag" // 默认格式
			}

			switch format {
			case "flag":
				// English note.
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, param.Flag)
				}
				formattedValue := e.formatParamValue(param, value)
				if formattedValue != "" {
					cmdArgs = append(cmdArgs, formattedValue)
				}
			case "combined":
				// English note.
				if param.Flag != "" {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", param.Flag, e.formatParamValue(param, value)))
				} else {
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
			case "template":
				// English note.
				if param.Template != "" {
					template := param.Template
					template = strings.ReplaceAll(template, "{flag}", param.Flag)
					template = strings.ReplaceAll(template, "{value}", e.formatParamValue(param, value))
					template = strings.ReplaceAll(template, "{name}", param.Name)
					cmdArgs = append(cmdArgs, strings.Fields(template)...)
				} else {
					// English note.
					if param.Flag != "" {
						cmdArgs = append(cmdArgs, param.Flag)
					}
					cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
				}
			case "positional":
				// English note.
				cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
			default:
				// English note.
				cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
			}
		}

		// English note.
		// English note.
		// English note.
		maxPosition := -1
		for _, param := range positionalParams {
			if param.Position != nil && *param.Position > maxPosition {
				maxPosition = *param.Position
			}
		}

		// English note.
		// English note.
		for i := 0; i <= maxPosition; i++ {
			if i == 0 {
				continue
			}
			for _, param := range positionalParams {
				// English note.
				// English note.
				if param.Name == "additional_args" || param.Name == "scan_type" || param.Name == "action" {
					continue
				}

				if param.Position != nil && *param.Position == i {
					value := e.getParamValue(args, param)
					if value == nil {
						if param.Required {
							// English note.
							e.logger.Warn("缺少必需的位置参数",
								zap.String("tool", toolName),
								zap.String("param", param.Name),
								zap.Int("position", *param.Position),
							)
							return []string{}
						}
						// English note.
						if param.Default != nil {
							value = param.Default
						} else {
							// English note.
							break
						}
					}
					// English note.
					if value != nil {
						cmdArgs = append(cmdArgs, e.formatParamValue(param, value))
					}
					break
				}
			}
			// English note.
			// English note.
		}

		// English note.
		if additionalArgs, ok := args["additional_args"].(string); ok && additionalArgs != "" {
			// English note.
			additionalArgsList := e.parseAdditionalArgs(additionalArgs)
			cmdArgs = append(cmdArgs, additionalArgsList...)
		}

		// English note.
		if hasScanType {
			scanTypeArgs := e.parseAdditionalArgs(scanTypeValue)
			if len(scanTypeArgs) > 0 {
				// English note.
				// English note.
				// English note.
				insertPos := len(cmdArgs)
				for i := len(cmdArgs) - 1; i >= 0; i-- {
					// English note.
					if !strings.HasPrefix(cmdArgs[i], "-") {
						insertPos = i
						break
					}
				}
				// English note.
				newArgs := make([]string, 0, len(cmdArgs)+len(scanTypeArgs))
				newArgs = append(newArgs, cmdArgs[:insertPos]...)
				newArgs = append(newArgs, scanTypeArgs...)
				newArgs = append(newArgs, cmdArgs[insertPos:]...)
				cmdArgs = newArgs
			}
		}

		return cmdArgs
	}

	// English note.
	// English note.
	cmdArgs = append(cmdArgs, toolConfig.Args...)

	// English note.
	for key, value := range args {
		if key == "_tool_name" {
			continue
		}
		// English note.
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
		if strValue, ok := value.(string); ok {
			cmdArgs = append(cmdArgs, strValue)
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
		}
	}

	return cmdArgs
}

// English note.
func (e *Executor) parseAdditionalArgs(argsStr string) []string {
	if argsStr == "" {
		return []string{}
	}

	result := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	var quoteChar rune
	escapeNext := false

	runes := []rune(argsStr)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		if r == '\\' {
			// English note.
			if i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\'') {
				// English note.
				i++
				current.WriteRune(runes[i])
			} else {
				// English note.
				escapeNext = true
				current.WriteRune(r)
			}
			continue
		}

		if !inQuotes && (r == '"' || r == '\'') {
			inQuotes = true
			quoteChar = r
			continue
		}

		if inQuotes && r == quoteChar {
			inQuotes = false
			quoteChar = 0
			continue
		}

		if !inQuotes && (r == ' ' || r == '\t' || r == '\n') {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	// English note.
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// English note.
	if len(result) == 0 {
		result = strings.Fields(argsStr)
	}

	return result
}

// English note.
func (e *Executor) getParamValue(args map[string]interface{}, param config.ParameterConfig) interface{} {
	// English note.
	if value, ok := args[param.Name]; ok && value != nil {
		return value
	}

	// English note.
	if param.Required {
		return nil
	}

	// English note.
	return param.Default
}

// English note.
func (e *Executor) formatParamValue(param config.ParameterConfig, value interface{}) string {
	switch param.Type {
	case "bool":
		// English note.
		if boolVal, ok := value.(bool); ok {
			return fmt.Sprintf("%v", boolVal)
		}
		return "false"
	case "array":
		// English note.
		if arr, ok := value.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, item := range arr {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			return strings.Join(strs, ",")
		}
		return fmt.Sprintf("%v", value)
	case "object":
		// English note.
		if jsonBytes, err := json.Marshal(value); err == nil {
			return string(jsonBytes)
		}
		// English note.
		return fmt.Sprintf("%v", value)
	default:
		formattedValue := fmt.Sprintf("%v", value)
		// English note.
		// English note.
		if param.Name == "ports" {
			// English note.
			formattedValue = strings.ReplaceAll(formattedValue, " ", "")
		}
		return formattedValue
	}
}

// English note.
// English note.
func (e *Executor) isBackgroundCommand(command string) bool {
	// English note.
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	// English note.
	// English note.
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	lastAmpersandPos := -1

	for i, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if r == '&' && !inSingleQuote && !inDoubleQuote {
			// English note.
			isStandalone := false

			// English note.
			if i == 0 {
				isStandalone = true
			} else {
				prev := command[i-1]
				if prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' {
					isStandalone = true
				}
			}

			// English note.
			if isStandalone {
				if i == len(command)-1 {
					// English note.
					lastAmpersandPos = i
				} else {
					next := command[i+1]
					if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
						// English note.
						lastAmpersandPos = i
					}
				}
			}
		}
	}

	// English note.
	if lastAmpersandPos == -1 {
		return false
	}

	// English note.
	afterAmpersand := strings.TrimSpace(command[lastAmpersandPos+1:])
	if afterAmpersand == "" {
		// English note.
		// English note.
		beforeAmpersand := strings.TrimSpace(command[:lastAmpersandPos])
		return beforeAmpersand != ""
	}

	// English note.
	// English note.
	return false
}

// English note.
func (e *Executor) executeSystemCommand(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// English note.
	command, ok := args["command"].(string)
	if !ok {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: 缺少command参数",
				},
			},
			IsError: true,
		}, nil
	}

	if command == "" {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: command参数不能为空",
				},
			},
			IsError: true,
		}, nil
	}

	// English note.
	e.logger.Warn("执行系统命令",
		zap.String("command", command),
	)

	// English note.
	shell := "sh"
	if s, ok := args["shell"].(string); ok && s != "" {
		shell = s
	}

	// English note.
	workDir := ""
	if wd, ok := args["workdir"].(string); ok && wd != "" {
		workDir = wd
	}

	// English note.
	isBackground := e.isBackgroundCommand(command)

	// English note.
	var cmd *exec.Cmd
	if workDir != "" {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
		cmd.Dir = workDir
	} else {
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	// English note.
	e.logger.Info("执行系统命令",
		zap.String("command", command),
		zap.String("shell", shell),
		zap.String("workdir", workDir),
		zap.Bool("isBackground", isBackground),
	)

	// English note.
	if isBackground {
		// English note.
		commandWithoutAmpersand := strings.TrimSuffix(strings.TrimSpace(command), "&")
		commandWithoutAmpersand = strings.TrimSpace(commandWithoutAmpersand)

		// English note.
		// English note.
		pidCommand := fmt.Sprintf("%s & pid=$!; echo $pid", commandWithoutAmpersand)

		// English note.
		var pidCmd *exec.Cmd
		if workDir != "" {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
			pidCmd.Dir = workDir
		} else {
			pidCmd = exec.CommandContext(ctx, shell, "-c", pidCommand)
		}

		// English note.
		stdout, err := pidCmd.StdoutPipe()
		if err != nil {
			e.logger.Error("创建stdout管道失败",
				zap.String("command", command),
				zap.Error(err),
			)
			// English note.
			if err := pidCmd.Start(); err != nil {
				return &mcp.ToolResult{
					Content: []mcp.Content{
						{
							Type: "text",
							Text: fmt.Sprintf("后台命令启动失败: %v", err),
						},
					},
					IsError: true,
				}, nil
			}
			pid := pidCmd.Process.Pid
			go pidCmd.Wait() // 在后台等待，避免僵尸进程
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("后台命令已启动\n命令: %s\n进程ID: %d (可能不准确，获取PID失败)\n\n注意: 后台进程将继续运行，不会等待其完成。", command, pid),
					},
				},
				IsError: false,
			}, nil
		}

		// English note.
		if err := pidCmd.Start(); err != nil {
			stdout.Close()
			e.logger.Error("后台命令启动失败",
				zap.String("command", command),
				zap.Error(err),
			)
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("后台命令启动失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		// English note.
		reader := bufio.NewReader(stdout)
		pidLine, err := reader.ReadString('\n')
		stdout.Close()

		var actualPid int
		if err != nil && err != io.EOF {
			e.logger.Warn("读取后台进程PID失败",
				zap.String("command", command),
				zap.Error(err),
			)
			// English note.
			actualPid = pidCmd.Process.Pid
		} else {
			// English note.
			pidStr := strings.TrimSpace(pidLine)
			if parsedPid, err := strconv.Atoi(pidStr); err == nil {
				actualPid = parsedPid
			} else {
				e.logger.Warn("解析后台进程PID失败",
					zap.String("command", command),
					zap.String("pidLine", pidStr),
					zap.Error(err),
				)
				// English note.
				actualPid = pidCmd.Process.Pid
			}
		}

		// English note.
		go func() {
			if err := pidCmd.Wait(); err != nil {
				e.logger.Debug("后台命令shell进程执行完成",
					zap.String("command", command),
					zap.Error(err),
				)
			}
		}()

		e.logger.Info("后台命令已启动",
			zap.String("command", command),
			zap.Int("actualPid", actualPid),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("后台命令已启动\n命令: %s\n进程ID: %d\n\n注意: 后台进程将继续运行，不会等待其完成。", command, actualPid),
				},
			},
			IsError: false,
		}, nil
	}

	// English note.
	var output string
	var err error
	// English note.
	if cb, ok := ctx.Value(ToolOutputCallbackCtxKey).(ToolOutputCallback); ok && cb != nil {
		output, err = streamCommandOutput(cmd, cb)
		if err != nil && shouldRetryWithPTY(output) {
			e.logger.Info("检测到系统命令需要 TTY，使用 PTY 重试")
			cmd2 := exec.CommandContext(ctx, shell, "-c", command)
			if workDir != "" {
				cmd2.Dir = workDir
			}
			applyDefaultTerminalEnv(cmd2)
			output, err = runCommandWithPTY(ctx, cmd2, cb)
		}
	} else {
		outputBytes, err2 := cmd.CombinedOutput()
		output = string(outputBytes)
		err = err2
		if err != nil && shouldRetryWithPTY(output) {
			e.logger.Info("检测到系统命令需要 TTY，使用 PTY 重试")
			cmd2 := exec.CommandContext(ctx, shell, "-c", command)
			if workDir != "" {
				cmd2.Dir = workDir
			}
			applyDefaultTerminalEnv(cmd2)
			output, err = runCommandWithPTY(ctx, cmd2, nil)
		}
	}
	if err != nil {
		e.logger.Error("系统命令执行失败",
			zap.String("command", command),
			zap.Error(err),
			zap.String("output", string(output)),
		)
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("命令执行失败: %v\n输出: %s", err, string(output)),
				},
			},
			IsError: true,
		}, nil
	}

	e.logger.Info("系统命令执行成功",
		zap.String("command", command),
		zap.String("output_length", fmt.Sprintf("%d", len(output))),
	)

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: string(output),
			},
		},
		IsError: false,
	}, nil
}

// English note.
// English note.
func streamCommandOutput(cmd *exec.Cmd, cb ToolOutputCallback) (string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		return "", err
	}
	if err := cmd.Start(); err != nil {
		_ = stdoutPipe.Close()
		_ = stderrPipe.Close()
		return "", err
	}

	chunks := make(chan string, 64)
	var wg sync.WaitGroup
	readFn := func(r io.Reader) {
		defer wg.Done()
		br := bufio.NewReader(r)
		for {
			s, readErr := br.ReadString('\n')
			if s != "" {
				chunks <- s
			}
			if readErr != nil {
				// English note.
				return
			}
		}
	}

	wg.Add(2)
	go readFn(stdoutPipe)
	go readFn(stderrPipe)

	go func() {
		wg.Wait()
		close(chunks)
	}()

	var outBuilder strings.Builder
	var deltaBuilder strings.Builder
	lastFlush := time.Now()

	flush := func() {
		if deltaBuilder.Len() == 0 {
			return
		}
		cb(deltaBuilder.String())
		deltaBuilder.Reset()
		lastFlush = time.Now()
	}

	for chunk := range chunks {
		outBuilder.WriteString(chunk)
		deltaBuilder.WriteString(chunk)
		// English note.
		if deltaBuilder.Len() >= 2048 || time.Since(lastFlush) >= 200*time.Millisecond {
			flush()
		}
	}
	flush()

	// English note.
	waitErr := cmd.Wait()
	return outBuilder.String(), waitErr
}

// English note.
// English note.
func applyDefaultTerminalEnv(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	// English note.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	// English note.
	has := func(k string) bool {
		prefix := k + "="
		for _, e := range cmd.Env {
			if strings.HasPrefix(e, prefix) {
				return true
			}
		}
		return false
	}
	if !has("TERM") {
		cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	}
	if !has("COLUMNS") {
		cmd.Env = append(cmd.Env, "COLUMNS=256")
	}
	if !has("LINES") {
		cmd.Env = append(cmd.Env, "LINES=40")
	}
}

func shouldRetryWithPTY(output string) bool {
	o := strings.ToLower(output)
	// English note.
	if strings.Contains(o, "inappropriate ioctl for device") {
		return true
	}
	if strings.Contains(o, "termios.error") {
		return true
	}
	// English note.
	if strings.Contains(o, "not a tty") {
		return true
	}
	return false
}

// English note.
// English note.
func runCommandWithPTY(ctx context.Context, cmd *exec.Cmd, cb ToolOutputCallback) (string, error) {
	if runtime.GOOS == "windows" {
		// English note.
		if cb != nil {
			return streamCommandOutput(cmd, cb)
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", err
	}
	defer func() { _ = ptmx.Close() }()

	// English note.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ptmx.Close() // 触发读退出
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()
	defer close(done)

	var outBuilder strings.Builder
	var deltaBuilder strings.Builder
	lastFlush := time.Now()
	flush := func() {
		if cb == nil || deltaBuilder.Len() == 0 {
			deltaBuilder.Reset()
			lastFlush = time.Now()
			return
		}
		cb(deltaBuilder.String())
		deltaBuilder.Reset()
		lastFlush = time.Now()
	}

	buf := make([]byte, 4096)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			// English note.
			chunk = strings.ReplaceAll(chunk, "\r\n", "\n")
			chunk = strings.ReplaceAll(chunk, "\r", "\n")
			outBuilder.WriteString(chunk)
			deltaBuilder.WriteString(chunk)
			if deltaBuilder.Len() >= 2048 || time.Since(lastFlush) >= 200*time.Millisecond {
				flush()
			}
		}
		if readErr != nil {
			break
		}
	}
	flush()

	waitErr := cmd.Wait()
	return outBuilder.String(), waitErr
}

// English note.
func (e *Executor) executeInternalTool(ctx context.Context, toolName string, command string, args map[string]interface{}) (*mcp.ToolResult, error) {
	// English note.
	internalToolType := strings.TrimPrefix(command, "internal:")

	e.logger.Info("执行内部工具",
		zap.String("toolName", toolName),
		zap.String("internalToolType", internalToolType),
		zap.Any("args", args),
	)

	// English note.
	switch internalToolType {
	case "query_execution_result":
		return e.executeQueryExecutionResult(ctx, args)
	default:
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("错误: 未知的内部工具类型: %s", internalToolType),
				},
			},
			IsError: true,
		}, nil
	}
}

// English note.
func (e *Executor) executeQueryExecutionResult(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
	// English note.
	executionID, ok := args["execution_id"].(string)
	if !ok || executionID == "" {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: execution_id 参数必需且不能为空",
				},
			},
			IsError: true,
		}, nil
	}

	// English note.
	page := 1
	if p, ok := args["page"].(float64); ok {
		page = int(p)
	}
	if page < 1 {
		page = 1
	}

	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500 // 限制最大每页行数
	}

	search := ""
	if s, ok := args["search"].(string); ok {
		search = s
	}

	filter := ""
	if f, ok := args["filter"].(string); ok {
		filter = f
	}

	useRegex := false
	if r, ok := args["use_regex"].(bool); ok {
		useRegex = r
	}

	// English note.
	if e.resultStorage == nil {
		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: "错误: 结果存储未初始化",
				},
			},
			IsError: true,
		}, nil
	}

	// English note.
	var resultPage *storage.ResultPage
	var err error

	if search != "" {
		// English note.
		matchedLines, err := e.resultStorage.SearchResult(executionID, search, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("搜索失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// English note.
		resultPage = paginateLines(matchedLines, page, limit)
	} else if filter != "" {
		// English note.
		filteredLines, err := e.resultStorage.FilterResult(executionID, filter, useRegex)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("过滤失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		// English note.
		resultPage = paginateLines(filteredLines, page, limit)
	} else {
		// English note.
		resultPage, err = e.resultStorage.GetResultPage(executionID, page, limit)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("查询失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// English note.
	metadata, err := e.resultStorage.GetResultMetadata(executionID)
	if err != nil {
		// English note.
		e.logger.Warn("获取结果元信息失败", zap.Error(err))
	}

	// English note.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("查询结果 (执行ID: %s)\n", executionID))

	if metadata != nil {
		sb.WriteString(fmt.Sprintf("工具: %s | 大小: %d 字节 (%.2f KB) | 总行数: %d\n",
			metadata.ToolName, metadata.TotalSize, float64(metadata.TotalSize)/1024, metadata.TotalLines))
	}

	sb.WriteString(fmt.Sprintf("第 %d/%d 页，每页 %d 行，共 %d 行\n\n",
		resultPage.Page, resultPage.TotalPages, resultPage.Limit, resultPage.TotalLines))

	if len(resultPage.Lines) == 0 {
		sb.WriteString("没有找到匹配的结果。\n")
	} else {
		for i, line := range resultPage.Lines {
			lineNum := (resultPage.Page-1)*resultPage.Limit + i + 1
			sb.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))
		}
	}

	sb.WriteString("\n")
	if resultPage.Page < resultPage.TotalPages {
		sb.WriteString(fmt.Sprintf("提示: 使用 page=%d 查看下一页", resultPage.Page+1))
		if search != "" {
			sb.WriteString(fmt.Sprintf("，或使用 search=\"%s\" 继续搜索", search))
			if useRegex {
				sb.WriteString(" (正则模式)")
			}
		}
		if filter != "" {
			sb.WriteString(fmt.Sprintf("，或使用 filter=\"%s\" 继续过滤", filter))
			if useRegex {
				sb.WriteString(" (正则模式)")
			}
		}
		sb.WriteString("\n")
	}

	return &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: sb.String(),
			},
		},
		IsError: false,
	}, nil
}

// English note.
func paginateLines(lines []string, page int, limit int) *storage.ResultPage {
	totalLines := len(lines)
	totalPages := (totalLines + limit - 1) / limit
	if page < 1 {
		page = 1
	}
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	start := (page - 1) * limit
	end := start + limit
	if end > totalLines {
		end = totalLines
	}

	var pageLines []string
	if start < totalLines {
		pageLines = lines[start:end]
	} else {
		pageLines = []string{}
	}

	return &storage.ResultPage{
		Lines:      pageLines,
		Page:       page,
		Limit:      limit,
		TotalLines: totalLines,
		TotalPages: totalPages,
	}
}

// English note.
func (e *Executor) buildInputSchema(toolConfig *config.ToolConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	// English note.
	if len(toolConfig.Parameters) > 0 {
		properties := make(map[string]interface{})
		required := []string{}

		for _, param := range toolConfig.Parameters {
			// English note.
			if strings.TrimSpace(param.Name) == "" {
				e.logger.Debug("跳过无名称的参数",
					zap.String("tool", toolConfig.Name),
					zap.String("type", param.Type),
				)
				continue
			}
			// English note.
			openAIType := e.convertToOpenAIType(param.Type)

			prop := map[string]interface{}{
				"type":        openAIType,
				"description": param.Description,
			}

			// English note.
			if openAIType == "array" {
				itemType := strings.TrimSpace(param.ItemType)
				if itemType == "" {
					itemType = "string"
				}
				prop["items"] = map[string]interface{}{
					"type": e.convertToOpenAIType(itemType),
				}
			}

			// English note.
			if param.Default != nil {
				prop["default"] = param.Default
			}

			// English note.
			if len(param.Options) > 0 {
				prop["enum"] = param.Options
			}

			properties[param.Name] = prop

			// English note.
			if param.Required {
				required = append(required, param.Name)
			}
		}

		schema["properties"] = properties
		schema["required"] = required
		return schema
	}

	// English note.
	// English note.
	// English note.
	e.logger.Warn("工具未定义参数配置，返回空schema",
		zap.String("tool", toolConfig.Name),
	)
	return schema
}

// English note.
func (e *Executor) convertToOpenAIType(configType string) string {
	// English note.
	if strings.TrimSpace(configType) == "" {
		return "string"
	}
	switch configType {
	case "bool":
		return "boolean"
	case "int", "integer":
		return "number"
	case "float", "double":
		return "number"
	case "string", "array", "object":
		return configType
	default:
		// English note.
		e.logger.Warn("未知的参数类型，使用原类型",
			zap.String("type", configType),
		)
		return configType
	}
}

// English note.
func getExitCode(err error) *int {
	if err == nil {
		return nil
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ProcessState != nil {
			exitCode := exitError.ExitCode()
			return &exitCode
		}
	}
	return nil
}

// English note.
func getExitCodeValue(err error) int {
	if code := getExitCode(err); code != nil {
		return *code
	}
	return -1
}
