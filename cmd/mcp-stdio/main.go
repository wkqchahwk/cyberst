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
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, "stderr")
	mcpServer := mcp.NewServer(log.Logger)
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	executor.RegisterTools(mcpServer)

	log.Logger.Info("MCP server (stdio mode) started and is waiting for messages")

	if err := mcpServer.HandleStdio(); err != nil {
		log.Logger.Error("MCP server stopped with an error", zap.Error(err))
		os.Exit(1)
	}
}
