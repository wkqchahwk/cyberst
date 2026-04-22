package handler

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// English note.
type ExternalMCPHandler struct {
	manager    *mcp.ExternalMCPManager
	config     *config.Config
	configPath string
	logger     *zap.Logger
	mu         sync.RWMutex
}

// English note.
func NewExternalMCPHandler(manager *mcp.ExternalMCPManager, cfg *config.Config, configPath string, logger *zap.Logger) *ExternalMCPHandler {
	return &ExternalMCPHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// English note.
func (h *ExternalMCPHandler) GetExternalMCPs(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := h.manager.GetConfigs()

	// English note.
	toolCounts := h.manager.GetToolCounts()

	// English note.
	result := make(map[string]ExternalMCPResponse)
	for name, cfg := range configs {
		client, exists := h.manager.GetClient(name)
		status := "disconnected"
		if exists {
			status = client.GetStatus()
		} else if h.isEnabled(cfg) {
			status = "disconnected"
		} else {
			status = "disabled"
		}

		toolCount := toolCounts[name]
		errorMsg := ""
		if status == "error" {
			errorMsg = h.manager.GetError(name)
		}

		result[name] = ExternalMCPResponse{
			Config:    cfg,
			Status:    status,
			ToolCount: toolCount,
			Error:     errorMsg,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"servers": result,
		"stats":   h.manager.GetStats(),
	})
}

// English note.
func (h *ExternalMCPHandler) GetExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := h.manager.GetConfigs()
	cfg, exists := configs[name]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP"})
		return
	}

	client, clientExists := h.manager.GetClient(name)
	status := "disconnected"
	if clientExists {
		status = client.GetStatus()
	} else if h.isEnabled(cfg) {
		status = "disconnected"
	} else {
		status = "disabled"
	}

	// English note.
	toolCount := 0
	if clientExists && client.IsConnected() {
		if count, err := h.manager.GetToolCount(name); err == nil {
			toolCount = count
		}
	}

	// English note.
	errorMsg := ""
	if status == "error" {
		errorMsg = h.manager.GetError(name)
	}

	c.JSON(http.StatusOK, ExternalMCPResponse{
		Config:    cfg,
		Status:    status,
		ToolCount: toolCount,
		Error:     errorMsg,
	})
}

// English note.
func (h *ExternalMCPHandler) AddOrUpdateExternalMCP(c *gin.Context) {
	var req AddOrUpdateExternalMCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ": " + err.Error()})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ""})
		return
	}

	// English note.
	if err := h.validateConfig(req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// English note.
	if err := h.manager.AddOrUpdateConfig(name, req.Config); err != nil {
		h.logger.Error("MCP", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	// English note.
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}

	// English note.
	// English note.
	cfg := req.Config

	if req.Config.Disabled {
		// English note.
		cfg.ExternalMCPEnable = false
		cfg.Disabled = true
		cfg.Enabled = false
	} else if req.Config.Enabled {
		// English note.
		cfg.ExternalMCPEnable = true
		cfg.Enabled = true
		cfg.Disabled = false
	} else if !req.Config.ExternalMCPEnable {
		// English note.
		// English note.
		if existingCfg, exists := h.config.ExternalMCP.Servers[name]; exists {
			// English note.
			cfg.Enabled = existingCfg.Enabled
			cfg.Disabled = existingCfg.Disabled
		}
	} else {
		// English note.
		// English note.
		// English note.
		cfg.Enabled = true
		cfg.Disabled = false
	}

	h.config.ExternalMCP.Servers[name] = cfg

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	h.logger.Info("MCP", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
func (h *ExternalMCPHandler) DeleteExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// English note.
	if err := h.manager.RemoveConfig(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": ""})
		return
	}

	// English note.
	if h.config.ExternalMCP.Servers != nil {
		delete(h.config.ExternalMCP.Servers, name)
	}

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	h.logger.Info("MCP", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": ""})
}

// English note.
func (h *ExternalMCPHandler) StartExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// English note.
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = true
	h.config.ExternalMCP.Servers[name] = cfg

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	// English note.
	h.logger.Info("MCP", zap.String("name", name))
	if err := h.manager.StartClient(name); err != nil {
		h.logger.Error("MCP", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  err.Error(),
			"status": "error",
		})
		return
	}

	// English note.
	client, exists := h.manager.GetClient(name)
	status := "connecting"
	if exists {
		status = client.GetStatus()
	}

	// English note.
	// English note.
	c.JSON(http.StatusOK, gin.H{
		"message": "MCP，",
		"status":  status,
	})
}

// English note.
func (h *ExternalMCPHandler) StopExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// English note.
	if err := h.manager.StopClient(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// English note.
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = false
	h.config.ExternalMCP.Servers[name] = cfg

	// English note.
	if err := h.saveConfig(); err != nil {
		h.logger.Error("", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": ": " + err.Error()})
		return
	}

	h.logger.Info("MCP", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "MCP"})
}

// English note.
func (h *ExternalMCPHandler) GetExternalMCPStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// English note.
func (h *ExternalMCPHandler) validateConfig(cfg config.ExternalMCPServerConfig) error {
	transport := cfg.Transport
	if transport == "" {
		// English note.
		if cfg.Command != "" {
			transport = "stdio"
		} else if cfg.URL != "" {
			transport = "http"
		} else {
			return fmt.Errorf("command（stdio）url（http/sse）")
		}
	}

	switch transport {
	case "http":
		if cfg.URL == "" {
			return fmt.Errorf("HTTPURL")
		}
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("stdiocommand")
		}
	case "sse":
		if cfg.URL == "" {
			return fmt.Errorf("SSEURL")
		}
	default:
		return fmt.Errorf(": %s，: http, stdio, sse", transport)
	}

	return nil
}

// English note.
func (h *ExternalMCPHandler) isEnabled(cfg config.ExternalMCPServerConfig) bool {
	// English note.
	// English note.
	if cfg.ExternalMCPEnable {
		return true
	}
	// English note.
	if cfg.Disabled {
		return false
	}
	if cfg.Enabled {
		return true
	}
	// English note.
	return true
}

// English note.
func (h *ExternalMCPHandler) saveConfig() error {
	// English note.
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	if err := os.WriteFile(h.configPath+".backup", data, 0644); err != nil {
		h.logger.Warn("", zap.Error(err))
	}

	root, err := loadYAMLDocument(h.configPath)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	originalConfigs := make(map[string]map[string]bool)
	externalMCPNode := findMapValue(root.Content[0], "external_mcp")
	if externalMCPNode != nil && externalMCPNode.Kind == yaml.MappingNode {
		serversNode := findMapValue(externalMCPNode, "servers")
		if serversNode != nil && serversNode.Kind == yaml.MappingNode {
			// English note.
			for i := 0; i < len(serversNode.Content); i += 2 {
				if i+1 >= len(serversNode.Content) {
					break
				}
				nameNode := serversNode.Content[i]
				serverNode := serversNode.Content[i+1]
				if nameNode.Kind == yaml.ScalarNode && serverNode.Kind == yaml.MappingNode {
					serverName := nameNode.Value
					originalConfigs[serverName] = make(map[string]bool)
					// English note.
					if enabledVal := findBoolInMap(serverNode, "enabled"); enabledVal != nil {
						originalConfigs[serverName]["enabled"] = *enabledVal
					}
					// English note.
					if disabledVal := findBoolInMap(serverNode, "disabled"); disabledVal != nil {
						originalConfigs[serverName]["disabled"] = *disabledVal
					}
				}
			}
		}
	}

	// English note.
	updateExternalMCPConfig(root, h.config.ExternalMCP, originalConfigs)

	if err := writeYAMLDocument(h.configPath, root); err != nil {
		return fmt.Errorf(": %w", err)
	}

	h.logger.Info("", zap.String("path", h.configPath))
	return nil
}

// English note.
func updateExternalMCPConfig(doc *yaml.Node, cfg config.ExternalMCPConfig, originalConfigs map[string]map[string]bool) {
	root := doc.Content[0]
	externalMCPNode := ensureMap(root, "external_mcp")
	serversNode := ensureMap(externalMCPNode, "servers")

	// English note.
	serversNode.Content = nil

	// English note.
	for name, serverCfg := range cfg.Servers {
		// English note.
		nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
		serverNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		serversNode.Content = append(serversNode.Content, nameNode, serverNode)

		// English note.
		if serverCfg.Command != "" {
			setStringInMap(serverNode, "command", serverCfg.Command)
		}
		if len(serverCfg.Args) > 0 {
			setStringArrayInMap(serverNode, "args", serverCfg.Args)
		}
		// English note.
		if serverCfg.Env != nil && len(serverCfg.Env) > 0 {
			envNode := ensureMap(serverNode, "env")
			for envKey, envValue := range serverCfg.Env {
				setStringInMap(envNode, envKey, envValue)
			}
		}
		if serverCfg.Transport != "" {
			setStringInMap(serverNode, "transport", serverCfg.Transport)
		}
		if serverCfg.URL != "" {
			setStringInMap(serverNode, "url", serverCfg.URL)
		}
		// English note.
		if serverCfg.Headers != nil && len(serverCfg.Headers) > 0 {
			headersNode := ensureMap(serverNode, "headers")
			for k, v := range serverCfg.Headers {
				setStringInMap(headersNode, k, v)
			}
		}
		if serverCfg.Description != "" {
			setStringInMap(serverNode, "description", serverCfg.Description)
		}
		if serverCfg.Timeout > 0 {
			setIntInMap(serverNode, "timeout", serverCfg.Timeout)
		}
		// English note.
		setBoolInMap(serverNode, "external_mcp_enable", serverCfg.ExternalMCPEnable)
		// English note.
		if serverCfg.ToolEnabled != nil && len(serverCfg.ToolEnabled) > 0 {
			toolEnabledNode := ensureMap(serverNode, "tool_enabled")
			for toolName, enabled := range serverCfg.ToolEnabled {
				setBoolInMap(toolEnabledNode, toolName, enabled)
			}
		}
		// English note.
		originalFields, hasOriginal := originalConfigs[name]

		// English note.
		if hasOriginal {
			if enabledVal, hasEnabled := originalFields["enabled"]; hasEnabled {
				setBoolInMap(serverNode, "enabled", enabledVal)
			}
			// English note.
			// English note.
			if disabledVal, hasDisabled := originalFields["disabled"]; hasDisabled {
				if disabledVal {
					setBoolInMap(serverNode, "disabled", disabledVal)
				} else {
					// English note.
					// English note.
					setBoolInMap(serverNode, "enabled", true)
				}
			}
		}

		// English note.
		if serverCfg.Enabled {
			setBoolInMap(serverNode, "enabled", serverCfg.Enabled)
		}
		if serverCfg.Disabled {
			setBoolInMap(serverNode, "disabled", serverCfg.Disabled)
		} else if !hasOriginal && serverCfg.ExternalMCPEnable {
			// English note.
			setBoolInMap(serverNode, "enabled", true)
		}
	}
}

// English note.
func setStringArrayInMap(mapNode *yaml.Node, key string, values []string) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.SequenceNode
	valueNode.Tag = "!!seq"
	valueNode.Content = nil
	for _, v := range values {
		itemNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
		valueNode.Content = append(valueNode.Content, itemNode)
	}
}

// English note.
type AddOrUpdateExternalMCPRequest struct {
	Config config.ExternalMCPServerConfig `json:"config"`
}

// English note.
type ExternalMCPResponse struct {
	Config    config.ExternalMCPServerConfig `json:"config"`
	Status    string                         `json:"status"`          // "connected", "disconnected", "disabled", "error", "connecting"
	ToolCount int                            `json:"tool_count"`      // 
	Error     string                         `json:"error,omitempty"` // （statuserror）
}
