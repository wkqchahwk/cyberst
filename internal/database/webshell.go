package database

import (
	"database/sql"
	"time"

	"go.uber.org/zap"
)

// English note.
type WebShellConnection struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Password  string    `json:"password"`
	Type      string    `json:"type"`
	Method    string    `json:"method"`
	CmdParam  string    `json:"cmdParam"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
}

// English note.
func (db *DB) GetWebshellConnectionState(connectionID string) (string, error) {
	var stateJSON string
	err := db.QueryRow(`SELECT state_json FROM webshell_connection_states WHERE connection_id = ?`, connectionID).Scan(&stateJSON)
	if err == sql.ErrNoRows {
		return "{}", nil
	}
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("connectionID", connectionID))
		return "", err
	}
	if stateJSON == "" {
		stateJSON = "{}"
	}
	return stateJSON, nil
}

// English note.
func (db *DB) UpsertWebshellConnectionState(connectionID, stateJSON string) error {
	if stateJSON == "" {
		stateJSON = "{}"
	}
	query := `
		INSERT INTO webshell_connection_states (connection_id, state_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(connection_id) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = excluded.updated_at
	`
	if _, err := db.Exec(query, connectionID, stateJSON, time.Now()); err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("connectionID", connectionID))
		return err
	}
	return nil
}

// English note.
func (db *DB) ListWebshellConnections() ([]WebShellConnection, error) {
	query := `
		SELECT id, url, password, type, method, cmd_param, remark, created_at
		FROM webshell_connections
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var list []WebShellConnection
	for rows.Next() {
		var c WebShellConnection
		err := rows.Scan(&c.ID, &c.URL, &c.Password, &c.Type, &c.Method, &c.CmdParam, &c.Remark, &c.CreatedAt)
		if err != nil {
			db.logger.Warn(" WebShell ", zap.Error(err))
			continue
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// English note.
func (db *DB) GetWebshellConnection(id string) (*WebShellConnection, error) {
	query := `
		SELECT id, url, password, type, method, cmd_param, remark, created_at
		FROM webshell_connections WHERE id = ?
	`
	var c WebShellConnection
	err := db.QueryRow(query, id).Scan(&c.ID, &c.URL, &c.Password, &c.Type, &c.Method, &c.CmdParam, &c.Remark, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("id", id))
		return nil, err
	}
	return &c, nil
}

// English note.
func (db *DB) CreateWebshellConnection(c *WebShellConnection) error {
	query := `
		INSERT INTO webshell_connections (id, url, password, type, method, cmd_param, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, c.ID, c.URL, c.Password, c.Type, c.Method, c.CmdParam, c.Remark, c.CreatedAt)
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("id", c.ID))
		return err
	}
	return nil
}

// English note.
func (db *DB) UpdateWebshellConnection(c *WebShellConnection) error {
	query := `
		UPDATE webshell_connections
		SET url = ?, password = ?, type = ?, method = ?, cmd_param = ?, remark = ?
		WHERE id = ?
	`
	result, err := db.Exec(query, c.URL, c.Password, c.Type, c.Method, c.CmdParam, c.Remark, c.ID)
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("id", c.ID))
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// English note.
func (db *DB) DeleteWebshellConnection(id string) error {
	result, err := db.Exec(`DELETE FROM webshell_connections WHERE id = ?`, id)
	if err != nil {
		db.logger.Error(" WebShell ", zap.Error(err), zap.String("id", id))
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
