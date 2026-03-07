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

	updates := map[string]string{
		"status":         string(StatusDone),
		"progress_notes": "[2026-01-01 00:00:00] Closed manually via CLI",
	}
	updated, err := db.UpdateTask(testWorkspace, task.ID, updates, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Status != StatusDone {
		t.Errorf("got status %q, want %q", updated.Status, StatusDone)
	}
	if !strings.Contains(updated.ProgressNotes, "Closed manually via CLI") {
		t.Errorf("expected progress notes to contain 'Closed manually via CLI', got %q", updated.ProgressNotes)
	}
}

func TestRunClose_WithNote(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Close with note", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	closedNote := "[2026-01-01 00:00:00] Closed manually via CLI"
	userNote := "[2026-01-01 00:00:00] All done here"
	notes := closedNote + "\n" + userNote

	updates := map[string]string{
		"status":         string(StatusDone),
		"progress_notes": notes,
	}
	updated, err := db.UpdateTask(testWorkspace, task.ID, updates, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Status != StatusDone {
		t.Errorf("got status %q, want %q", updated.Status, StatusDone)
	}
	if !strings.Contains(updated.ProgressNotes, "Closed manually via CLI") {
		t.Errorf("expected 'Closed manually via CLI' in notes, got %q", updated.ProgressNotes)
	}
	if !strings.Contains(updated.ProgressNotes, "All done here") {
		t.Errorf("expected 'All done here' in notes, got %q", updated.ProgressNotes)
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
