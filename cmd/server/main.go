package main

import (
	"cyberstrike-ai/internal/app"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"flag"
	"fmt"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// English note.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	// English note.
	if err := config.EnsureMCPAuth(*configPath, cfg); err != nil {
		fmt.Printf("MCP 鉴权配置失败: %v\n", err)
		return
	}
	if cfg.MCP.Enabled {
		config.PrintMCPConfigJSON(cfg.MCP)
	}

	// English note.
	log := logger.New(cfg.Log.Level, cfg.Log.Output)

	// English note.
	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("应用初始化失败", "error", err)
	}

	// English note.
	if err := application.Run(); err != nil {
		log.Fatal("服务器启动失败", "error", err)
	}
}

