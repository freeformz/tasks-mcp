package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func OpenDB(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	d := &DB{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
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

func (d *DB) CreateTask(workspace, title, description string, status TaskStatus, priority TaskPriority, parentID string, tags []string, dependsOn []string) (*Task, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO tasks (id, workspace, title, description, status, priority, parent_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		id, workspace, title, description, string(status), string(priority), parentID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO task_tags (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return nil, fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	for _, depID := range dependsOn {
		depID = strings.TrimSpace(depID)
		if depID == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`, id, depID); err != nil {
			return nil, fmt.Errorf("insert dependency %q: %w", depID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return d.GetTask(workspace, id)
}

func (d *DB) GetTask(workspace, id string) (*Task, error) {
	task := &Task{}
	var parentID sql.NullString
	err := d.db.QueryRow(
		`SELECT id, workspace, title, description, status, priority, parent_id, progress_notes, created_at, updated_at
		 FROM tasks WHERE id = ? AND workspace = ?`, id, workspace,
	).Scan(&task.ID, &task.Workspace, &task.Title, &task.Description, &task.Status, &task.Priority,
		&parentID, &task.ProgressNotes, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	task.ParentID = parentID.String

	tags, err := d.getTaskTags(id)
	if err != nil {
		return nil, err
	}
	task.Tags = tags

	deps, err := d.getTaskDependencies(id)
	if err != nil {
		return nil, err
	}
	task.DependsOn = deps

	subtasks, err := d.getSubtasks(workspace, id)
	if err != nil {
		return nil, err
	}
	task.Subtasks = subtasks

	return task, nil
}

func (d *DB) ListTasks(workspace string, statusFilter string, tagFilter string, parentID string, includeDone bool) ([]Task, error) {
	query := `SELECT DISTINCT t.id, t.workspace, t.title, t.description, t.status, t.priority, t.parent_id, t.progress_notes, t.created_at, t.updated_at
		FROM tasks t`
	var args []any
	var conditions []string

	conditions = append(conditions, "t.workspace = ?")
	args = append(args, workspace)

	if tagFilter != "" {
		query += ` JOIN task_tags tt ON t.id = tt.task_id`
		conditions = append(conditions, "tt.tag = ?")
		args = append(args, tagFilter)
	}

	if statusFilter != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, statusFilter)
	} else if !includeDone {
		conditions = append(conditions, "t.status != 'done'")
	}

	if parentID != "" {
		conditions = append(conditions, "t.parent_id = ?")
		args = append(args, parentID)
	} else {
		conditions = append(conditions, "t.parent_id IS NULL")
	}

	query += " WHERE " + strings.Join(conditions, " AND ")
	query += " ORDER BY CASE t.priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END, t.created_at"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var parentID sql.NullString
		if err := rows.Scan(&t.ID, &t.Workspace, &t.Title, &t.Description, &t.Status, &t.Priority,
			&parentID, &t.ProgressNotes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.ParentID = parentID.String

		tags, err := d.getTaskTags(t.ID)
		if err != nil {
			return nil, err
		}
		t.Tags = tags

		deps, err := d.getTaskDependencies(t.ID)
		if err != nil {
			return nil, err
		}
		t.DependsOn = deps

		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (d *DB) UpdateTask(workspace, id string, updates map[string]string, addTags, removeTags, addDeps, removeDeps []string) (*Task, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Verify task exists and belongs to workspace.
	var exists bool
	if err := tx.QueryRow(`SELECT 1 FROM tasks WHERE id = ? AND workspace = ?`, id, workspace).Scan(&exists); err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	if len(updates) > 0 {
		var setClauses []string
		var args []any
		for col, val := range updates {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
		setClauses = append(setClauses, "updated_at = ?")
		args = append(args, time.Now().UTC())
		args = append(args, id)

		query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		if _, err := tx.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update task: %w", err)
		}
	}

	for _, tag := range addTags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return nil, fmt.Errorf("add tag: %w", err)
		}
	}

	for _, tag := range removeTags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag = ?`, id, tag); err != nil {
			return nil, fmt.Errorf("remove tag: %w", err)
		}
	}

	for _, depID := range addDeps {
		depID = strings.TrimSpace(depID)
		if depID == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`, id, depID); err != nil {
			return nil, fmt.Errorf("add dependency: %w", err)
		}
	}

	for _, depID := range removeDeps {
		depID = strings.TrimSpace(depID)
		if depID == "" {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on_id = ?`, id, depID); err != nil {
			return nil, fmt.Errorf("remove dependency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return d.GetTask(workspace, id)
}

func (d *DB) DeleteTask(workspace, id string) error {
	result, err := d.db.Exec(`DELETE FROM tasks WHERE id = ? AND workspace = ?`, id, workspace)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

func (d *DB) PendingSummary(workspace string) ([]Task, error) {
	return d.ListTasks(workspace, "", "", "", false)
}

func (d *DB) HasActiveTasks(workspace string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE workspace = ? AND status = 'in_progress'`, workspace,
	).Scan(&count)
	return count > 0, err
}

func (d *DB) getTaskTags(taskID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (d *DB) getTaskDependencies(taskID string) ([]string, error) {
	rows, err := d.db.Query(`SELECT depends_on_id FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

func (d *DB) getSubtasks(workspace, parentID string) ([]Task, error) {
	rows, err := d.db.Query(
		`SELECT id, workspace, title, description, status, priority, parent_id, progress_notes, created_at, updated_at
		 FROM tasks WHERE parent_id = ? AND workspace = ? ORDER BY created_at`, parentID, workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("get subtasks: %w", err)
	}
	defer rows.Close()

	var subtasks []Task
	for rows.Next() {
		var t Task
		var pid sql.NullString
		if err := rows.Scan(&t.ID, &t.Workspace, &t.Title, &t.Description, &t.Status, &t.Priority,
			&pid, &t.ProgressNotes, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.ParentID = pid.String

		tags, err := d.getTaskTags(t.ID)
		if err != nil {
			return nil, err
		}
		t.Tags = tags
		subtasks = append(subtasks, t)
	}
	return subtasks, rows.Err()
}
