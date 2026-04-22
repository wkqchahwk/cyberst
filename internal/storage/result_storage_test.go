package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// English note.
func setupTestStorage(t *testing.T) (*FileResultStorage, string) {
	tmpDir := filepath.Join(os.TempDir(), "test_result_storage_"+time.Now().Format("20060102_150405"))
	logger := zap.NewNop()

	storage, err := NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	return storage, tmpDir
}

// English note.
func cleanupTestStorage(t *testing.T, tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Logf(": %v", err)
	}
}

func TestNewFileResultStorage(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_new_storage_"+time.Now().Format("20060102_150405"))
	defer cleanupTestStorage(t, tmpDir)

	logger := zap.NewNop()
	storage, err := NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if storage == nil {
		t.Fatal("nil")
	}

	// English note.
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatal("")
	}
}

func TestFileResultStorage_SaveResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_001"
	toolName := "nmap_scan"
	result := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"

	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	resultPath := filepath.Join(tmpDir, executionID+".txt")
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Fatal("")
	}

	// English note.
	metadataPath := filepath.Join(tmpDir, executionID+".meta.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("")
	}
}

func TestFileResultStorage_GetResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_002"
	toolName := "test_tool"
	expectedResult := "Test result content\nLine 2\nLine 3"

	// English note.
	err := storage.SaveResult(executionID, toolName, expectedResult)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	result, err := storage.GetResult(executionID)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if result != expectedResult {
		t.Errorf("。: %q, : %q", expectedResult, result)
	}

	// English note.
	_, err = storage.GetResult("nonexistent_id")
	if err == nil {
		t.Fatal("")
	}
}

func TestFileResultStorage_GetResultMetadata(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_003"
	toolName := "test_tool"
	result := "Line 1\nLine 2\nLine 3"

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	metadata, err := storage.GetResultMetadata(executionID)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if metadata.ExecutionID != executionID {
		t.Errorf("ID。: %s, : %s", executionID, metadata.ExecutionID)
	}

	if metadata.ToolName != toolName {
		t.Errorf("。: %s, : %s", toolName, metadata.ToolName)
	}

	if metadata.TotalSize != len(result) {
		t.Errorf("。: %d, : %d", len(result), metadata.TotalSize)
	}

	expectedLines := len(strings.Split(result, "\n"))
	if metadata.TotalLines != expectedLines {
		t.Errorf("。: %d, : %d", expectedLines, metadata.TotalLines)
	}

	// English note.
	now := time.Now()
	if metadata.CreatedAt.After(now) || metadata.CreatedAt.Before(now.Add(-time.Second)) {
		t.Errorf(": %v", metadata.CreatedAt)
	}
}

func TestFileResultStorage_GetResultPage(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_004"
	toolName := "test_tool"
	// English note.
	lines := make([]string, 10)
	for i := 0; i < 10; i++ {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	result := strings.Join(lines, "\n")

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	page, err := storage.GetResultPage(executionID, 1, 3)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if page.Page != 1 {
		t.Errorf("。: 1, : %d", page.Page)
	}

	if page.Limit != 3 {
		t.Errorf("。: 3, : %d", page.Limit)
	}

	if page.TotalLines != 10 {
		t.Errorf("。: 10, : %d", page.TotalLines)
	}

	if page.TotalPages != 4 {
		t.Errorf("。: 4, : %d", page.TotalPages)
	}

	if len(page.Lines) != 3 {
		t.Errorf("。: 3, : %d", len(page.Lines))
	}

	if page.Lines[0] != "Line 1" {
		t.Errorf("。: Line 1, : %s", page.Lines[0])
	}

	// English note.
	page2, err := storage.GetResultPage(executionID, 2, 3)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(page2.Lines) != 3 {
		t.Errorf("。: 3, : %d", len(page2.Lines))
	}

	if page2.Lines[0] != "Line 4" {
		t.Errorf("。: Line 4, : %s", page2.Lines[0])
	}

	// English note.
	page4, err := storage.GetResultPage(executionID, 4, 3)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(page4.Lines) != 1 {
		t.Errorf("。: 1, : %d", len(page4.Lines))
	}

	// English note.
	page5, err := storage.GetResultPage(executionID, 5, 3)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	if page5.Page != 4 {
		t.Errorf("。: 4, : %d", page5.Page)
	}

	// English note.
	if len(page5.Lines) != 1 {
		t.Errorf("1。: %d", len(page5.Lines))
	}
}

func TestFileResultStorage_SearchResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_005"
	toolName := "test_tool"
	result := "Line 1: error occurred\nLine 2: success\nLine 3: error again\nLine 4: ok"

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	matchedLines, err := storage.SearchResult(executionID, "error", false)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(matchedLines) != 2 {
		t.Errorf("。: 2, : %d", len(matchedLines))
	}

	// English note.
	for i, line := range matchedLines {
		if !strings.Contains(line, "error") {
			t.Errorf("%d: %s", i+1, line)
		}
	}

	// English note.
	noMatch, err := storage.SearchResult(executionID, "nonexistent", false)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(noMatch) != 0 {
		t.Errorf("。: %d", len(noMatch))
	}

	// English note.
	regexMatched, err := storage.SearchResult(executionID, "error.*again", true)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(regexMatched) != 1 {
		t.Errorf("。: 1, : %d", len(regexMatched))
	}
}

func TestFileResultStorage_FilterResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_006"
	toolName := "test_tool"
	result := "Line 1: warning message\nLine 2: info message\nLine 3: warning again\nLine 4: debug message"

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	filteredLines, err := storage.FilterResult(executionID, "warning", false)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if len(filteredLines) != 2 {
		t.Errorf("。: 2, : %d", len(filteredLines))
	}

	// English note.
	for i, line := range filteredLines {
		if !strings.Contains(line, "warning") {
			t.Errorf("%d: %s", i+1, line)
		}
	}
}

func TestFileResultStorage_DeleteResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_007"
	toolName := "test_tool"
	result := "Test result"

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	resultPath := filepath.Join(tmpDir, executionID+".txt")
	metadataPath := filepath.Join(tmpDir, executionID+".meta.json")

	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Fatal("")
	}

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("")
	}

	// English note.
	err = storage.DeleteResult(executionID)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
		t.Fatal("")
	}

	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Fatal("")
	}

	// English note.
	err = storage.DeleteResult("nonexistent_id")
	if err != nil {
		t.Errorf("ID: %v", err)
	}
}

func TestFileResultStorage_ConcurrentAccess(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	// English note.
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			executionID := fmt.Sprintf("test_exec_%d", id)
			toolName := "test_tool"
			result := fmt.Sprintf("Result %d\nLine 2\nLine 3", id)

			err := storage.SaveResult(executionID, toolName, result)
			if err != nil {
				t.Errorf(" (ID: %s): %v", executionID, err)
			}

			// English note.
			_, err = storage.GetResult(executionID)
			if err != nil {
				t.Errorf(" (ID: %s): %v", executionID, err)
			}

			done <- true
		}(i)
	}

	// English note.
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFileResultStorage_LargeResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_large"
	toolName := "test_tool"

	// English note.
	lines := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		lines[i] = fmt.Sprintf("Line %d: This is a test line with some content", i+1)
	}
	result := strings.Join(lines, "\n")

	// English note.
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	// English note.
	metadata, err := storage.GetResultMetadata(executionID)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if metadata.TotalLines != 1000 {
		t.Errorf("。: 1000, : %d", metadata.TotalLines)
	}

	// English note.
	page, err := storage.GetResultPage(executionID, 1, 100)
	if err != nil {
		t.Fatalf(": %v", err)
	}

	if page.TotalPages != 10 {
		t.Errorf("。: 10, : %d", page.TotalPages)
	}

	if len(page.Lines) != 100 {
		t.Errorf("。: 100, : %d", len(page.Lines))
	}
}
