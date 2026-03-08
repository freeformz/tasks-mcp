package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ShortID returns the last segment of a UUID (final 12 hex chars after the last hyphen).
func ShortID(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

// ResolveTaskID resolves a task ID that may be a short suffix or full UUID.
// It tries a direct lookup first, then falls back to suffix matching.
func ResolveTaskID(db *DB, workspace, input string) (*Task, error) {
	// Try exact match first.
	task, err := db.GetTask(workspace, input)
	if err == nil {
		return task, nil
	}

	// Try suffix match.
	task, err = db.FindTaskBySuffix(workspace, input)
	if err != nil {
		return nil, fmt.Errorf("could not resolve task ID %q: %w", input, err)
	}
	return task, nil
}

// FindTaskBySuffix finds a task whose ID ends with the given suffix.
// Returns an error if zero or multiple tasks match.
func (d *DB) FindTaskBySuffix(workspace, suffix string) (*Task, error) {
	rows, err := d.db.Query(
		`SELECT `+taskColumns+` FROM tasks WHERE workspace = ? AND id LIKE ?`,
		workspace, "%"+suffix,
	)
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
		return nil, fmt.Errorf("no task found matching suffix %q", suffix)
	case 1:
		return d.GetTask(workspace, matches[0].ID)
	default:
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, fmt.Errorf("ambiguous suffix %q matches %d tasks: %s", suffix, len(matches), strings.Join(ids, ", "))
	}
}

var (
	statusStyleDone       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	statusStyleInProgress = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	statusStyleBlocked    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	statusStyleTodo       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // dim/gray
	priorityStyleCritical = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	priorityStyleHigh     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// StyledStatus returns a colorized status string.
func StyledStatus(status TaskStatus) string {
	s := string(status)
	switch status {
	case StatusDone:
		return statusStyleDone.Render(s)
	case StatusInProgress:
		return statusStyleInProgress.Render(s)
	case StatusBlocked:
		return statusStyleBlocked.Render(s)
	default:
		return statusStyleTodo.Render(s)
	}
}

// StyledPriority returns a colorized priority string for high/critical.
func StyledPriority(priority TaskPriority) string {
	s := string(priority)
	switch priority {
	case PriorityCritical:
		return priorityStyleCritical.Render(s)
	case PriorityHigh:
		return priorityStyleHigh.Render(s)
	default:
		return s
	}
}

// formatProgressNote creates a timestamped progress note entry.
func formatProgressNote(note string) string {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	return fmt.Sprintf("[%s] %s", timestamp, note)
}

// appendProgressNote appends a new note to existing progress notes.
func appendProgressNote(existing, newNote string) string {
	if existing != "" {
		return existing + "\n" + newNote
	}
	return newNote
}

// resolveWorkspace returns the given workspace if non-empty, or the current working directory.
func resolveWorkspace(workspace string) (string, error) {
	if workspace != "" {
		return workspace, nil
	}
	return os.Getwd()
}

// validateSubtasksDone checks that all subtasks of a task are done.
// Returns an error listing incomplete subtasks, or nil if all are done (or there are no subtasks).
func validateSubtasksDone(db *DB, workspace, taskID string) error {
	subtasks, err := db.ListTasks(workspace, ListFilter{ParentID: taskID, IncludeDone: true})
	if err != nil {
		return fmt.Errorf("check subtasks: %w", err)
	}
	var incomplete []Task
	for _, st := range subtasks {
		if st.Status != StatusDone {
			incomplete = append(incomplete, st)
		}
	}
	if len(incomplete) > 0 {
		var b strings.Builder
		b.WriteString("cannot set status to done: the following subtasks are not yet done:\n")
		for _, t := range incomplete {
			fmt.Fprintf(&b, "- %s [%s] (id: %s)\n", t.Title, t.Status, t.ID)
		}
		b.WriteString("Complete these subtasks first, or delete them if they are no longer needed.")
		return fmt.Errorf("%s", b.String())
	}
	return nil
}

// validateDependencies checks that all dependencies of a task are done.
// Returns an error describing incomplete dependencies, or nil if all are satisfied.
func validateDependencies(db *DB, workspace, taskID, targetStatus string) error {
	incomplete, err := db.CheckDependencies(workspace, taskID)
	if err != nil {
		return fmt.Errorf("check dependencies: %w", err)
	}
	if len(incomplete) > 0 {
		return fmt.Errorf("%s", formatDependencyError(targetStatus, incomplete))
	}
	return nil
}
