package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// English note.
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// English note.
func NewDB(dbPath string, logger *zap.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	database := &DB{
		DB:     db,
		logger: logger,
	}

	// English note.
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return database, nil
}

// English note.
func (db *DB) initTables() error {
	// English note.
	createConversationsTable := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_react_input TEXT,
		last_react_output TEXT
	);`

	// English note.
	createMessagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		mcp_execution_ids TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// English note.
	createProcessDetailsTable := `
	CREATE TABLE IF NOT EXISTS process_details (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL,
		conversation_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		message TEXT,
		data TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// English note.
	createToolExecutionsTable := `
	CREATE TABLE IF NOT EXISTS tool_executions (
		id TEXT PRIMARY KEY,
		tool_name TEXT NOT NULL,
		arguments TEXT NOT NULL,
		status TEXT NOT NULL,
		result TEXT,
		error TEXT,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		duration_ms INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// English note.
	createToolStatsTable := `
	CREATE TABLE IF NOT EXISTS tool_stats (
		tool_name TEXT PRIMARY KEY,
		total_calls INTEGER NOT NULL DEFAULT 0,
		success_calls INTEGER NOT NULL DEFAULT 0,
		failed_calls INTEGER NOT NULL DEFAULT 0,
		last_call_time DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// English note.
	createSkillStatsTable := `
	CREATE TABLE IF NOT EXISTS skill_stats (
		skill_name TEXT PRIMARY KEY,
		total_calls INTEGER NOT NULL DEFAULT 0,
		success_calls INTEGER NOT NULL DEFAULT 0,
		failed_calls INTEGER NOT NULL DEFAULT 0,
		last_call_time DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// English note.
	createAttackChainNodesTable := `
	CREATE TABLE IF NOT EXISTS attack_chain_nodes (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		node_type TEXT NOT NULL,
		node_name TEXT NOT NULL,
		tool_execution_id TEXT,
		metadata TEXT,
		risk_score INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id) ON DELETE SET NULL
	);`

	// English note.
	createAttackChainEdgesTable := `
	CREATE TABLE IF NOT EXISTS attack_chain_edges (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		source_node_id TEXT NOT NULL,
		target_node_id TEXT NOT NULL,
		edge_type TEXT NOT NULL,
		weight INTEGER DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (source_node_id) REFERENCES attack_chain_nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (target_node_id) REFERENCES attack_chain_nodes(id) ON DELETE CASCADE
	);`

	// English note.
	createKnowledgeRetrievalLogsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
		id TEXT PRIMARY KEY,
		conversation_id TEXT,
		message_id TEXT,
		query TEXT NOT NULL,
		risk_type TEXT,
		retrieved_items TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL
	);`

	// English note.
	createConversationGroupsTable := `
	CREATE TABLE IF NOT EXISTS conversation_groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	// English note.
	createConversationGroupMappingsTable := `
	CREATE TABLE IF NOT EXISTS conversation_group_mappings (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		group_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (group_id) REFERENCES conversation_groups(id) ON DELETE CASCADE,
		UNIQUE(conversation_id, group_id)
	);`

	// English note.
	createVulnerabilitiesTable := `
	CREATE TABLE IF NOT EXISTS vulnerabilities (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		severity TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		vulnerability_type TEXT,
		target TEXT,
		proof TEXT,
		impact TEXT,
		recommendation TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// English note.
	createBatchTaskQueuesTable := `
	CREATE TABLE IF NOT EXISTS batch_task_queues (
		id TEXT PRIMARY KEY,
		title TEXT,
		role TEXT,
		agent_mode TEXT NOT NULL DEFAULT 'single',
		schedule_mode TEXT NOT NULL DEFAULT 'manual',
		cron_expr TEXT,
		next_run_at DATETIME,
		schedule_enabled INTEGER NOT NULL DEFAULT 1,
		last_schedule_trigger_at DATETIME,
		last_schedule_error TEXT,
		last_run_error TEXT,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		current_index INTEGER NOT NULL DEFAULT 0
	);`

	// English note.
	createBatchTasksTable := `
	CREATE TABLE IF NOT EXISTS batch_tasks (
		id TEXT PRIMARY KEY,
		queue_id TEXT NOT NULL,
		message TEXT NOT NULL,
		conversation_id TEXT,
		status TEXT NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		error TEXT,
		result TEXT,
		FOREIGN KEY (queue_id) REFERENCES batch_task_queues(id) ON DELETE CASCADE
	);`

	// English note.
	createWebshellConnectionsTable := `
	CREATE TABLE IF NOT EXISTS webshell_connections (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		password TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT 'php',
		method TEXT NOT NULL DEFAULT 'post',
		cmd_param TEXT NOT NULL DEFAULT '',
		remark TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// English note.
	createWebshellConnectionStatesTable := `
	CREATE TABLE IF NOT EXISTS webshell_connection_states (
		connection_id TEXT PRIMARY KEY,
		state_json TEXT NOT NULL DEFAULT '{}',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (connection_id) REFERENCES webshell_connections(id) ON DELETE CASCADE
	);`

	// English note.
	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
	CREATE INDEX IF NOT EXISTS idx_process_details_message_id ON process_details(message_id);
	CREATE INDEX IF NOT EXISTS idx_process_details_conversation_id ON process_details(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_tool_name ON tool_executions(tool_name);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_start_time ON tool_executions(start_time);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_status ON tool_executions(status);
	CREATE INDEX IF NOT EXISTS idx_chain_nodes_conversation ON attack_chain_nodes(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_conversation ON attack_chain_edges(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_source ON attack_chain_edges(source_node_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_target ON attack_chain_edges(target_node_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_conversation ON knowledge_retrieval_logs(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_message ON knowledge_retrieval_logs(message_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_created_at ON knowledge_retrieval_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_conversation_group_mappings_conversation ON conversation_group_mappings(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversation_group_mappings_group ON conversation_group_mappings(group_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_pinned ON conversations(pinned);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_conversation_id ON vulnerabilities(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_severity ON vulnerabilities(severity);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_status ON vulnerabilities(status);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_created_at ON vulnerabilities(created_at);
	CREATE INDEX IF NOT EXISTS idx_batch_tasks_queue_id ON batch_tasks(queue_id);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_created_at ON batch_task_queues(created_at);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_title ON batch_task_queues(title);
	CREATE INDEX IF NOT EXISTS idx_webshell_connections_created_at ON webshell_connections(created_at);
	CREATE INDEX IF NOT EXISTS idx_webshell_connection_states_updated_at ON webshell_connection_states(updated_at);
	`

	if _, err := db.Exec(createConversationsTable); err != nil {
		return fmt.Errorf("conversations: %w", err)
	}

	if _, err := db.Exec(createMessagesTable); err != nil {
		return fmt.Errorf("messages: %w", err)
	}

	if _, err := db.Exec(createProcessDetailsTable); err != nil {
		return fmt.Errorf("process_details: %w", err)
	}

	if _, err := db.Exec(createToolExecutionsTable); err != nil {
		return fmt.Errorf("tool_executions: %w", err)
	}

	if _, err := db.Exec(createToolStatsTable); err != nil {
		return fmt.Errorf("tool_stats: %w", err)
	}

	if _, err := db.Exec(createSkillStatsTable); err != nil {
		return fmt.Errorf("skill_stats: %w", err)
	}

	if _, err := db.Exec(createAttackChainNodesTable); err != nil {
		return fmt.Errorf("attack_chain_nodes: %w", err)
	}

	if _, err := db.Exec(createAttackChainEdgesTable); err != nil {
		return fmt.Errorf("attack_chain_edges: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("knowledge_retrieval_logs: %w", err)
	}

	if _, err := db.Exec(createConversationGroupsTable); err != nil {
		return fmt.Errorf("conversation_groups: %w", err)
	}

	if _, err := db.Exec(createConversationGroupMappingsTable); err != nil {
		return fmt.Errorf("conversation_group_mappings: %w", err)
	}

	if _, err := db.Exec(createVulnerabilitiesTable); err != nil {
		return fmt.Errorf("vulnerabilities: %w", err)
	}

	if _, err := db.Exec(createBatchTaskQueuesTable); err != nil {
		return fmt.Errorf("batch_task_queues: %w", err)
	}

	if _, err := db.Exec(createBatchTasksTable); err != nil {
		return fmt.Errorf("batch_tasks: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionsTable); err != nil {
		return fmt.Errorf("webshell_connections: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionStatesTable); err != nil {
		return fmt.Errorf("webshell_connection_states: %w", err)
	}

	// English note.
	if err := db.migrateConversationsTable(); err != nil {
		db.logger.Warn("conversations", zap.Error(err))
		// English note.
	}

	if err := db.migrateConversationGroupsTable(); err != nil {
		db.logger.Warn("conversation_groups", zap.Error(err))
		// English note.
	}

	if err := db.migrateConversationGroupMappingsTable(); err != nil {
		db.logger.Warn("conversation_group_mappings", zap.Error(err))
		// English note.
	}

	if err := db.migrateBatchTaskQueuesTable(); err != nil {
		db.logger.Warn("batch_task_queues", zap.Error(err))
		// English note.
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf(": %w", err)
	}

	db.logger.Info("")
	return nil
}

// English note.
func (db *DB) migrateConversationsTable() error {
	// English note.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_input'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("last_react_input", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); err != nil {
			db.logger.Warn("last_react_input", zap.Error(err))
		}
	}

	// English note.
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_output'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("last_react_output", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); err != nil {
			db.logger.Warn("last_react_output", zap.Error(err))
		}
	}

	// English note.
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("pinned", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("pinned", zap.Error(err))
		}
	}

	// English note.
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='webshell_connection_id'").Scan(&count)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("webshell_connection_id", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); err != nil {
			db.logger.Warn("webshell_connection_id", zap.Error(err))
		}
	}

	return nil
}

// English note.
func (db *DB) migrateConversationGroupsTable() error {
	// English note.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_groups') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("pinned", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("pinned", zap.Error(err))
		}
	}

	return nil
}

// English note.
func (db *DB) migrateConversationGroupMappingsTable() error {
	// English note.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_group_mappings') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("pinned", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("pinned", zap.Error(err))
		}
	}

	return nil
}

// English note.
func (db *DB) migrateBatchTaskQueuesTable() error {
	// English note.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='title'").Scan(&count)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("title", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); err != nil {
			db.logger.Warn("title", zap.Error(err))
		}
	}

	// English note.
	var roleCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='role'").Scan(&roleCount)
	if err != nil {
		// English note.
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); addErr != nil {
			// English note.
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("role", zap.Error(addErr))
			}
		}
	} else if roleCount == 0 {
		// English note.
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); err != nil {
			db.logger.Warn("role", zap.Error(err))
		}
	}

	// English note.
	var agentModeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='agent_mode'").Scan(&agentModeCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'single'"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("agent_mode", zap.Error(addErr))
			}
		}
	} else if agentModeCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'single'"); err != nil {
			db.logger.Warn("agent_mode", zap.Error(err))
		}
	}

	// English note.
	var scheduleModeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='schedule_mode'").Scan(&scheduleModeCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_mode TEXT NOT NULL DEFAULT 'manual'"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("schedule_mode", zap.Error(addErr))
			}
		}
	} else if scheduleModeCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_mode TEXT NOT NULL DEFAULT 'manual'"); err != nil {
			db.logger.Warn("schedule_mode", zap.Error(err))
		}
	}

	// English note.
	var cronExprCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='cron_expr'").Scan(&cronExprCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN cron_expr TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("cron_expr", zap.Error(addErr))
			}
		}
	} else if cronExprCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN cron_expr TEXT"); err != nil {
			db.logger.Warn("cron_expr", zap.Error(err))
		}
	}

	// English note.
	var nextRunAtCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='next_run_at'").Scan(&nextRunAtCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN next_run_at DATETIME"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("next_run_at", zap.Error(addErr))
			}
		}
	} else if nextRunAtCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN next_run_at DATETIME"); err != nil {
			db.logger.Warn("next_run_at", zap.Error(err))
		}
	}

	// English note.
	var scheduleEnCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='schedule_enabled'").Scan(&scheduleEnCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_enabled INTEGER NOT NULL DEFAULT 1"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("schedule_enabled", zap.Error(addErr))
			}
		}
	} else if scheduleEnCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_enabled INTEGER NOT NULL DEFAULT 1"); err != nil {
			db.logger.Warn("schedule_enabled", zap.Error(err))
		}
	}

	var lastTrigCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_schedule_trigger_at'").Scan(&lastTrigCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_trigger_at DATETIME"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("last_schedule_trigger_at", zap.Error(addErr))
			}
		}
	} else if lastTrigCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_trigger_at DATETIME"); err != nil {
			db.logger.Warn("last_schedule_trigger_at", zap.Error(err))
		}
	}

	var lastSchedErrCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_schedule_error'").Scan(&lastSchedErrCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_error TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("last_schedule_error", zap.Error(addErr))
			}
		}
	} else if lastSchedErrCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_error TEXT"); err != nil {
			db.logger.Warn("last_schedule_error", zap.Error(err))
		}
	}

	var lastRunErrCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_run_error'").Scan(&lastRunErrCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_run_error TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("last_run_error", zap.Error(addErr))
			}
		}
	} else if lastRunErrCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_run_error TEXT"); err != nil {
			db.logger.Warn("last_run_error", zap.Error(err))
		}
	}

	return nil
}

// English note.
func NewKnowledgeDB(dbPath string, logger *zap.Logger) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	database := &DB{
		DB:     sqlDB,
		logger: logger,
	}

	// English note.
	if err := database.initKnowledgeTables(); err != nil {
		return nil, fmt.Errorf(": %w", err)
	}

	return database, nil
}

// English note.
func (db *DB) initKnowledgeTables() error {
	// English note.
	createKnowledgeBaseItemsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_base_items (
		id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		title TEXT NOT NULL,
		file_path TEXT NOT NULL,
		content TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	// English note.
	createKnowledgeEmbeddingsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_embeddings (
		id TEXT PRIMARY KEY,
		item_id TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		embedding TEXT NOT NULL,
		sub_indexes TEXT NOT NULL DEFAULT '',
		embedding_model TEXT NOT NULL DEFAULT '',
		embedding_dim INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (item_id) REFERENCES knowledge_base_items(id) ON DELETE CASCADE
	);`

	// English note.
	createKnowledgeRetrievalLogsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
		id TEXT PRIMARY KEY,
		conversation_id TEXT,
		message_id TEXT,
		query TEXT NOT NULL,
		risk_type TEXT,
		retrieved_items TEXT,
		created_at DATETIME NOT NULL
	);`

	// English note.
	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_knowledge_items_category ON knowledge_base_items(category);
	CREATE INDEX IF NOT EXISTS idx_knowledge_embeddings_item_id ON knowledge_embeddings(item_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_conversation ON knowledge_retrieval_logs(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_message ON knowledge_retrieval_logs(message_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_created_at ON knowledge_retrieval_logs(created_at);
	`

	if _, err := db.Exec(createKnowledgeBaseItemsTable); err != nil {
		return fmt.Errorf("knowledge_base_items: %w", err)
	}

	if _, err := db.Exec(createKnowledgeEmbeddingsTable); err != nil {
		return fmt.Errorf("knowledge_embeddings: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("knowledge_retrieval_logs: %w", err)
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf(": %w", err)
	}

	if err := db.migrateKnowledgeEmbeddingsColumns(); err != nil {
		return fmt.Errorf(" knowledge_embeddings : %w", err)
	}

	db.logger.Info("")
	return nil
}

// English note.
func (db *DB) migrateKnowledgeEmbeddingsColumns() error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_embeddings'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	migrations := []struct {
		col  string
		stmt string
	}{
		{"sub_indexes", `ALTER TABLE knowledge_embeddings ADD COLUMN sub_indexes TEXT NOT NULL DEFAULT ''`},
		{"embedding_model", `ALTER TABLE knowledge_embeddings ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''`},
		{"embedding_dim", `ALTER TABLE knowledge_embeddings ADD COLUMN embedding_dim INTEGER NOT NULL DEFAULT 0`},
	}
	for _, m := range migrations {
		var colCount int
		q := `SELECT COUNT(*) FROM pragma_table_info('knowledge_embeddings') WHERE name = ?`
		if err := db.QueryRow(q, m.col).Scan(&colCount); err != nil {
			return err
		}
		if colCount > 0 {
			continue
		}
		if _, err := db.Exec(m.stmt); err != nil {
			return err
		}
	}
	return nil
}

// English note.
func (db *DB) Close() error {
	return db.DB.Close()
}
