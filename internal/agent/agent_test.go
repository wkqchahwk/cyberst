package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// English note.
func setupTestAgent(t *testing.T) (*Agent, *storage.FileResultStorage) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	
	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}
	
	agentCfg := &config.AgentConfig{
		MaxIterations:        10,
		LargeResultThreshold: 100, // 
		ResultStorageDir:     "",
	}
	
	agent := NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 10)
	
	// English note.
	tmpDir := filepath.Join(os.TempDir(), "test_agent_storage_"+time.Now().Format("20060102_150405"))
	testStorage, err := storage.NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	agent.SetResultStorage(testStorage)
	
	return agent, testStorage
}

func TestAgent_FormatMinimalNotification(t *testing.T) {
	agent, testStorage := setupTestAgent(t)
	_ = testStorage // 
	
	executionID := "test_exec_001"
	toolName := "nmap_scan"
	size := 50000
	lineCount := 1000
	filePath := "tmp/test_exec_001.txt"
	
	notification := agent.formatMinimalNotification(executionID, toolName, size, lineCount, filePath)
	
	// English note.
	if !strings.Contains(notification, executionID) {
		t.Errorf("ID: %s", executionID)
	}
	
	if !strings.Contains(notification, toolName) {
		t.Errorf(": %s", toolName)
	}
	
	if !strings.Contains(notification, "50000") {
		t.Errorf("")
	}
	
	if !strings.Contains(notification, "1000") {
		t.Errorf("")
	}
	
	if !strings.Contains(notification, "query_execution_result") {
		t.Errorf("")
	}
}

func TestAgent_ExecuteToolViaMCP_LargeResult(t *testing.T) {
	agent, _ := setupTestAgent(t)
	
	// English note.
	largeResult := &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: strings.Repeat("This is a test line with some content.\n", 1000), // 50KB
			},
		},
		IsError: false,
	}
	
	// English note.
	// English note.
	// English note.
	
	// English note.
	agent.mu.Lock()
	agent.largeResultThreshold = 1000 // 
	agent.mu.Unlock()
	
	// English note.
	executionID := "test_exec_large_001"
	toolName := "test_tool"
	
	// English note.
	var resultText strings.Builder
	for _, content := range largeResult.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}
	
	resultStr := resultText.String()
	resultSize := len(resultStr)
	
	// English note.
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	storage := agent.resultStorage
	agent.mu.RUnlock()
	
	if resultSize > threshold && storage != nil {
		// English note.
		err := storage.SaveResult(executionID, toolName, resultStr)
		if err != nil {
			t.Fatalf(": %v", err)
		}
		
		// English note.
		lines := strings.Split(resultStr, "\n")
		filePath := storage.GetResultPath(executionID)
		notification := agent.formatMinimalNotification(executionID, toolName, resultSize, len(lines), filePath)
		
		// English note.
		if !strings.Contains(notification, executionID) {
			t.Errorf("ID")
		}
		
		// English note.
		savedResult, err := storage.GetResult(executionID)
		if err != nil {
			t.Fatalf(": %v", err)
		}
		
		if savedResult != resultStr {
			t.Errorf("")
		}
	} else {
		t.Fatal("")
	}
}

func TestAgent_ExecuteToolViaMCP_SmallResult(t *testing.T) {
	agent, _ := setupTestAgent(t)
	
	// English note.
	smallResult := &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Small result content",
			},
		},
		IsError: false,
	}
	
	// English note.
	agent.mu.Lock()
	agent.largeResultThreshold = 100000 // 100KB
	agent.mu.Unlock()
	
	// English note.
	var resultText strings.Builder
	for _, content := range smallResult.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}
	
	resultStr := resultText.String()
	resultSize := len(resultStr)
	
	// English note.
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	storage := agent.resultStorage
	agent.mu.RUnlock()
	
	if resultSize > threshold && storage != nil {
		t.Fatal("")
	}
	
	// English note.
	if resultSize <= threshold {
		// English note.
		if resultStr == "" {
			t.Fatal("，")
		}
	}
}

func TestAgent_SetResultStorage(t *testing.T) {
	agent, _ := setupTestAgent(t)
	
	// English note.
	tmpDir := filepath.Join(os.TempDir(), "test_new_storage_"+time.Now().Format("20060102_150405"))
	newStorage, err := storage.NewFileResultStorage(tmpDir, zap.NewNop())
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	// English note.
	agent.SetResultStorage(newStorage)
	
	// English note.
	agent.mu.RLock()
	currentStorage := agent.resultStorage
	agent.mu.RUnlock()
	
	if currentStorage != newStorage {
		t.Fatal("")
	}
	
	// English note.
	os.RemoveAll(tmpDir)
}

func TestAgent_NewAgent_DefaultValues(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	
	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}
	
	// English note.
	agent := NewAgent(openAICfg, nil, mcpServer, nil, logger, 0)
	
	if agent.maxIterations != 30 {
		t.Errorf("。: 30, : %d", agent.maxIterations)
	}
	
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	agent.mu.RUnlock()
	
	if threshold != 50*1024 {
		t.Errorf("。: %d, : %d", 50*1024, threshold)
	}
}

func TestAgent_NewAgent_CustomConfig(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	
	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}
	
	agentCfg := &config.AgentConfig{
		MaxIterations:        20,
		LargeResultThreshold: 100 * 1024, // 100KB
		ResultStorageDir:     "custom_tmp",
	}
	
	agent := NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 15)
	
	if agent.maxIterations != 15 {
		t.Errorf("。: 15, : %d", agent.maxIterations)
	}
	
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	agent.mu.RUnlock()
	
	if threshold != 100*1024 {
		t.Errorf("。: %d, : %d", 100*1024, threshold)
	}
}

