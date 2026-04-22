package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// English note.
type ResultStorage interface {
	// English note.
	SaveResult(executionID string, toolName string, result string) error

	// English note.
	GetResult(executionID string) (string, error)

	// English note.
	GetResultPage(executionID string, page int, limit int) (*ResultPage, error)

	// English note.
	// English note.
	SearchResult(executionID string, keyword string, useRegex bool) ([]string, error)

	// English note.
	// English note.
	FilterResult(executionID string, filter string, useRegex bool) ([]string, error)

	// English note.
	GetResultMetadata(executionID string) (*ResultMetadata, error)

	// English note.
	GetResultPath(executionID string) string

	// English note.
	DeleteResult(executionID string) error
}

// English note.
type ResultPage struct {
	Lines      []string `json:"lines"`
	Page       int      `json:"page"`
	Limit      int      `json:"limit"`
	TotalLines int      `json:"total_lines"`
	TotalPages int      `json:"total_pages"`
}

// English note.
type ResultMetadata struct {
	ExecutionID string    `json:"execution_id"`
	ToolName    string    `json:"tool_name"`
	TotalSize   int       `json:"total_size"`
	TotalLines  int       `json:"total_lines"`
	CreatedAt   time.Time `json:"created_at"`
}

// English note.
type FileResultStorage struct {
	baseDir string
	logger  *zap.Logger
	mu      sync.RWMutex
}

// English note.
func NewFileResultStorage(baseDir string, logger *zap.Logger) (*FileResultStorage, error) {
	// English note.
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return &FileResultStorage{
		baseDir: baseDir,
		logger:  logger,
	}, nil
}

// English note.
func (s *FileResultStorage) getResultPath(executionID string) string {
	return filepath.Join(s.baseDir, executionID+".txt")
}

// English note.
func (s *FileResultStorage) getMetadataPath(executionID string) string {
	return filepath.Join(s.baseDir, executionID+".meta.json")
}

// English note.
func (s *FileResultStorage) SaveResult(executionID string, toolName string, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// English note.
	resultPath := s.getResultPath(executionID)
	if err := os.WriteFile(resultPath, []byte(result), 0644); err != nil {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	lines := strings.Split(result, "\n")
	metadata := &ResultMetadata{
		ExecutionID: executionID,
		ToolName:    toolName,
		TotalSize:   len(result),
		TotalLines:  len(lines),
		CreatedAt:   time.Now(),
	}

	// English note.
	metadataPath := s.getMetadataPath(executionID)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf(": %w", err)
	}

	s.logger.Info("",
		zap.String("executionID", executionID),
		zap.String("toolName", toolName),
		zap.Int("size", len(result)),
		zap.Int("lines", len(lines)),
	)

	return nil
}

// English note.
func (s *FileResultStorage) GetResult(executionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resultPath := s.getResultPath(executionID)
	data, err := os.ReadFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(": %s", executionID)
		}
		return "", fmt.Errorf(": %w", err)
	}

	return string(data), nil
}

// English note.
func (s *FileResultStorage) GetResultMetadata(executionID string) (*ResultMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadataPath := s.getMetadataPath(executionID)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(": %s", executionID)
		}
		return nil, fmt.Errorf(": %w", err)
	}

	var metadata ResultMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return &metadata, nil
}

// English note.
func (s *FileResultStorage) GetResultPage(executionID string, page int, limit int) (*ResultPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// English note.
	result, err := s.GetResult(executionID)
	if err != nil {
		return nil, err
	}

	// English note.
	lines := strings.Split(result, "\n")
	totalLines := len(lines)

	// English note.
	totalPages := (totalLines + limit - 1) / limit
	if page < 1 {
		page = 1
	}
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	// English note.
	start := (page - 1) * limit
	end := start + limit
	if end > totalLines {
		end = totalLines
	}

	// English note.
	var pageLines []string
	if start < totalLines {
		pageLines = lines[start:end]
	} else {
		pageLines = []string{}
	}

	return &ResultPage{
		Lines:      pageLines,
		Page:       page,
		Limit:      limit,
		TotalLines: totalLines,
		TotalPages: totalPages,
	}, nil
}

// English note.
func (s *FileResultStorage) SearchResult(executionID string, keyword string, useRegex bool) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// English note.
	result, err := s.GetResult(executionID)
	if err != nil {
		return nil, err
	}

	// English note.
	var regex *regexp.Regexp
	if useRegex {
		compiledRegex, err := regexp.Compile(keyword)
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}
		regex = compiledRegex
	}

	// English note.
	lines := strings.Split(result, "\n")
	var matchedLines []string

	for _, line := range lines {
		var matched bool
		if useRegex {
			matched = regex.MatchString(line)
		} else {
			matched = strings.Contains(line, keyword)
		}

		if matched {
			matchedLines = append(matchedLines, line)
		}
	}

	return matchedLines, nil
}

// English note.
func (s *FileResultStorage) FilterResult(executionID string, filter string, useRegex bool) ([]string, error) {
	// English note.
	return s.SearchResult(executionID, filter, useRegex)
}

// English note.
func (s *FileResultStorage) GetResultPath(executionID string) string {
	return s.getResultPath(executionID)
}

// English note.
func (s *FileResultStorage) DeleteResult(executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resultPath := s.getResultPath(executionID)
	metadataPath := s.getMetadataPath(executionID)

	// English note.
	if err := os.Remove(resultPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(": %w", err)
	}

	s.logger.Info("",
		zap.String("executionID", executionID),
	)

	return nil
}
