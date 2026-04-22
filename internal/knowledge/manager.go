package knowledge

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// English note.
type Manager struct {
	db       *sql.DB
	basePath string
	logger   *zap.Logger
}

// English note.
func NewManager(db *sql.DB, basePath string, logger *zap.Logger) *Manager {
	return &Manager{
		db:       db,
		basePath: basePath,
		logger:   logger,
	}
}

// English note.
// English note.
func (m *Manager) ScanKnowledgeBase() ([]string, error) {
	if m.basePath == "" {
		return nil, fmt.Errorf("")
	}

	// English note.
	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	var itemsToIndex []string

	// English note.
	err := filepath.WalkDir(m.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// English note.
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// English note.
		relPath, err := filepath.Rel(m.basePath, path)
		if err != nil {
			return err
		}

		// English note.
		parts := strings.Split(relPath, string(filepath.Separator))
		category := ""
		if len(parts) > 1 {
			category = parts[0]
		}

		// English note.
		title := strings.TrimSuffix(filepath.Base(path), ".md")

		// English note.
		content, err := os.ReadFile(path)
		if err != nil {
			m.logger.Warn("", zap.String("path", path), zap.Error(err))
			return nil // 
		}

		// English note.
		var existingID string
		var existingContent string
		var existingUpdatedAt time.Time
		err = m.db.QueryRow(
			"SELECT id, content, updated_at FROM knowledge_base_items WHERE file_path = ?",
			path,
		).Scan(&existingID, &existingContent, &existingUpdatedAt)

		if err == sql.ErrNoRows {
			// English note.
			id := uuid.New().String()
			now := time.Now()
			_, err = m.db.Exec(
				"INSERT INTO knowledge_base_items (id, category, title, file_path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
				id, category, title, path, string(content), now, now,
			)
			if err != nil {
				return fmt.Errorf(": %w", err)
			}
			m.logger.Info("", zap.String("id", id), zap.String("title", title), zap.String("category", category))
			// English note.
			itemsToIndex = append(itemsToIndex, id)
		} else if err == nil {
			// English note.
			contentChanged := existingContent != string(content)
			if contentChanged {
				// English note.
				_, err = m.db.Exec(
					"UPDATE knowledge_base_items SET category = ?, title = ?, content = ?, updated_at = ? WHERE id = ?",
					category, title, string(content), time.Now(), existingID,
				)
				if err != nil {
					return fmt.Errorf(": %w", err)
				}
				m.logger.Info("", zap.String("id", existingID), zap.String("title", title))
				// English note.
				itemsToIndex = append(itemsToIndex, existingID)
			} else {
				m.logger.Debug("，", zap.String("id", existingID), zap.String("title", title))
			}
		} else {
			return fmt.Errorf(": %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return itemsToIndex, nil
}

// English note.
func (m *Manager) GetCategories() ([]string, error) {
	rows, err := m.db.Query("SELECT DISTINCT category FROM knowledge_base_items ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}
		categories = append(categories, category)
	}

	return categories, nil
}

// English note.
func (m *Manager) GetStats() (int, int, error) {
	// English note.
	categories, err := m.GetCategories()
	if err != nil {
		return 0, 0, fmt.Errorf(": %w", err)
	}
	totalCategories := len(categories)

	// English note.
	var totalItems int
	err = m.db.QueryRow("SELECT COUNT(*) FROM knowledge_base_items").Scan(&totalItems)
	if err != nil {
		return totalCategories, 0, fmt.Errorf(": %w", err)
	}

	return totalCategories, totalItems, nil
}

// English note.
// English note.
// English note.
func (m *Manager) GetCategoriesWithItems(limit, offset int) ([]*CategoryWithItems, int, error) {
	// English note.
	rows, err := m.db.Query(`
		SELECT category, COUNT(*) as item_count 
		FROM knowledge_base_items 
		GROUP BY category 
		ORDER BY category
	`)
	if err != nil {
		return nil, 0, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	// English note.
	type categoryInfo struct {
		name      string
		itemCount int
	}
	var allCategories []categoryInfo
	for rows.Next() {
		var info categoryInfo
		if err := rows.Scan(&info.name, &info.itemCount); err != nil {
			return nil, 0, fmt.Errorf(": %w", err)
		}
		allCategories = append(allCategories, info)
	}

	totalCategories := len(allCategories)

	// English note.
	var paginatedCategories []categoryInfo
	if limit > 0 {
		start := offset
		end := offset + limit
		if start >= totalCategories {
			paginatedCategories = []categoryInfo{}
		} else {
			if end > totalCategories {
				end = totalCategories
			}
			paginatedCategories = allCategories[start:end]
		}
	} else {
		paginatedCategories = allCategories
	}

	// English note.
	result := make([]*CategoryWithItems, 0, len(paginatedCategories))
	for _, catInfo := range paginatedCategories {
		// English note.
		items, _, err := m.GetItemsSummary(catInfo.name, 0, 0)
		if err != nil {
			return nil, 0, fmt.Errorf(" %s : %w", catInfo.name, err)
		}

		result = append(result, &CategoryWithItems{
			Category:  catInfo.name,
			ItemCount: catInfo.itemCount,
			Items:     items,
		})
	}

	return result, totalCategories, nil
}

// English note.
func (m *Manager) GetItems(category string) ([]*KnowledgeItem, error) {
	return m.GetItemsWithOptions(category, 0, 0, true)
}

// English note.
// English note.
// English note.
// English note.
// English note.
func (m *Manager) GetItemsWithOptions(category string, limit, offset int, includeContent bool) ([]*KnowledgeItem, error) {
	var rows *sql.Rows
	var err error

	// English note.
	var query string
	var args []interface{}

	if includeContent {
		query = "SELECT id, category, title, file_path, content, created_at, updated_at FROM knowledge_base_items"
	} else {
		query = "SELECT id, category, title, file_path, created_at, updated_at FROM knowledge_base_items"
	}

	if category != "" {
		query += " WHERE category = ?"
		args = append(args, category)
	}

	query += " ORDER BY category, title"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	rows, err = m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	var items []*KnowledgeItem
	for rows.Next() {
		item := &KnowledgeItem{}
		var createdAt, updatedAt string

		if includeContent {
			if err := rows.Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &item.Content, &createdAt, &updatedAt); err != nil {
				return nil, fmt.Errorf(": %w", err)
			}
		} else {
			if err := rows.Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &createdAt, &updatedAt); err != nil {
				return nil, fmt.Errorf(": %w", err)
			}
			// English note.
			item.Content = ""
		}

		// English note.
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		// English note.
		if createdAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, createdAt)
				if err == nil && !parsed.IsZero() {
					item.CreatedAt = parsed
					break
				}
			}
		}

		// English note.
		if updatedAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, updatedAt)
				if err == nil && !parsed.IsZero() {
					item.UpdatedAt = parsed
					break
				}
			}
		}

		// English note.
		if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}

		items = append(items, item)
	}

	return items, nil
}

// English note.
func (m *Manager) GetItemsCount(category string) (int, error) {
	var count int
	var err error

	if category != "" {
		err = m.db.QueryRow("SELECT COUNT(*) FROM knowledge_base_items WHERE category = ?", category).Scan(&count)
	} else {
		err = m.db.QueryRow("SELECT COUNT(*) FROM knowledge_base_items").Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf(": %w", err)
	}

	return count, nil
}

// English note.
func (m *Manager) SearchItemsByKeyword(keyword string, category string) ([]*KnowledgeItemSummary, error) {
	if keyword == "" {
		return nil, fmt.Errorf("")
	}

	// English note.
	var query string
	var args []interface{}

	// English note.
	// English note.
	searchPattern := "%" + keyword + "%"

	query = `
		SELECT id, category, title, file_path, created_at, updated_at 
		FROM knowledge_base_items 
		WHERE (LOWER(title) LIKE LOWER(?) OR LOWER(category) LIKE LOWER(?) OR LOWER(file_path) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?))
	`
	args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)

	// English note.
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	query += " ORDER BY category, title"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	var items []*KnowledgeItemSummary
	for rows.Next() {
		item := &KnowledgeItemSummary{}
		var createdAt, updatedAt string

		if err := rows.Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		if createdAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, createdAt)
				if err == nil && !parsed.IsZero() {
					item.CreatedAt = parsed
					break
				}
			}
		}

		if updatedAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, updatedAt)
				if err == nil && !parsed.IsZero() {
					item.UpdatedAt = parsed
					break
				}
			}
		}

		if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}

		items = append(items, item)
	}

	return items, nil
}

// English note.
func (m *Manager) GetItemsSummary(category string, limit, offset int) ([]*KnowledgeItemSummary, int, error) {
	// English note.
	total, err := m.GetItemsCount(category)
	if err != nil {
		return nil, 0, err
	}

	// English note.
	var rows *sql.Rows
	var query string
	var args []interface{}

	query = "SELECT id, category, title, file_path, created_at, updated_at FROM knowledge_base_items"

	if category != "" {
		query += " WHERE category = ?"
		args = append(args, category)
	}

	query += " ORDER BY category, title"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	rows, err = m.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	var items []*KnowledgeItemSummary
	for rows.Next() {
		item := &KnowledgeItemSummary{}
		var createdAt, updatedAt string

		if err := rows.Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf(": %w", err)
		}

		// English note.
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		if createdAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, createdAt)
				if err == nil && !parsed.IsZero() {
					item.CreatedAt = parsed
					break
				}
			}
		}

		if updatedAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, updatedAt)
				if err == nil && !parsed.IsZero() {
					item.UpdatedAt = parsed
					break
				}
			}
		}

		if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}

		items = append(items, item)
	}

	return items, total, nil
}

// English note.
func (m *Manager) GetItem(id string) (*KnowledgeItem, error) {
	item := &KnowledgeItem{}
	var createdAt, updatedAt string
	err := m.db.QueryRow(
		"SELECT id, category, title, file_path, content, created_at, updated_at FROM knowledge_base_items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &item.Content, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("")
	}
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	timeFormats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	// English note.
	if createdAt != "" {
		for _, format := range timeFormats {
			parsed, err := time.Parse(format, createdAt)
			if err == nil && !parsed.IsZero() {
				item.CreatedAt = parsed
				break
			}
		}
	}

	// English note.
	if updatedAt != "" {
		for _, format := range timeFormats {
			parsed, err := time.Parse(format, updatedAt)
			if err == nil && !parsed.IsZero() {
				item.UpdatedAt = parsed
				break
			}
		}
	}

	// English note.
	if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}

	return item, nil
}

// English note.
func (m *Manager) CreateItem(category, title, content string) (*KnowledgeItem, error) {
	id := uuid.New().String()
	now := time.Now()

	// English note.
	filePath := filepath.Join(m.basePath, category, title+".md")

	// English note.
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	_, err := m.db.Exec(
		"INSERT INTO knowledge_base_items (id, category, title, file_path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, category, title, filePath, content, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return &KnowledgeItem{
		ID:        id,
		Category:  category,
		Title:     title,
		FilePath:  filePath,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// English note.
func (m *Manager) UpdateItem(id, category, title, content string) (*KnowledgeItem, error) {
	// English note.
	item, err := m.GetItem(id)
	if err != nil {
		return nil, err
	}

	// English note.
	newFilePath := filepath.Join(m.basePath, category, title+".md")

	// English note.
	if item.FilePath != newFilePath {
		// English note.
		if err := os.MkdirAll(filepath.Dir(newFilePath), 0755); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		if err := os.Rename(item.FilePath, newFilePath); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		oldDir := filepath.Dir(item.FilePath)
		if isEmpty, _ := isEmptyDir(oldDir); isEmpty {
			// English note.
			if oldDir != m.basePath {
				if err := os.Remove(oldDir); err != nil {
					m.logger.Warn("", zap.String("dir", oldDir), zap.Error(err))
				}
			}
		}
	}

	// English note.
	if err := os.WriteFile(newFilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	_, err = m.db.Exec(
		"UPDATE knowledge_base_items SET category = ?, title = ?, file_path = ?, content = ?, updated_at = ? WHERE id = ?",
		category, title, newFilePath, content, time.Now(), id,
	)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	_, err = m.db.Exec("DELETE FROM knowledge_embeddings WHERE item_id = ?", id)
	if err != nil {
		m.logger.Warn("", zap.Error(err))
	}

	return m.GetItem(id)
}

// English note.
func (m *Manager) DeleteItem(id string) error {
	// English note.
	var filePath string
	err := m.db.QueryRow("SELECT file_path FROM knowledge_base_items WHERE id = ?", id).Scan(&filePath)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		m.logger.Warn("", zap.String("path", filePath), zap.Error(err))
	}

	// English note.
	_, err = m.db.Exec("DELETE FROM knowledge_base_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	// English note.
	dir := filepath.Dir(filePath)
	if isEmpty, _ := isEmptyDir(dir); isEmpty {
		// English note.
		if dir != m.basePath {
			if err := os.Remove(dir); err != nil {
				m.logger.Warn("", zap.String("dir", dir), zap.Error(err))
			}
		}
	}

	return nil
}

// English note.
func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		// English note.
		if !strings.HasPrefix(entry.Name(), ".") {
			return false, nil
		}
	}
	return true, nil
}

// English note.
func (m *Manager) LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error {
	id := uuid.New().String()
	itemsJSON, _ := json.Marshal(retrievedItems)

	_, err := m.db.Exec(
		"INSERT INTO knowledge_retrieval_logs (id, conversation_id, message_id, query, risk_type, retrieved_items, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, conversationID, messageID, query, riskType, string(itemsJSON), time.Now(),
	)
	return err
}

// English note.
func (m *Manager) GetIndexStatus() (map[string]interface{}, error) {
	// English note.
	var totalItems int
	err := m.db.QueryRow("SELECT COUNT(*) FROM knowledge_base_items").Scan(&totalItems)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	var indexedItems int
	err = m.db.QueryRow(`
		SELECT COUNT(DISTINCT item_id) 
		FROM knowledge_embeddings
	`).Scan(&indexedItems)
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	// English note.
	var progressPercent float64
	if totalItems > 0 {
		progressPercent = float64(indexedItems) / float64(totalItems) * 100
	} else {
		progressPercent = 100.0
	}

	// English note.
	isComplete := indexedItems >= totalItems && totalItems > 0

	return map[string]interface{}{
		"total_items":      totalItems,
		"indexed_items":    indexedItems,
		"progress_percent": progressPercent,
		"is_complete":      isComplete,
	}, nil
}

// English note.
func (m *Manager) GetRetrievalLogs(conversationID, messageID string, limit int) ([]*RetrievalLog, error) {
	var rows *sql.Rows
	var err error

	if messageID != "" {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs WHERE message_id = ? ORDER BY created_at DESC LIMIT ?",
			messageID, limit,
		)
	} else if conversationID != "" {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?",
			conversationID, limit,
		)
	} else {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs ORDER BY created_at DESC LIMIT ?",
			limit,
		)
	}

	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}
	defer rows.Close()

	var logs []*RetrievalLog
	for rows.Next() {
		log := &RetrievalLog{}
		var createdAt string
		var itemsJSON sql.NullString
		if err := rows.Scan(&log.ID, &log.ConversationID, &log.MessageID, &log.Query, &log.RiskType, &itemsJSON, &createdAt); err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		// English note.
		var err error
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		for _, format := range timeFormats {
			log.CreatedAt, err = time.Parse(format, createdAt)
			if err == nil && !log.CreatedAt.IsZero() {
				break
			}
		}

		// English note.
		if log.CreatedAt.IsZero() {
			m.logger.Warn("",
				zap.String("timeStr", createdAt),
				zap.Error(err),
			)
			// English note.
			log.CreatedAt = time.Now()
		}

		// English note.
		if itemsJSON.Valid {
			json.Unmarshal([]byte(itemsJSON.String), &log.RetrievedItems)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// English note.
func (m *Manager) DeleteRetrievalLog(id string) error {
	result, err := m.db.Exec("DELETE FROM knowledge_retrieval_logs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(": %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("")
	}

	return nil
}
