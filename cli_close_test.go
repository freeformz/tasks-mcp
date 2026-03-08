package main

import (
	"strings"
	"testing"
)

func TestRunClose_SetsStatusDone(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Close me", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate what cli_close does: update status, then add note.
	updates := map[string]string{
		"status": string(StatusDone),
	}
	if _, err := db.UpdateTask(testWorkspace, task.ID, updates, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := db.AddNote(task.ID, "Closed manually via CLI"); err != nil {
		t.Fatal(err)
	}

	updated, err := db.GetTask(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Status != StatusDone {
		t.Errorf("got status %q, want %q", updated.Status, StatusDone)
	}
	if updated.NoteCount != 1 {
		t.Errorf("got note count %d, want 1", updated.NoteCount)
	}
	if len(updated.Notes) != 1 || updated.Notes[0].Content != "Closed manually via CLI" {
		t.Errorf("expected note 'Closed manually via CLI', got %v", updated.Notes)
	}
}

func TestRunClose_WithNote(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Close with note", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate cli_close: update status first, then add notes.
	updates := map[string]string{
		"status": string(StatusDone),
	}
	if _, err := db.UpdateTask(testWorkspace, task.ID, updates, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := db.AddNote(task.ID, "Closed manually via CLI"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddNote(task.ID, "All done here"); err != nil {
		t.Fatal(err)
	}

	updated, err := db.GetTask(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Status != StatusDone {
		t.Errorf("got status %q, want %q", updated.Status, StatusDone)
	}
	if updated.NoteCount != 2 {
		t.Errorf("got note count %d, want 2", updated.NoteCount)
	}
}

func TestRunClose_FailsWithIncompleteSubtasks(t *testing.T) {
	db := testDB(t)

	parent, err := db.CreateTask(testWorkspace, "Parent task", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.CreateTask(testWorkspace, "Child task", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = validateSubtasksDone(db, testWorkspace, parent.ID)
	if err == nil {
		t.Fatal("expected error for incomplete subtasks, got nil")
	}
	if !strings.Contains(err.Error(), "subtasks are not yet done") {
		t.Errorf("expected subtask error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Child task") {
		t.Errorf("expected child task name in error, got %q", err.Error())
	}
}

func TestRunClose_FailsWithIncompleteDeps(t *testing.T) {
	db := testDB(t)

	dep, err := db.CreateTask(testWorkspace, "Dependency task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	task, err := db.CreateTask(testWorkspace, "Blocked task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Check dependencies -- should return the incomplete dep.
	incomplete, err := db.CheckDependencies(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) == 0 {
		t.Fatal("expected incomplete dependencies, got none")
	}

	errMsg := formatDependencyError("done", incomplete)
	if !strings.Contains(errMsg, "cannot set status to done") {
		t.Errorf("expected dependency error message, got %q", errMsg)
	}
	if !strings.Contains(errMsg, dep.Title) {
		t.Errorf("expected dep title in error, got %q", errMsg)
	}
}
