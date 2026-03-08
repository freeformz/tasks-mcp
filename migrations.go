package main

import "fmt"

// migrations is an ordered list of schema migrations. Each migration runs
// exactly once, tracked by SQLite's PRAGMA user_version. Migrations must be
// append-only — never reorder or remove entries.
var migrations = []func(*DB) error{
	migrateV1InitialSchema,
	migrateV2AddAssignee,
	migrateV3AgentPresence,
	migrateV4TaskNotes,
}

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if err := migrations[i](d); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := d.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return fmt.Errorf("set schema version %d: %w", i+1, err)
		}
	}

	return nil
}

// migrateV1InitialSchema creates the base tables and indexes.
// Idempotent: uses IF NOT EXISTS so it works on both fresh and
// pre-versioned existing databases.
func migrateV1InitialSchema(d *DB) error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			workspace TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo',
			priority TEXT NOT NULL DEFAULT 'medium',
			parent_id TEXT REFERENCES tasks(id) ON DELETE CASCADE,
			progress_notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS task_tags (
			task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			tag TEXT NOT NULL,
			PRIMARY KEY (task_id, tag)
		);

		CREATE TABLE IF NOT EXISTS task_dependencies (
			task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			depends_on_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, depends_on_id)
		);

		CREATE INDEX IF NOT EXISTS idx_tasks_workspace ON tasks(workspace);
		CREATE INDEX IF NOT EXISTS idx_tasks_workspace_status ON tasks(workspace, status);
		CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
	`)
	return err
}

// migrateV2AddAssignee adds the assignee column and its index.
// Idempotent: checks for column existence before altering.
func migrateV2AddAssignee(d *DB) error {
	var exists bool
	if err := d.db.QueryRow(
		"SELECT COUNT(*) > 0 FROM pragma_table_info('tasks') WHERE name = 'assignee'",
	).Scan(&exists); err != nil {
		return fmt.Errorf("check assignee column: %w", err)
	}

	if !exists {
		if _, err := d.db.Exec(`ALTER TABLE tasks ADD COLUMN assignee TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add assignee column: %w", err)
		}
	}

	if _, err := d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_workspace_assignee ON tasks(workspace, assignee)`); err != nil {
		return fmt.Errorf("create assignee index: %w", err)
	}

	return nil
}

// migrateV4TaskNotes creates the task_notes table for structured note storage
// and drops the progress_notes column from the tasks table.
func migrateV4TaskNotes(d *DB) error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS task_notes (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_task_notes_task_id_created ON task_notes(task_id, created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("create task_notes table: %w", err)
	}

	// Drop the old progress_notes column.
	var hasColumn bool
	if err := d.db.QueryRow(
		"SELECT COUNT(*) > 0 FROM pragma_table_info('tasks') WHERE name = 'progress_notes'",
	).Scan(&hasColumn); err != nil {
		return fmt.Errorf("check progress_notes column: %w", err)
	}
	if hasColumn {
		if _, err := d.db.Exec(`ALTER TABLE tasks DROP COLUMN progress_notes`); err != nil {
			return fmt.Errorf("drop progress_notes column: %w", err)
		}
	}

	return nil
}

// migrateV3AgentPresence creates the agent_presence table for tracking
// which agents are currently active in a workspace.
func migrateV3AgentPresence(d *DB) error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_presence (
			id TEXT PRIMARY KEY,
			workspace TEXT NOT NULL,
			agent_name TEXT NOT NULL,
			session_id TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			last_heartbeat DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_agent_presence_workspace ON agent_presence(workspace);
	`)
	return err
}
