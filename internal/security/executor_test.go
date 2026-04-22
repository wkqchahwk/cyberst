package security

import (
	"context"
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
func setupTestExecutor(t *testing.T) (*Executor, *mcp.Server) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	
	cfg := &config.SecurityConfig{
		Tools: []config.ToolConfig{},
	}
	
	executor := NewExecutor(cfg, mcpServer, logger)
	return executor, mcpServer
}

// English note.
func setupTestStorage(t *testing.T) *storage.FileResultStorage {
	tmpDir := filepath.Join(os.TempDir(), "test_executor_storage_"+time.Now().Format("20060102_150405"))
	logger := zap.NewNop()
	
	storage, err := storage.NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	return storage
}

func TestExecutor_ExecuteInternalTool_QueryExecutionResult(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	testStorage := setupTestStorage(t)
	executor.SetResultStorage(testStorage)
	
	// English note.
	executionID := "test_exec_001"
	toolName := "nmap_scan"
	result := "Line 1: Port 22 open\nLine 2: Port 80 open\nLine 3: Port 443 open\nLine 4: error occurred"
	
	// English note.
	err := testStorage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	ctx := context.Background()
	
	// English note.
	args := map[string]interface{}{
		"execution_id": executionID,
		"page":         float64(1),
		"limit":        float64(2),
	}
	
	toolResult, err := executor.executeQueryExecutionResult(ctx, args)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if toolResult.IsError {
		t.Fatalf("，: %s", toolResult.Content[0].Text)
	}
	
	// English note.
	resultText := toolResult.Content[0].Text
	if !strings.Contains(resultText, executionID) {
		t.Errorf("ID: %s", executionID)
	}
	
	if !strings.Contains(resultText, " 1/") {
		t.Errorf("")
	}
	
	// English note.
	args2 := map[string]interface{}{
		"execution_id": executionID,
		"search":       "error",
		"page":         float64(1),
		"limit":        float64(10),
	}
	
	toolResult2, err := executor.executeQueryExecutionResult(ctx, args2)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if toolResult2.IsError {
		t.Fatalf("，: %s", toolResult2.Content[0].Text)
	}
	
	resultText2 := toolResult2.Content[0].Text
	if !strings.Contains(resultText2, "error") {
		t.Errorf(": error")
	}
	
	// English note.
	args3 := map[string]interface{}{
		"execution_id": executionID,
		"filter":       "Port",
		"page":         float64(1),
		"limit":        float64(10),
	}
	
	toolResult3, err := executor.executeQueryExecutionResult(ctx, args3)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if toolResult3.IsError {
		t.Fatalf("，: %s", toolResult3.Content[0].Text)
	}
	
	resultText3 := toolResult3.Content[0].Text
	if !strings.Contains(resultText3, "Port") {
		t.Errorf(": Port")
	}
	
	// English note.
	args4 := map[string]interface{}{
		"page": float64(1),
	}
	
	toolResult4, err := executor.executeQueryExecutionResult(ctx, args4)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if !toolResult4.IsError {
		t.Fatal("execution_id")
	}
	
	// English note.
	args5 := map[string]interface{}{
		"execution_id": "nonexistent_id",
		"page":         float64(1),
	}
	
	toolResult5, err := executor.executeQueryExecutionResult(ctx, args5)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if !toolResult5.IsError {
		t.Fatal("ID")
	}
}

func TestExecutor_ExecuteInternalTool_UnknownTool(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	
	ctx := context.Background()
	args := map[string]interface{}{
		"test": "value",
	}
	
	// English note.
	toolResult, err := executor.executeInternalTool(ctx, "unknown_tool", "internal:unknown_tool", args)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if !toolResult.IsError {
		t.Fatal("")
	}
	
	if !strings.Contains(toolResult.Content[0].Text, "") {
		t.Errorf("''")
	}
}

func TestExecutor_ExecuteInternalTool_NoStorage(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	// English note.
	
	ctx := context.Background()
	args := map[string]interface{}{
		"execution_id": "test_id",
	}
	
	toolResult, err := executor.executeQueryExecutionResult(ctx, args)
	if err != nil {
		t.Fatalf(": %v", err)
	}
	
	if !toolResult.IsError {
		t.Fatal("")
	}
	
	if !strings.Contains(toolResult.Content[0].Text, "") {
		t.Errorf("''")
	}
}

func TestPaginateLines(t *testing.T) {
	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}
	
	// English note.
	page := paginateLines(lines, 1, 2)
	if page.Page != 1 {
		t.Errorf("。: 1, : %d", page.Page)
	}
	if page.Limit != 2 {
		t.Errorf("。: 2, : %d", page.Limit)
	}
	if page.TotalLines != 5 {
		t.Errorf("。: 5, : %d", page.TotalLines)
	}
	if page.TotalPages != 3 {
		t.Errorf("。: 3, : %d", page.TotalPages)
	}
	if len(page.Lines) != 2 {
		t.Errorf("。: 2, : %d", len(page.Lines))
	}
	
	// English note.
	page2 := paginateLines(lines, 2, 2)
	if len(page2.Lines) != 2 {
		t.Errorf("。: 2, : %d", len(page2.Lines))
	}
	if page2.Lines[0] != "Line 3" {
		t.Errorf("。: Line 3, : %s", page2.Lines[0])
	}
	
	// English note.
	page3 := paginateLines(lines, 3, 2)
	if len(page3.Lines) != 1 {
		t.Errorf("。: 1, : %d", len(page3.Lines))
	}
	
	// English note.
	page4 := paginateLines(lines, 4, 2)
	if page4.Page != 3 {
		t.Errorf("。: 3, : %d", page4.Page)
	}
	if len(page4.Lines) != 1 {
		t.Errorf("1。: %d", len(page4.Lines))
	}
	
	// English note.
	page0 := paginateLines(lines, 0, 2)
	if page0.Page != 1 {
		t.Errorf("1。: %d", page0.Page)
	}
	
	// English note.
	emptyPage := paginateLines([]string{}, 1, 10)
	if emptyPage.TotalLines != 0 {
		t.Errorf("0。: %d", emptyPage.TotalLines)
	}
	if len(emptyPage.Lines) != 0 {
		t.Errorf("。: %d", len(emptyPage.Lines))
	}
}

