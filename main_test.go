package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestFormatActiveTasksReminder(t *testing.T) {
	tasks := []Task{
		{ID: "abc", Title: "Task A"},
		{ID: "def", Title: "Task B"},
	}
	msg := formatActiveTasksReminder(tasks)

	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "Task A") {
		t.Error("expected message to contain 'Task A'")
	}
	if !strings.Contains(msg, "Task B") {
		t.Error("expected message to contain 'Task B'")
	}
	if !strings.Contains(msg, "abc") {
		t.Error("expected message to contain task ID 'abc'")
	}
}

func TestFormatActiveTasksReminderEmpty(t *testing.T) {
	msg := formatActiveTasksReminder(nil)
	if msg == "" {
		t.Fatal("expected non-empty message even with no tasks")
	}
}

func TestPrintTask(t *testing.T) {
	// Redirect stdout to capture output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTask(Task{
		ID:       "test-id-123",
		Title:    "Test task",
		Status:   StatusInProgress,
		Priority: PriorityHigh,
		Tags:     []string{"bug", "urgent"},
		Assignee: "alice",
		Subtasks: []Task{
			{ID: "sub-1", Title: "Subtask 1", Status: StatusTodo},
		},
	})

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Test task") {
		t.Errorf("expected task title in output, got %q", output)
	}
	if !strings.Contains(output, "[HIGH]") {
		t.Errorf("expected [HIGH] priority, got %q", output)
	}
	if !strings.Contains(output, "(bug, urgent)") {
		t.Errorf("expected tags, got %q", output)
	}
	if !strings.Contains(output, "@alice") {
		t.Errorf("expected assignee, got %q", output)
	}
	if !strings.Contains(output, "Subtask 1") {
		t.Errorf("expected subtask, got %q", output)
	}
}

func TestPrintTask_Minimal(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTask(Task{
		ID:       "min-id",
		Title:    "Minimal",
		Status:   StatusTodo,
		Priority: PriorityMedium,
	})

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Minimal") {
		t.Errorf("expected task title, got %q", output)
	}
	// No [HIGH]/[CRITICAL], no tags, no assignee.
	if strings.Contains(output, "[") && strings.Contains(output, "MEDIUM") {
		t.Errorf("medium priority should not be shown in brackets, got %q", output)
	}
}

func TestPrintTask_Critical(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTask(Task{
		ID:       "crit-id",
		Title:    "Critical task",
		Status:   StatusBlocked,
		Priority: PriorityCritical,
	})

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "[CRITICAL]") {
		t.Errorf("expected [CRITICAL], got %q", output)
	}
}

func TestResolveWorkspace(t *testing.T) {
	// With explicit workspace.
	ws, err := resolveWorkspace("/explicit/path")
	if err != nil {
		t.Fatal(err)
	}
	if ws != "/explicit/path" {
		t.Errorf("expected /explicit/path, got %q", ws)
	}

	// With empty workspace — should return cwd.
	ws, err = resolveWorkspace("")
	if err != nil {
		t.Fatal(err)
	}
	if ws == "" {
		t.Error("expected non-empty workspace from cwd")
	}
}

func TestNewServer(t *testing.T) {
	db := testDB(t)
	srv := NewServer(db, testWorkspace)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}
