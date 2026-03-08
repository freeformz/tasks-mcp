package main

import (
	"database/sql"
	"errors"
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

const taskColumns = `id, workspace, title, description, status, priority, assignee, parent_id, created_at, updated_at`

func scanTask(scanner interface{ Scan(...any) error }) (*Task, error) {
	t := &Task{}
	var parentID sql.NullString
	err := scanner.Scan(&t.ID, &t.Workspace, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Assignee, &parentID, &t.CreatedAt, &t.UpdatedAt)
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

	for _, tag := range trimSlice(tags) {
		if _, err := tx.Exec(`INSERT INTO task_tags (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return nil, fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	for _, depID := range trimSlice(dependsOn) {
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
	return d.getTask(
		`SELECT `+taskColumns+` FROM tasks WHERE id = ? AND workspace = ?`,
		"get task", id, workspace,
	)
}

// GetTaskGlobal retrieves a task by ID without workspace scoping.
func (d *DB) GetTaskGlobal(id string) (*Task, error) {
	return d.getTask(
		`SELECT `+taskColumns+` FROM tasks WHERE id = ?`,
		"get task (global)", id,
	)
}

func (d *DB) getTask(query, errPrefix string, args ...any) (*Task, error) {
	task, err := scanTask(d.db.QueryRow(query, args...))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}

	if err := d.enrichTaskMetadata(task); err != nil {
		return nil, err
	}

	subtasks, err := d.getSubtasks(task.Workspace, task.ID)
	if err != nil {
		return nil, err
	}
	task.Subtasks = subtasks

	return task, nil
}

// escapeLikePattern escapes special characters in SQLite LIKE patterns
// so that the input is treated literally. Uses '\' as the escape character.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// FindTaskBySuffix finds a task whose ID ends with the given suffix within a workspace.
func (d *DB) FindTaskBySuffix(workspace, suffix string) (*Task, error) {
	return d.findBySuffix(
		`SELECT `+taskColumns+` FROM tasks WHERE workspace = ? AND id LIKE ? ESCAPE '\'`,
		func(t *Task) (*Task, error) { return d.GetTask(workspace, t.ID) },
		func(m *Task) string { return m.ID },
		suffix, workspace, "%"+escapeLikePattern(suffix),
	)
}

// FindTaskBySuffixGlobal finds a task whose ID ends with the given suffix across all workspaces.
func (d *DB) FindTaskBySuffixGlobal(suffix string) (*Task, error) {
	return d.findBySuffix(
		`SELECT `+taskColumns+` FROM tasks WHERE id LIKE ? ESCAPE '\'`,
		func(t *Task) (*Task, error) { return d.GetTaskGlobal(t.ID) },
		func(m *Task) string { return fmt.Sprintf("%s (%s)", ShortID(m.ID), m.Workspace) },
		suffix, "%"+escapeLikePattern(suffix),
	)
}

func (d *DB) findBySuffix(query string, fetch func(*Task) (*Task, error), describe func(*Task) string, suffix string, args ...any) (*Task, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find by suffix: %w", err)
	}
	defer rows.Close()

	var matches []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		matches = append(matches, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: no task matching suffix %q", ErrTaskNotFound, suffix)
	case 1:
		return fetch(matches[0])
	default:
		var descs []string
		for _, m := range matches {
			descs = append(descs, describe(m))
		}
		return nil, fmt.Errorf("ambiguous suffix %q matches %d tasks: %s", suffix, len(matches), strings.Join(descs, ", "))
	}
}

func (d *DB) ListTasks(workspace string, filter ListFilter) ([]Task, error) {
	query := `SELECT DISTINCT t.` + strings.ReplaceAll(taskColumns, ", ", ", t.")
	query += ` FROM tasks t`
	var args []any
	var conditions []string

	if !filter.AllWorkspaces {
		conditions = append(conditions, "t.workspace = ?")
		args = append(args, workspace)
	}

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

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	orderBy := " ORDER BY "
	if filter.AllWorkspaces {
		orderBy += "t.workspace, "
	}
	orderBy += "CASE t.priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END, t.created_at"
	query += orderBy

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

		if err := d.enrichTaskMetadata(t); err != nil {
			return nil, err
		}

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

	for _, tag := range trimSlice(addTags) {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return nil, fmt.Errorf("add tag: %w", err)
		}
	}

	for _, tag := range trimSlice(removeTags) {
		if _, err := tx.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag = ?`, id, tag); err != nil {
			return nil, fmt.Errorf("remove tag: %w", err)
		}
	}

	for _, depID := range trimSlice(addDeps) {
		if err := d.CheckCycle(id, depID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`, id, depID); err != nil {
			return nil, fmt.Errorf("add dependency: %w", err)
		}
	}

	for _, depID := range trimSlice(removeDeps) {
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

func (d *DB) enrichTaskMetadata(t *Task) error {
	tags, err := d.queryStrings(`SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, t.ID)
	if err != nil {
		return fmt.Errorf("get tags: %w", err)
	}
	t.Tags = tags

	deps, err := d.queryStrings(`SELECT depends_on_id FROM task_dependencies WHERE task_id = ?`, t.ID)
	if err != nil {
		return fmt.Errorf("get dependencies: %w", err)
	}
	t.DependsOn = deps

	// Fetch last 5 notes and total count in a single query using a window function.
	noteRows, err := d.db.Query(
		`SELECT id, task_id, content, created_at, COUNT(*) OVER () AS total_count
		 FROM task_notes WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 5`, t.ID,
	)
	if err != nil {
		return fmt.Errorf("get notes: %w", err)
	}
	defer noteRows.Close()

	for noteRows.Next() {
		var note TaskNote
		var totalCount int
		if err := noteRows.Scan(&note.ID, &note.TaskID, &note.Content, &note.CreatedAt, &totalCount); err != nil {
			return fmt.Errorf("scan note: %w", err)
		}
		t.Notes = append(t.Notes, note)
		t.NoteCount = totalCount
	}
	if err := noteRows.Err(); err != nil {
		return fmt.Errorf("iterate notes: %w", err)
	}

	return nil
}

// TaskExists checks if a task exists in a workspace.
// Returns ErrTaskNotFound (wrapped) if the task does not exist.
func (d *DB) TaskExists(workspace, id string) error {
	var n int
	err := d.db.QueryRow(`SELECT 1 FROM tasks WHERE id = ? AND workspace = ?`, id, workspace).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("task %s: %w", id, ErrTaskNotFound)
		}
		return fmt.Errorf("check task exists: %w", err)
	}
	return nil
}

// AddNote adds a timestamped note to a task.
func (d *DB) AddNote(taskID, content string) (*TaskNote, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := d.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx for add note: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO task_notes (id, task_id, content, created_at) VALUES (?, ?, ?, ?)`,
		id, taskID, content, now,
	); err != nil {
		return nil, fmt.Errorf("add note: %w", err)
	}

	// Touch the task's updated_at.
	res, err := tx.Exec(`UPDATE tasks SET updated_at = ? WHERE id = ?`, now, taskID)
	if err != nil {
		return nil, fmt.Errorf("update task timestamp: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("update task timestamp rows affected: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("add note: %w", ErrTaskNotFound)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add note: %w", err)
	}
	return &TaskNote{ID: id, TaskID: taskID, Content: content, CreatedAt: now}, nil
}

// GetNotes returns the most recent notes for a task, ordered newest first, limited to n.
// If n <= 0, all notes are returned.
func (d *DB) GetNotes(taskID string, n int) ([]TaskNote, error) {
	query := `SELECT id, task_id, content, created_at FROM task_notes WHERE task_id = ? ORDER BY created_at DESC, rowid DESC`
	var args []any
	args = append(args, taskID)
	if n > 0 {
		query += ` LIMIT ?`
		args = append(args, n)
	}
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get notes: %w", err)
	}
	defer rows.Close()

	var notes []TaskNote
	for rows.Next() {
		var note TaskNote
		if err := rows.Scan(&note.ID, &note.TaskID, &note.Content, &note.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

// GetNoteCount returns the total number of notes for a task.
func (d *DB) GetNoteCount(taskID string) (int, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM task_notes WHERE task_id = ?`, taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get note count: %w", err)
	}
	return count, nil
}

// queryStrings executes a query returning a single string column and collects the results.
func (d *DB) queryStrings(query string, args ...any) ([]string, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (d *DB) RegisterPresence(workspace, agentName, sessionID string) error {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := d.db.Exec(
		`INSERT INTO agent_presence (id, workspace, agent_name, session_id, started_at, last_heartbeat)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, workspace, agentName, sessionID, now, now,
	)
	if err != nil {
		return fmt.Errorf("register presence: %w", err)
	}
	return nil
}

func (d *DB) HeartbeatPresence(workspace, sessionID string) error {
	result, err := d.db.Exec(
		`UPDATE agent_presence SET last_heartbeat = ? WHERE session_id = ? AND workspace = ?`,
		time.Now().UTC(), sessionID, workspace,
	)
	if err != nil {
		return fmt.Errorf("heartbeat presence: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("heartbeat rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

func (d *DB) DeregisterPresence(workspace, sessionID string) error {
	_, err := d.db.Exec(
		`DELETE FROM agent_presence WHERE session_id = ? AND workspace = ?`,
		sessionID, workspace,
	)
	if err != nil {
		return fmt.Errorf("deregister presence: %w", err)
	}
	return nil
}

func (d *DB) ListActivePresence(workspace string, staleThreshold time.Duration) ([]AgentPresence, error) {
	cutoff := time.Now().UTC().Add(-staleThreshold)

	// Delete stale entries.
	if _, err := d.db.Exec(
		`DELETE FROM agent_presence WHERE workspace = ? AND last_heartbeat < ?`,
		workspace, cutoff,
	); err != nil {
		return nil, fmt.Errorf("cleanup stale presence: %w", err)
	}

	rows, err := d.db.Query(
		`SELECT id, workspace, agent_name, session_id, started_at, last_heartbeat
		 FROM agent_presence WHERE workspace = ? ORDER BY started_at`,
		workspace,
	)
	if err != nil {
		return nil, fmt.Errorf("list presence: %w", err)
	}
	defer rows.Close()

	var results []AgentPresence
	for rows.Next() {
		var p AgentPresence
		if err := rows.Scan(&p.ID, &p.Workspace, &p.AgentName, &p.SessionID, &p.StartedAt, &p.LastHeartbeat); err != nil {
			return nil, fmt.Errorf("scan presence: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// getSubtasks loads subtasks for a parent task, including their metadata (tags, deps, notes).
// This results in per-subtask queries, which is acceptable for the typical task tree size
// (< 20 subtasks). For larger trees, consider batch-fetching metadata.
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

		if err := d.enrichTaskMetadata(t); err != nil {
			return nil, fmt.Errorf("enrich subtask: %w", err)
		}
		subtasks = append(subtasks, *t)
	}
	return subtasks, rows.Err()
}
