package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// English note.
type ConversationGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// English note.
func (db *DB) GroupExistsByName(name string, excludeID string) (bool, error) {
	var count int
	var err error

	if excludeID != "" {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM conversation_groups WHERE name = ? AND id != ?",
			name, excludeID,
		).Scan(&count)
	} else {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM conversation_groups WHERE name = ?",
			name,
		).Scan(&count)
	}

	if err != nil {
		return false, fmt.Errorf("检查分组名称失败: %w", err)
	}

	return count > 0, nil
}

// English note.
func (db *DB) CreateGroup(name, icon string) (*ConversationGroup, error) {
	// English note.
	exists, err := db.GroupExistsByName(name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("分组名称已存在")
	}

	id := uuid.New().String()
	now := time.Now()

	if icon == "" {
		icon = "📁"
	}

	_, err = db.Exec(
		"INSERT INTO conversation_groups (id, name, icon, pinned, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, icon, 0, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建分组失败: %w", err)
	}

	return &ConversationGroup{
		ID:        id,
		Name:      name,
		Icon:      icon,
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// English note.
func (db *DB) ListGroups() ([]*ConversationGroup, error) {
	rows, err := db.Query(
		"SELECT id, name, icon, COALESCE(pinned, 0), created_at, updated_at FROM conversation_groups ORDER BY COALESCE(pinned, 0) DESC, created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组列表失败: %w", err)
	}
	defer rows.Close()

	var groups []*ConversationGroup
	for rows.Next() {
		var group ConversationGroup
		var createdAt, updatedAt string
		var pinned int

		if err := rows.Scan(&group.ID, &group.Name, &group.Icon, &pinned, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描分组失败: %w", err)
		}

		group.Pinned = pinned != 0

		// English note.
		var err1, err2 error
		group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			group.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			group.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		}

		groups = append(groups, &group)
	}

	return groups, nil
}

// English note.
func (db *DB) GetGroup(id string) (*ConversationGroup, error) {
	var group ConversationGroup
	var createdAt, updatedAt string
	var pinned int

	err := db.QueryRow(
		"SELECT id, name, icon, COALESCE(pinned, 0), created_at, updated_at FROM conversation_groups WHERE id = ?",
		id,
	).Scan(&group.ID, &group.Name, &group.Icon, &pinned, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("分组不存在")
		}
		return nil, fmt.Errorf("查询分组失败: %w", err)
	}

	// English note.
	var err1, err2 error
	group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
	if err1 != nil {
		group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	if err1 != nil {
		group.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}

	group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
	if err2 != nil {
		group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	if err2 != nil {
		group.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	group.Pinned = pinned != 0

	return &group, nil
}

// English note.
func (db *DB) UpdateGroup(id, name, icon string) error {
	// English note.
	exists, err := db.GroupExistsByName(name, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组名称已存在")
	}

	_, err = db.Exec(
		"UPDATE conversation_groups SET name = ?, icon = ?, updated_at = ? WHERE id = ?",
		name, icon, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新分组失败: %w", err)
	}
	return nil
}

// English note.
func (db *DB) DeleteGroup(id string) error {
	_, err := db.Exec("DELETE FROM conversation_groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除分组失败: %w", err)
	}
	return nil
}

// English note.
// English note.
func (db *DB) AddConversationToGroup(conversationID, groupID string) error {
	// English note.
	_, err := db.Exec(
		"DELETE FROM conversation_group_mappings WHERE conversation_id = ?",
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("删除对话旧分组关联失败: %w", err)
	}

	// English note.
	id := uuid.New().String()
	_, err = db.Exec(
		"INSERT INTO conversation_group_mappings (id, conversation_id, group_id, created_at) VALUES (?, ?, ?, ?)",
		id, conversationID, groupID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("添加对话到分组失败: %w", err)
	}
	return nil
}

// English note.
func (db *DB) RemoveConversationFromGroup(conversationID, groupID string) error {
	_, err := db.Exec(
		"DELETE FROM conversation_group_mappings WHERE conversation_id = ? AND group_id = ?",
		conversationID, groupID,
	)
	if err != nil {
		return fmt.Errorf("从分组中移除对话失败: %w", err)
	}
	return nil
}

// English note.
func (db *DB) GetConversationsByGroup(groupID string) ([]*Conversation, error) {
	rows, err := db.Query(
		`SELECT c.id, c.title, COALESCE(c.pinned, 0), c.created_at, c.updated_at, COALESCE(cgm.pinned, 0) as group_pinned
		 FROM conversations c
		 INNER JOIN conversation_group_mappings cgm ON c.id = cgm.conversation_id
		 WHERE cgm.group_id = ?
		 ORDER BY COALESCE(cgm.pinned, 0) DESC, c.updated_at DESC`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组对话失败: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string
		var pinned int
		var groupPinned int

		if err := rows.Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt, &groupPinned); err != nil {
			return nil, fmt.Errorf("扫描对话失败: %w", err)
		}

		// English note.
		var err1, err2 error
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			conv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			conv.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		}

		conv.Pinned = pinned != 0

		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// English note.
func (db *DB) SearchConversationsByGroup(groupID string, searchQuery string) ([]*Conversation, error) {
	// English note.
	// English note.
	query := `SELECT DISTINCT c.id, c.title, COALESCE(c.pinned, 0), c.created_at, c.updated_at, COALESCE(cgm.pinned, 0) as group_pinned
		 FROM conversations c
		 INNER JOIN conversation_group_mappings cgm ON c.id = cgm.conversation_id
		 WHERE cgm.group_id = ?`

	args := []interface{}{groupID}

	// English note.
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		// English note.
		// English note.
		query += ` AND (
			LOWER(c.title) LIKE LOWER(?)
			OR EXISTS (
				SELECT 1 FROM messages m 
				WHERE m.conversation_id = c.id 
				AND LOWER(m.content) LIKE LOWER(?)
			)
		)`
		args = append(args, searchPattern, searchPattern)
	}

	query += " ORDER BY COALESCE(cgm.pinned, 0) DESC, c.updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索分组对话失败: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string
		var pinned int
		var groupPinned int

		if err := rows.Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt, &groupPinned); err != nil {
			return nil, fmt.Errorf("扫描对话失败: %w", err)
		}

		// English note.
		var err1, err2 error
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			conv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			conv.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		}

		conv.Pinned = pinned != 0

		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// English note.
func (db *DB) GetGroupByConversation(conversationID string) (string, error) {
	var groupID string
	err := db.QueryRow(
		"SELECT group_id FROM conversation_group_mappings WHERE conversation_id = ? LIMIT 1",
		conversationID,
	).Scan(&groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // 没有分组
		}
		return "", fmt.Errorf("查询对话分组失败: %w", err)
	}
	return groupID, nil
}

// English note.
func (db *DB) UpdateConversationPinned(id string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	// English note.
	_, err := db.Exec(
		"UPDATE conversations SET pinned = ? WHERE id = ?",
		pinnedValue, id,
	)
	if err != nil {
		return fmt.Errorf("更新对话置顶状态失败: %w", err)
	}
	return nil
}

// English note.
func (db *DB) UpdateGroupPinned(id string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	_, err := db.Exec(
		"UPDATE conversation_groups SET pinned = ?, updated_at = ? WHERE id = ?",
		pinnedValue, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新分组置顶状态失败: %w", err)
	}
	return nil
}

// English note.
type GroupMapping struct {
	ConversationID string `json:"conversationId"`
	GroupID        string `json:"groupId"`
}

// English note.
func (db *DB) GetAllGroupMappings() ([]GroupMapping, error) {
	rows, err := db.Query("SELECT conversation_id, group_id FROM conversation_group_mappings")
	if err != nil {
		return nil, fmt.Errorf("查询分组映射失败: %w", err)
	}
	defer rows.Close()

	var mappings []GroupMapping
	for rows.Next() {
		var m GroupMapping
		if err := rows.Scan(&m.ConversationID, &m.GroupID); err != nil {
			return nil, fmt.Errorf("扫描分组映射失败: %w", err)
		}
		mappings = append(mappings, m)
	}

	if mappings == nil {
		mappings = []GroupMapping{}
	}
	return mappings, nil
}

// English note.
func (db *DB) UpdateConversationPinnedInGroup(conversationID, groupID string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	_, err := db.Exec(
		"UPDATE conversation_group_mappings SET pinned = ? WHERE conversation_id = ? AND group_id = ?",
		pinnedValue, conversationID, groupID,
	)
	if err != nil {
		return fmt.Errorf("更新分组对话置顶状态失败: %w", err)
	}
	return nil
}
