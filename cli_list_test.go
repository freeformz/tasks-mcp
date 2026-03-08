package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	if err := printTaskTable(w, tasks, false, false, db); err != nil {
		t.Fatal(err)
	}
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
	db, _ := testListDB(t)

	ws1 := "/workspace/alpha"
	ws2 := "/workspace/beta"

	_, err := db.CreateTask(ws1, "Alpha task", "", StatusTodo, PriorityHigh, "", "", nil, nil)
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
	if err := printTaskTable(w, tasks, false, true, db); err != nil {
		t.Fatal(err)
	}
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

	if err := printTaskTable(w, tasks, true, false, db); err != nil {
		t.Fatal(err)
	}
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
