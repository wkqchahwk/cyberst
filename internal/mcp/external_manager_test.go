package mcp

import (
	"context"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

func TestExternalMCPManager_AddOrUpdateConfig(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// English note.
	stdioCfg := config.ExternalMCPServerConfig{
		Command:     "python3",
		Args:        []string{"/path/to/script.py"},
		Transport:   "stdio",
		Description: "Test stdio MCP",
		Timeout:     30,
		Enabled:     true,
	}

	err := manager.AddOrUpdateConfig("test-stdio", stdioCfg)
	if err != nil {
		t.Fatalf("添加stdio配置失败: %v", err)
	}

	// English note.
	httpCfg := config.ExternalMCPServerConfig{
		Transport:   "http",
		URL:         "http://127.0.0.1:8081/mcp",
		Description: "Test HTTP MCP",
		Timeout:     30,
		Enabled:     false,
	}

	err = manager.AddOrUpdateConfig("test-http", httpCfg)
	if err != nil {
		t.Fatalf("添加HTTP配置失败: %v", err)
	}

	// English note.
	configs := manager.GetConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望2个配置，实际%d个", len(configs))
	}

	if configs["test-stdio"].Command != stdioCfg.Command {
		t.Errorf("stdio配置命令不匹配")
	}

	if configs["test-http"].URL != httpCfg.URL {
		t.Errorf("HTTP配置URL不匹配")
	}
}

func TestExternalMCPManager_RemoveConfig(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	cfg := config.ExternalMCPServerConfig{
		Command:   "python3",
		Transport: "stdio",
		Enabled:   false,
	}

	manager.AddOrUpdateConfig("test-remove", cfg)

	// English note.
	err := manager.RemoveConfig("test-remove")
	if err != nil {
		t.Fatalf("移除配置失败: %v", err)
	}

	configs := manager.GetConfigs()
	if _, exists := configs["test-remove"]; exists {
		t.Error("配置应该已被移除")
	}
}

func TestExternalMCPManager_GetStats(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// English note.
	manager.AddOrUpdateConfig("enabled1", config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	})

	manager.AddOrUpdateConfig("enabled2", config.ExternalMCPServerConfig{
		URL:     "http://127.0.0.1:8081/mcp",
		Enabled: true,
	})

	manager.AddOrUpdateConfig("disabled1", config.ExternalMCPServerConfig{
		Command:  "python3",
		Enabled:  false,
		Disabled: true, // 明确设置为禁用
	})

	stats := manager.GetStats()

	if stats["total"].(int) != 3 {
		t.Errorf("期望总数3，实际%d", stats["total"])
	}

	if stats["enabled"].(int) != 2 {
		t.Errorf("期望启用数2，实际%d", stats["enabled"])
	}

	if stats["disabled"].(int) != 1 {
		t.Errorf("期望停用数1，实际%d", stats["disabled"])
	}
}

func TestExternalMCPManager_LoadConfigs(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	externalMCPConfig := config.ExternalMCPConfig{
		Servers: map[string]config.ExternalMCPServerConfig{
			"loaded1": {
				Command: "python3",
				Enabled: true,
			},
			"loaded2": {
				URL:     "http://127.0.0.1:8081/mcp",
				Enabled: false,
			},
		},
	}

	manager.LoadConfigs(&externalMCPConfig)

	configs := manager.GetConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望2个配置，实际%d个", len(configs))
	}

	if configs["loaded1"].Command != "python3" {
		t.Error("配置1加载失败")
	}

	if configs["loaded2"].URL != "http://127.0.0.1:8081/mcp" {
		t.Error("配置2加载失败")
	}
}

// English note.
func TestLazySDKClient_InitializeFails(t *testing.T) {
	logger := zap.NewNop()
	// English note.
	cfg := config.ExternalMCPServerConfig{
		Transport: "http",
		URL:       "http://127.0.0.1:19999/nonexistent",
		Timeout:   2,
	}
	c := newLazySDKClient(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := c.Initialize(ctx)
	if err == nil {
		t.Fatal("expected error when connecting to invalid server")
	}
	if c.GetStatus() != "error" {
		t.Errorf("expected status error, got %s", c.GetStatus())
	}
	c.Close()
}

func TestExternalMCPManager_StartStopClient(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// English note.
	cfg := config.ExternalMCPServerConfig{
		Command:   "python3",
		Transport: "stdio",
		Enabled:   false,
	}

	manager.AddOrUpdateConfig("test-start-stop", cfg)

	// English note.
	err := manager.StartClient("test-start-stop")
	if err != nil {
		t.Logf("启动失败（可能是没有服务器）: %v", err)
	}

	// English note.
	err = manager.StopClient("test-start-stop")
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	// English note.
	configs := manager.GetConfigs()
	if configs["test-start-stop"].Enabled {
		t.Error("配置应该已被禁用")
	}
}

func TestExternalMCPManager_CallTool(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// English note.
	_, _, err := manager.CallTool(context.Background(), "nonexistent::tool", map[string]interface{}{})
	if err == nil {
		t.Error("应该返回错误")
	}

	// English note.
	_, _, err = manager.CallTool(context.Background(), "invalid-tool-name", map[string]interface{}{})
	if err == nil {
		t.Error("应该返回错误（无效格式）")
	}
}

func TestExternalMCPManager_GetAllTools(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	ctx := context.Background()
	tools, err := manager.GetAllTools(ctx)
	if err != nil {
		t.Fatalf("获取工具列表失败: %v", err)
	}

	// English note.
	if len(tools) != 0 {
		t.Logf("获取到%d个工具", len(tools))
	}
}
