package main

import (
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// English note.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// English note.
	log := logger.New(cfg.Log.Level, "stderr")

	// English note.
	mcpServer := mcp.NewServer(log.Logger)

	// English note.
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// English note.
	executor.RegisterTools(mcpServer)

	log.Logger.Info("MCP服务器（stdio模式）已启动，等待消息...")

	// English note.
	if err := mcpServer.HandleStdio(); err != nil {
		log.Logger.Error("MCP服务器运行失败", zap.Error(err))
		os.Exit(1)
	}
}

