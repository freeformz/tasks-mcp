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

func TestFlagValueFrom(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag string
		want string
	}{
		{"found", []string{"cmd", "--workspace", "/path"}, "--workspace", "/path"},
		{"not found", []string{"cmd", "--other", "val"}, "--workspace", ""},
		{"at end", []string{"cmd", "--workspace"}, "--workspace", ""},
		{"empty args", []string{"cmd"}, "--workspace", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flagValueFrom(tt.args, tt.flag)
			if got != tt.want {
				t.Errorf("flagValueFrom(%v, %q) = %q, want %q", tt.args, tt.flag, got, tt.want)
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	db := testDB(t)
	srv := NewServer(db, testWorkspace)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}
