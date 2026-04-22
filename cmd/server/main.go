package main

import (
	"cyberstrike-ai/internal/app"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"flag"
	"fmt"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	if err := config.EnsureMCPAuth(*configPath, cfg); err != nil {
		fmt.Printf("Failed to ensure MCP auth configuration: %v\n", err)
		return
	}
	if cfg.MCP.Enabled {
		config.PrintMCPConfigJSON(cfg.MCP)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Output)

	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("Application initialization failed", "error", err)
	}

	if err := application.Run(); err != nil {
		log.Fatal("Server startup failed", "error", err)
	}
}
