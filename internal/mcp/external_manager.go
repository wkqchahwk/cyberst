package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/google/uuid"

	"go.uber.org/zap"
)

// English note.
type ExternalMCPManager struct {
	clients      map[string]ExternalMCPClient
	configs      map[string]config.ExternalMCPServerConfig
	logger       *zap.Logger
	storage      MonitorStorage            // 
	executions   map[string]*ToolExecution // 
	stats        map[string]*ToolStats     // 
	errors       map[string]string         // 
	toolCounts   map[string]int            // 
	toolCountsMu sync.RWMutex              // 
	toolCache    map[string][]Tool         // ：MCP -> 
	toolCacheMu  sync.RWMutex              // 
	stopRefresh  chan struct{}             // 
	refreshWg    sync.WaitGroup            // goroutine
	mu           sync.RWMutex
}

// English note.
func NewExternalMCPManager(logger *zap.Logger) *ExternalMCPManager {
	return NewExternalMCPManagerWithStorage(logger, nil)
}

// English note.
func NewExternalMCPManagerWithStorage(logger *zap.Logger, storage MonitorStorage) *ExternalMCPManager {
	manager := &ExternalMCPManager{
		clients:     make(map[string]ExternalMCPClient),
		configs:     make(map[string]config.ExternalMCPServerConfig),
		logger:      logger,
		storage:     storage,
		executions:  make(map[string]*ToolExecution),
		stats:       make(map[string]*ToolStats),
		errors:      make(map[string]string),
		toolCounts:  make(map[string]int),
		toolCache:   make(map[string][]Tool),
		stopRefresh: make(chan struct{}),
	}
	// English note.
	manager.startToolCountRefresh()
	return manager
}

// English note.
func (m *ExternalMCPManager) LoadConfigs(cfg *config.ExternalMCPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg == nil || cfg.Servers == nil {
		return
	}

	m.configs = make(map[string]config.ExternalMCPServerConfig)
	for name, serverCfg := range cfg.Servers {
		m.configs[name] = serverCfg
	}
}

// English note.
func (m *ExternalMCPManager) GetConfigs() map[string]config.ExternalMCPServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]config.ExternalMCPServerConfig)
	for k, v := range m.configs {
		result[k] = v
	}
	return result
}

// English note.
func (m *ExternalMCPManager) AddOrUpdateConfig(name string, serverCfg config.ExternalMCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// English note.
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	m.configs[name] = serverCfg

	// English note.
	if m.isEnabled(serverCfg) {
		go m.connectClient(name, serverCfg)
	}

	return nil
}

// English note.
func (m *ExternalMCPManager) RemoveConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// English note.
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	delete(m.configs, name)

	// English note.
	m.toolCountsMu.Lock()
	delete(m.toolCounts, name)
	m.toolCountsMu.Unlock()

	// English note.
	m.toolCacheMu.Lock()
	delete(m.toolCache, name)
	m.toolCacheMu.Unlock()

	return nil
}

// English note.
func (m *ExternalMCPManager) StartClient(name string) error {
	m.mu.Lock()
	serverCfg, exists := m.configs[name]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf(": %s", name)
	}

	// English note.
	m.mu.RLock()
	existingClient, hasClient := m.clients[name]
	m.mu.RUnlock()

	if hasClient {
		// English note.
		if existingClient.IsConnected() {
			// English note.
			// English note.
			m.mu.Lock()
			serverCfg.ExternalMCPEnable = true
			m.configs[name] = serverCfg
			m.mu.Unlock()
			return nil
		}
		// English note.
		existingClient.Close()
		m.mu.Lock()
		delete(m.clients, name)
		m.mu.Unlock()
	}

	// English note.
	m.mu.Lock()
	serverCfg.ExternalMCPEnable = true
	m.configs[name] = serverCfg
	// English note.
	delete(m.errors, name)
	m.mu.Unlock()

	// English note.
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("：")
	}

	// English note.
	m.setClientStatus(client, "connecting")

	// English note.
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	// English note.
	go func() {
		if err := m.doConnect(name, serverCfg, client); err != nil {
			m.logger.Error("MCP",
				zap.String("name", name),
				zap.Error(err),
			)
			// English note.
			m.setClientStatus(client, "error")
			m.mu.Lock()
			m.errors[name] = err.Error()
			m.mu.Unlock()
			// English note.
			m.triggerToolCountRefresh()
		} else {
			// English note.
			m.mu.Lock()
			delete(m.errors, name)
			m.mu.Unlock()
			// English note.
			m.triggerToolCountRefresh()
			m.refreshToolCache(name, client)
			// English note.
			go func() {
				time.Sleep(2 * time.Second)
				m.triggerToolCountRefresh()
				m.refreshToolCache(name, client)
			}()
		}
	}()

	return nil
}

// English note.
func (m *ExternalMCPManager) StopClient(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverCfg, exists := m.configs[name]
	if !exists {
		return fmt.Errorf(": %s", name)
	}

	// English note.
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	// English note.
	delete(m.errors, name)

	// English note.
	m.toolCountsMu.Lock()
	m.toolCounts[name] = 0
	m.toolCountsMu.Unlock()

	// English note.
	serverCfg.ExternalMCPEnable = false
	m.configs[name] = serverCfg

	return nil
}

// English note.
func (m *ExternalMCPManager) GetClient(name string) (ExternalMCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	return client, exists
}

// English note.
func (m *ExternalMCPManager) GetError(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.errors[name]
}

// English note.
// English note.
// English note.
// English note.
// English note.
// English note.
func (m *ExternalMCPManager) GetAllTools(ctx context.Context) ([]Tool, error) {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	var allTools []Tool
	var hasError bool
	var lastError error

	// English note.
	quickCtx, quickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer quickCancel()

	for name, client := range clients {
		tools, err := m.getToolsForClient(name, client, quickCtx)
		if err != nil {
			// English note.
			hasError = true
			if lastError == nil {
				lastError = err
			}
			continue
		}

		// English note.
		for _, tool := range tools {
			tool.Name = fmt.Sprintf("%s::%s", name, tool.Name)
			allTools = append(allTools, tool)
		}
	}

	// English note.
	if hasError && len(allTools) == 0 {
		return nil, fmt.Errorf("MCP: %w", lastError)
	}

	return allTools, nil
}

// English note.
// English note.
func (m *ExternalMCPManager) getToolsForClient(name string, client ExternalMCPClient, ctx context.Context) ([]Tool, error) {
	status := client.GetStatus()

	// English note.
	if status == "error" {
		m.logger.Debug("MCP（）",
			zap.String("name", name),
			zap.String("status", status),
		)
		return nil, fmt.Errorf("MCP: %s", name)
	}

	// English note.
	if client.IsConnected() {
		tools, err := client.ListTools(ctx)
		if err != nil {
			// English note.
			return m.getCachedTools(name, "", err)
		}

		// English note.
		m.updateToolCache(name, tools)
		return tools, nil
	}

	// English note.
	if status == "disconnected" || status == "connecting" {
		return m.getCachedTools(name, fmt.Sprintf("（: %s）", status), nil)
	}

	// English note.
	m.logger.Debug("MCP（）",
		zap.String("name", name),
		zap.String("status", status),
	)
	return nil, fmt.Errorf("MCP: %s (: %s)", name, status)
}

// English note.
func (m *ExternalMCPManager) getCachedTools(name, reason string, originalErr error) ([]Tool, error) {
	m.toolCacheMu.RLock()
	cachedTools, hasCache := m.toolCache[name]
	m.toolCacheMu.RUnlock()

	if hasCache && len(cachedTools) > 0 {
		m.logger.Debug("",
			zap.String("name", name),
			zap.String("reason", reason),
			zap.Int("count", len(cachedTools)),
			zap.Error(originalErr),
		)
		return cachedTools, nil
	}

	// English note.
	if originalErr != nil {
		return nil, fmt.Errorf("MCP: %w", originalErr)
	}
	return nil, fmt.Errorf("MCP: %s", name)
}

// English note.
func (m *ExternalMCPManager) updateToolCache(name string, tools []Tool) {
	m.toolCacheMu.Lock()
	m.toolCache[name] = tools
	m.toolCacheMu.Unlock()

	// English note.
	if len(tools) == 0 {
		m.logger.Warn("MCP",
			zap.String("name", name),
			zap.String("hint", "，"),
		)
	} else {
		m.logger.Debug("",
			zap.String("name", name),
			zap.Int("count", len(tools)),
		)
	}
}

// English note.
func (m *ExternalMCPManager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, string, error) {
	// English note.
	var mcpName, actualToolName string
	if idx := findSubstring(toolName, "::"); idx > 0 {
		mcpName = toolName[:idx]
		actualToolName = toolName[idx+2:]
	} else {
		return nil, "", fmt.Errorf(": %s", toolName)
	}

	client, exists := m.GetClient(mcpName)
	if !exists {
		return nil, "", fmt.Errorf("MCP: %s", mcpName)
	}

	// English note.
	if !client.IsConnected() {
		status := client.GetStatus()
		if status == "error" {
			// English note.
			errorMsg := m.GetError(mcpName)
			if errorMsg != "" {
				return nil, "", fmt.Errorf("MCP: %s (: %s)", mcpName, errorMsg)
			}
			return nil, "", fmt.Errorf("MCP: %s", mcpName)
		}
		return nil, "", fmt.Errorf("MCP: %s (: %s)", mcpName, status)
	}

	// English note.
	executionID := uuid.New().String()
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  toolName, // （MCP）
		Arguments: args,
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.executions[executionID] = execution
	// English note.
	m.cleanupOldExecutions()
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("", zap.Error(err))
		}
	}

	// English note.
	result, err := client.CallTool(ctx, actualToolName, args)

	// English note.
	m.mu.Lock()
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
	} else if result != nil && result.IsError {
		execution.Status = "failed"
		if len(result.Content) > 0 {
			execution.Error = result.Content[0].Text
		} else {
			execution.Error = ""
		}
		execution.Result = result
	} else {
		execution.Status = "completed"
		if result == nil {
			result = &ToolResult{
				Content: []Content{
					{Type: "text", Text: "，"},
				},
			}
		}
		execution.Result = result
	}
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("", zap.Error(err))
		}
	}

	// English note.
	failed := err != nil || (result != nil && result.IsError)
	m.updateStats(toolName, failed)

	// English note.
	if m.storage != nil {
		m.mu.Lock()
		delete(m.executions, executionID)
		m.mu.Unlock()
	}

	if err != nil {
		return nil, executionID, err
	}

	return result, executionID, nil
}

// English note.
func (m *ExternalMCPManager) cleanupOldExecutions() {
	const maxExecutionsInMemory = 1000
	if len(m.executions) <= maxExecutionsInMemory {
		return
	}

	// English note.
	type execTime struct {
		id        string
		startTime time.Time
	}
	var execs []execTime
	for id, exec := range m.executions {
		execs = append(execs, execTime{id: id, startTime: exec.StartTime})
	}

	// English note.
	for i := 0; i < len(execs)-1; i++ {
		for j := i + 1; j < len(execs); j++ {
			if execs[i].startTime.After(execs[j].startTime) {
				execs[i], execs[j] = execs[j], execs[i]
			}
		}
	}

	// English note.
	toDelete := len(m.executions) - maxExecutionsInMemory
	for i := 0; i < toDelete && i < len(execs); i++ {
		delete(m.executions, execs[i].id)
	}
}

// English note.
func (m *ExternalMCPManager) GetExecution(id string) (*ToolExecution, bool) {
	m.mu.RLock()
	exec, exists := m.executions[id]
	m.mu.RUnlock()

	if exists {
		return exec, true
	}

	if m.storage != nil {
		exec, err := m.storage.GetToolExecution(id)
		if err == nil {
			return exec, true
		}
	}

	return nil, false
}

// English note.
func (m *ExternalMCPManager) updateStats(toolName string, failed bool) {
	now := time.Now()
	if m.storage != nil {
		totalCalls := 1
		successCalls := 0
		failedCalls := 0
		if failed {
			failedCalls = 1
		} else {
			successCalls = 1
		}
		if err := m.storage.UpdateToolStats(toolName, totalCalls, successCalls, failedCalls, &now); err != nil {
			m.logger.Warn("", zap.Error(err))
		}
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stats[toolName] == nil {
		m.stats[toolName] = &ToolStats{
			ToolName: toolName,
		}
	}

	stats := m.stats[toolName]
	stats.TotalCalls++
	stats.LastCallTime = &now

	if failed {
		stats.FailedCalls++
	} else {
		stats.SuccessCalls++
	}
}

// English note.
func (m *ExternalMCPManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.configs)
	enabled := 0
	disabled := 0
	connected := 0

	for name, cfg := range m.configs {
		if m.isEnabled(cfg) {
			enabled++
			if client, exists := m.clients[name]; exists && client.IsConnected() {
				connected++
			}
		} else {
			disabled++
		}
	}

	return map[string]interface{}{
		"total":     total,
		"enabled":   enabled,
		"disabled":  disabled,
		"connected": connected,
	}
}

// English note.
// English note.
func (m *ExternalMCPManager) GetToolStats() map[string]*ToolStats {
	result := make(map[string]*ToolStats)

	// English note.
	if m.storage != nil {
		dbStats, err := m.storage.LoadToolStats()
		if err == nil {
			// English note.
			for k, v := range dbStats {
				if findSubstring(k, "::") > 0 {
					result[k] = v
				}
			}
		} else {
			m.logger.Warn("", zap.Error(err))
		}
	}

	// English note.
	m.mu.RLock()
	for k, v := range m.stats {
		// English note.
		if existing, exists := result[k]; exists {
			// English note.
			merged := &ToolStats{
				ToolName:     k,
				TotalCalls:   existing.TotalCalls + v.TotalCalls,
				SuccessCalls: existing.SuccessCalls + v.SuccessCalls,
				FailedCalls:  existing.FailedCalls + v.FailedCalls,
			}
			// English note.
			if v.LastCallTime != nil && (existing.LastCallTime == nil || v.LastCallTime.After(*existing.LastCallTime)) {
				merged.LastCallTime = v.LastCallTime
			} else if existing.LastCallTime != nil {
				timeCopy := *existing.LastCallTime
				merged.LastCallTime = &timeCopy
			}
			result[k] = merged
		} else {
			// English note.
			statCopy := *v
			result[k] = &statCopy
		}
	}
	m.mu.RUnlock()

	return result
}

// English note.
func (m *ExternalMCPManager) GetToolCount(name string) (int, error) {
	// English note.
	m.toolCountsMu.RLock()
	if count, exists := m.toolCounts[name]; exists {
		m.toolCountsMu.RUnlock()
		return count, nil
	}
	m.toolCountsMu.RUnlock()

	// English note.
	client, exists := m.GetClient(name)
	if !exists {
		return 0, fmt.Errorf(": %s", name)
	}

	if !client.IsConnected() {
		// English note.
		m.toolCountsMu.Lock()
		m.toolCounts[name] = 0
		m.toolCountsMu.Unlock()
		return 0, nil
	}

	// English note.
	m.triggerToolCountRefresh()
	return 0, nil
}

// English note.
func (m *ExternalMCPManager) GetToolCounts() map[string]int {
	m.toolCountsMu.RLock()
	defer m.toolCountsMu.RUnlock()

	// English note.
	result := make(map[string]int)
	for k, v := range m.toolCounts {
		result[k] = v
	}
	return result
}

// English note.
func (m *ExternalMCPManager) refreshToolCounts() {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	newCounts := make(map[string]int)

	// English note.
	type countResult struct {
		name  string
		count int
	}
	resultChan := make(chan countResult, len(clients))

	for name, client := range clients {
		go func(n string, c ExternalMCPClient) {
			if !c.IsConnected() {
				resultChan <- countResult{name: n, count: 0}
				return
			}

			// English note.
			// English note.
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			tools, err := c.ListTools(ctx)
			cancel()

			if err != nil {
				errStr := err.Error()
				// English note.
				if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "client is closing") {
					m.logger.Warn("MCP（SSE  tools/list ）",
						zap.String("name", n),
						zap.String("hint", " SSE ， GET  MCP  event: message  JSON-RPC "),
						zap.Error(err),
					)
				} else {
					m.logger.Warn("MCP， tools/list",
						zap.String("name", n),
						zap.Error(err),
					)
				}
				resultChan <- countResult{name: n, count: -1} // -1 
				return
			}

			resultChan <- countResult{name: n, count: len(tools)}
		}(name, client)
	}

	// English note.
	m.toolCountsMu.RLock()
	oldCounts := make(map[string]int)
	for k, v := range m.toolCounts {
		oldCounts[k] = v
	}
	m.toolCountsMu.RUnlock()

	for i := 0; i < len(clients); i++ {
		result := <-resultChan
		if result.count >= 0 {
			newCounts[result.name] = result.count
		} else {
			// English note.
			if oldCount, exists := oldCounts[result.name]; exists {
				newCounts[result.name] = oldCount
			} else {
				newCounts[result.name] = 0
			}
		}
	}

	// English note.
	m.toolCountsMu.Lock()
	// English note.
	for name, count := range newCounts {
		m.toolCounts[name] = count
	}
	// English note.
	for name, client := range clients {
		if !client.IsConnected() {
			m.toolCounts[name] = 0
		}
	}
	m.toolCountsMu.Unlock()
}

// English note.
func (m *ExternalMCPManager) refreshToolCache(name string, client ExternalMCPClient) {
	if !client.IsConnected() {
		return
	}

	// English note.
	status := client.GetStatus()
	if status == "error" {
		m.logger.Debug("（）",
			zap.String("name", name),
			zap.String("status", status),
		)
		return
	}

	// English note.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		m.logger.Debug("",
			zap.String("name", name),
			zap.Error(err),
		)
		// English note.
		return
	}

	// English note.
	m.updateToolCache(name, tools)
}

// English note.
func (m *ExternalMCPManager) startToolCountRefresh() {
	m.refreshWg.Add(1)
	go func() {
		defer m.refreshWg.Done()
		ticker := time.NewTicker(10 * time.Second) // 10
		defer ticker.Stop()

		// English note.
		m.refreshToolCounts()

		for {
			select {
			case <-ticker.C:
				m.refreshToolCounts()
			case <-m.stopRefresh:
				return
			}
		}
	}()
}

// English note.
func (m *ExternalMCPManager) triggerToolCountRefresh() {
	go m.refreshToolCounts()
}

// English note.
func (m *ExternalMCPManager) createClient(serverCfg config.ExternalMCPServerConfig) ExternalMCPClient {
	transport := serverCfg.Transport
	if transport == "" {
		if serverCfg.Command != "" {
			transport = "stdio"
		} else if serverCfg.URL != "" {
			transport = "http"
		} else {
			return nil
		}
	}

	switch transport {
	case "http":
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "simple_http":
		// English note.
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "stdio":
		if serverCfg.Command == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "sse":
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	default:
		return nil
	}
}

// English note.
func (m *ExternalMCPManager) doConnect(name string, serverCfg config.ExternalMCPServerConfig, client ExternalMCPClient) error {
	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// English note.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		return err
	}

	m.logger.Info("MCP",
		zap.String("name", name),
	)

	return nil
}

// English note.
func (m *ExternalMCPManager) setClientStatus(client ExternalMCPClient, status string) {
	if c, ok := client.(*lazySDKClient); ok {
		c.setStatus(status)
	}
}

// English note.
func (m *ExternalMCPManager) connectClient(name string, serverCfg config.ExternalMCPServerConfig) error {
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("：")
	}

	// English note.
	m.setClientStatus(client, "connecting")

	// English note.
	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		m.logger.Error("MCP",
			zap.String("name", name),
			zap.Error(err),
		)
		return err
	}

	// English note.
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	m.logger.Info("MCP",
		zap.String("name", name),
	)

	// English note.
	m.triggerToolCountRefresh()
	m.mu.RLock()
	if client, exists := m.clients[name]; exists {
		m.refreshToolCache(name, client)
	}
	m.mu.RUnlock()

	return nil
}

// English note.
func (m *ExternalMCPManager) isEnabled(cfg config.ExternalMCPServerConfig) bool {
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
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// English note.
func (m *ExternalMCPManager) StartAllEnabled() {
	m.mu.RLock()
	configs := make(map[string]config.ExternalMCPServerConfig)
	for k, v := range m.configs {
		configs[k] = v
	}
	m.mu.RUnlock()

	for name, cfg := range configs {
		if m.isEnabled(cfg) {
			go func(n string, c config.ExternalMCPServerConfig) {
				if err := m.connectClient(n, c); err != nil {
					// English note.
					errStr := strings.ToLower(err.Error())
					isConnectionRefused := strings.Contains(errStr, "connection refused") ||
						strings.Contains(errStr, "dial tcp") ||
						strings.Contains(errStr, "connect: connection refused")

					if isConnectionRefused {
						// English note.
						// English note.
						fields := []zap.Field{
							zap.String("name", n),
							zap.String("message", "，。，"),
							zap.Error(err),
						}

						// English note.
						transport := c.Transport
						if transport == "" {
							if c.Command != "" {
								transport = "stdio"
							} else if c.URL != "" {
								transport = "http"
							}
						}

						if transport == "http" && c.URL != "" {
							fields = append(fields, zap.String("url", c.URL))
						} else if transport == "stdio" && c.Command != "" {
							fields = append(fields, zap.String("command", c.Command))
						}

						m.logger.Warn("MCP", fields...)
					} else {
						// English note.
						m.logger.Error("MCP",
							zap.String("name", n),
							zap.Error(err),
						)
					}
				}
			}(name, cfg)
		}
	}
}

// English note.
func (m *ExternalMCPManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}

	// English note.
	m.toolCountsMu.Lock()
	m.toolCounts = make(map[string]int)
	m.toolCountsMu.Unlock()

	// English note.
	m.toolCacheMu.Lock()
	m.toolCache = make(map[string][]Tool)
	m.toolCacheMu.Unlock()

	// English note.
	select {
	case <-m.stopRefresh:
		// English note.
	default:
		close(m.stopRefresh)
		m.refreshWg.Wait()
	}
}
