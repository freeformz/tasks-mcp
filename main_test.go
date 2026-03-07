package main

import (
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

func TestNewServer(t *testing.T) {
	db := testDB(t)
	srv := NewServer(db, testWorkspace)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}
