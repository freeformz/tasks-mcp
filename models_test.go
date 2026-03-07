package main

import "testing"

func TestTaskStatusValid(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   bool
	}{
		{StatusTodo, true},
		{StatusInProgress, true},
		{StatusDone, true},
		{StatusBlocked, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.status.Valid(); got != tt.want {
			t.Errorf("TaskStatus(%q).Valid() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestTaskPriorityValid(t *testing.T) {
	tests := []struct {
		priority TaskPriority
		want     bool
	}{
		{PriorityLow, true},
		{PriorityMedium, true},
		{PriorityHigh, true},
		{PriorityCritical, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.priority.Valid(); got != tt.want {
			t.Errorf("TaskPriority(%q).Valid() = %v, want %v", tt.priority, got, tt.want)
		}
	}
}
