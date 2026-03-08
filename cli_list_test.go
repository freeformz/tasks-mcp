package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testListDB(t *testing.T) (*DB, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, "/test/list-workspace"
}

func TestStaticListOutput(t *testing.T) {
	db, ws := testListDB(t)

	// Create tasks with different priorities to test ordering.
	_, err := db.CreateTask(ws, "High task", "", StatusTodo, PriorityHigh, "alice", "", []string{"bug"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Low task", "", StatusInProgress, PriorityLow, "", "", []string{"feature"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := db.ListTasks(ws, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// Capture table output.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	printTaskTable(w, tasks, false, false, db, ws)
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify header columns.
	if !strings.Contains(output, "ID") {
		t.Error("output missing ID column")
	}
	if !strings.Contains(output, "STATUS") {
		t.Error("output missing STATUS column")
	}
	if !strings.Contains(output, "PRIORITY") {
		t.Error("output missing PRIORITY column")
	}
	if !strings.Contains(output, "TITLE") {
		t.Error("output missing TITLE column")
	}
	if !strings.Contains(output, "ASSIGNEE") {
		t.Error("output missing ASSIGNEE column")
	}
	if !strings.Contains(output, "TAGS") {
		t.Error("output missing TAGS column")
	}

	// Verify task content appears.
	if !strings.Contains(output, "High task") {
		t.Error("output missing 'High task'")
	}
	if !strings.Contains(output, "Low task") {
		t.Error("output missing 'Low task'")
	}
	if !strings.Contains(output, "alice") {
		t.Error("output missing assignee 'alice'")
	}
}

func TestStaticListAllWorkspaces(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ws1 := "/workspace/alpha"
	ws2 := "/workspace/beta"

	_, err = db.CreateTask(ws1, "Alpha task", "", StatusTodo, PriorityHigh, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws2, "Beta task", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Without AllWorkspaces, only ws1 tasks.
	tasks, err := db.ListTasks(ws1, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task in ws1, got %d", len(tasks))
	}

	// With AllWorkspaces, both.
	tasks, err = db.ListTasks("", ListFilter{AllWorkspaces: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks across workspaces, got %d", len(tasks))
	}

	// Verify table output includes WORKSPACE column.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	printTaskTable(w, tasks, false, true, db, "")
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "WORKSPACE") {
		t.Error("output missing WORKSPACE column header")
	}
	if !strings.Contains(output, "Alpha task") {
		t.Error("output missing 'Alpha task'")
	}
	if !strings.Contains(output, "Beta task") {
		t.Error("output missing 'Beta task'")
	}
}

func TestStaticListWithSubtasks(t *testing.T) {
	db, ws := testListDB(t)

	parent, err := db.CreateTask(ws, "Parent task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child task", "", StatusTodo, PriorityLow, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := db.ListTasks(ws, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	// Without subtasks flag, only parent should appear.
	if len(tasks) != 1 {
		t.Fatalf("expected 1 top-level task, got %d", len(tasks))
	}

	// With subtasks, child should appear in output.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	printTaskTable(w, tasks, true, false, db, ws)
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Parent task") {
		t.Error("output missing parent task")
	}
	if !strings.Contains(output, "Child task") {
		t.Error("output missing child task when --subtasks is enabled")
	}
}

func TestModelUpdateNavigation(t *testing.T) {
	db, ws := testListDB(t)

	_, err := db.CreateTask(ws, "Task A", "", StatusTodo, PriorityHigh, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Task B", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Task C", "", StatusTodo, PriorityLow, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModel(db, ws)

	// Simulate tasks loaded.
	msg := m.loadTasks()
	updated, _ := m.Update(msg)
	m = updated.(listModel)

	if len(m.tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(listModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.cursor)
	}

	// Move down again.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(listModel)
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 after second j, got %d", m.cursor)
	}

	// Move down at bottom should stay.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(listModel)
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.cursor)
	}

	// Move up.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(listModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m.cursor)
	}

	// Arrow keys should also work.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(listModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 after up arrow, got %d", m.cursor)
	}

	// Up at top should stay.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(listModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestModelUpdateExpandCollapse(t *testing.T) {
	db, ws := testListDB(t)

	task, err := db.CreateTask(ws, "Expandable task", "details here", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModel(db, ws)

	// Load tasks.
	msg := m.loadTasks()
	updated, _ := m.Update(msg)
	m = updated.(listModel)

	if m.view != viewList {
		t.Fatalf("expected viewList, got %d", m.view)
	}

	// Press enter to expand.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(listModel)

	// Execute the command to load detail.
	if cmd != nil {
		detailMsg := cmd()
		updated, _ = m.Update(detailMsg)
		m = updated.(listModel)
	}

	if m.view != viewDetail {
		t.Errorf("expected viewDetail after enter, got %d", m.view)
	}
	if m.detail == nil {
		t.Fatal("expected detail to be loaded")
	}
	if m.detail.ID != task.ID {
		t.Errorf("expected detail ID %s, got %s", task.ID, m.detail.ID)
	}

	// Press esc to go back.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(listModel)

	if m.view != viewList {
		t.Errorf("expected viewList after esc, got %d", m.view)
	}
	if m.detail != nil {
		t.Error("expected detail to be nil after going back")
	}
}

func TestModelConfirmClose(t *testing.T) {
	db, ws := testListDB(t)

	_, err := db.CreateTask(ws, "Task to close", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModel(db, ws)

	// Load tasks.
	msg := m.loadTasks()
	updated, _ := m.Update(msg)
	m = updated.(listModel)

	// Press c to enter confirm mode.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(listModel)

	if m.view != viewConfirmClose {
		t.Errorf("expected viewConfirmClose after c, got %d", m.view)
	}

	// Press n to cancel.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(listModel)

	if m.view != viewList {
		t.Errorf("expected viewList after n, got %d", m.view)
	}
}

func TestModelConfirmCloseBlockedBySubtask(t *testing.T) {
	db, ws := testListDB(t)

	parent, err := db.CreateTask(ws, "Parent with child", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Incomplete child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModel(db, ws)

	// Load tasks.
	msg := m.loadTasks()
	updated, _ := m.Update(msg)
	m = updated.(listModel)

	// Press c then y to confirm close.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(listModel)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(listModel)

	// Execute close command — should fail.
	if cmd != nil {
		closeMsg := cmd()
		updated, _ = m.Update(closeMsg)
		m = updated.(listModel)
	}

	// Task should NOT be closed.
	task, err := db.GetTask(ws, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status == StatusDone {
		t.Error("expected task to remain open when subtask is incomplete")
	}

	// Model should have an error.
	if m.err == nil {
		t.Error("expected error in model for incomplete subtask")
	}
}

func TestModelConfirmCloseYes(t *testing.T) {
	db, ws := testListDB(t)

	task, err := db.CreateTask(ws, "Task to close", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModel(db, ws)

	// Load tasks.
	msg := m.loadTasks()
	updated, _ := m.Update(msg)
	m = updated.(listModel)

	// Press c then y.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(listModel)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(listModel)

	// Execute close command.
	if cmd != nil {
		closeMsg := cmd()
		updated, _ = m.Update(closeMsg)
		m = updated.(listModel)
	}

	// Verify task was closed.
	closed, err := db.GetTask(ws, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != StatusDone {
		t.Errorf("expected task status done, got %s", closed.Status)
	}
	if !strings.Contains(closed.ProgressNotes, "Closed manually via CLI") {
		t.Error("expected progress note about CLI closure")
	}
}
