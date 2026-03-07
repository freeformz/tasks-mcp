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

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
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

const taskColumns = `id, workspace, title, description, status, priority, assignee, parent_id, progress_notes, created_at, updated_at`

func scanTask(scanner interface{ Scan(...any) error }) (*Task, error) {
	t := &Task{}
	var parentID sql.NullString
	err := scanner.Scan(&t.ID, &t.Workspace, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Assignee, &parentID, &t.ProgressNotes, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.ParentID = parentID.String
	return t, nil
}

func (d *DB) CreateTask(workspace, title, description string, status TaskStatus, priority TaskPriority, assignee, parentID string, tags []string, dependsOn []string) (*Task, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO tasks (id, workspace, title, description, status, priority, assignee, parent_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		id, workspace, title, description, string(status), string(priority), assignee, parentID, now, now,
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
		if err := d.CheckCycle(id, depID); err != nil {
			return nil, err
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
	task, err := scanTask(d.db.QueryRow(
		`SELECT `+taskColumns+` FROM tasks WHERE id = ? AND workspace = ?`, id, workspace,
	))
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

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

func (d *DB) ListTasks(workspace string, filter ListFilter) ([]Task, error) {
	query := `SELECT DISTINCT t.` + strings.ReplaceAll(taskColumns, ", ", ", t.")
	query += ` FROM tasks t`
	var args []any
	var conditions []string

	conditions = append(conditions, "t.workspace = ?")
	args = append(args, workspace)

	if filter.Tag != "" {
		query += ` JOIN task_tags tt ON t.id = tt.task_id`
		conditions = append(conditions, "tt.tag = ?")
		args = append(args, filter.Tag)
	}

	if filter.Status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, filter.Status)
	} else if !filter.IncludeDone {
		conditions = append(conditions, "t.status != 'done'")
	}

	if filter.Assignee != "" {
		conditions = append(conditions, "t.assignee = ?")
		args = append(args, filter.Assignee)
	}

	if filter.ParentID != "" {
		conditions = append(conditions, "t.parent_id = ?")
		args = append(args, filter.ParentID)
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
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

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

		tasks = append(tasks, *t)
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
		if err := d.CheckCycle(id, depID); err != nil {
			return nil, err
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

// CheckCycle detects whether adding a dependency from taskID to newDepID would
// create a circular dependency. It also rejects self-dependencies.
func (d *DB) CheckCycle(taskID, newDepID string) error {
	if taskID == newDepID {
		return fmt.Errorf("cannot add self-dependency: task %s depends on itself", taskID)
	}

	// BFS from newDepID through the dependency graph.
	// If we reach taskID, adding this edge would create a cycle.
	visited := map[string]bool{}
	queue := []string{newDepID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		rows, err := d.db.Query(`SELECT depends_on_id FROM task_dependencies WHERE task_id = ?`, current)
		if err != nil {
			return fmt.Errorf("check cycle: %w", err)
		}

		for rows.Next() {
			var depID string
			if err := rows.Scan(&depID); err != nil {
				rows.Close()
				return fmt.Errorf("check cycle scan: %w", err)
			}
			if depID == taskID {
				rows.Close()
				return fmt.Errorf("circular dependency detected: adding dependency %s -> %s would create a cycle", taskID, newDepID)
			}
			if !visited[depID] {
				queue = append(queue, depID)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("check cycle rows: %w", err)
		}
	}

	return nil
}

// CheckDependencies returns incomplete (non-done) dependencies for a task.
func (d *DB) CheckDependencies(workspace, taskID string) ([]Task, error) {
	rows, err := d.db.Query(`
		SELECT t.`+taskColumns+`
		FROM tasks t
		JOIN task_dependencies td ON t.id = td.depends_on_id
		WHERE td.task_id = ? AND t.workspace = ? AND t.status != 'done'
	`, taskID, workspace)
	if err != nil {
		return nil, fmt.Errorf("check dependencies: %w", err)
	}
	defer rows.Close()

	var incomplete []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		incomplete = append(incomplete, *t)
	}
	return incomplete, rows.Err()
}

func (d *DB) PendingSummary(workspace string) ([]Task, error) {
	return d.ListTasks(workspace, ListFilter{})
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
		`SELECT `+taskColumns+` FROM tasks WHERE parent_id = ? AND workspace = ? ORDER BY created_at`, parentID, workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("get subtasks: %w", err)
	}
	defer rows.Close()

	var subtasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tags, err := d.getTaskTags(t.ID)
		if err != nil {
			return nil, err
		}
		t.Tags = tags
		subtasks = append(subtasks, *t)
	}
	return subtasks, rows.Err()
}
