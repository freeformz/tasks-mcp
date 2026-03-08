package main

import (
	"errors"
	"testing"
)

func TestShortID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"550e8400-e29b-41d4-a716-446655440000", "446655440000"},
		{"no-hyphens", "hyphens"},
		{"single", "single"},       // no hyphen — return as-is
		{"trailing-", "trailing-"}, // hyphen at end with nothing after
		{"", ""},
	}

	for _, tt := range tests {
		got := ShortID(tt.input)
		if got != tt.want {
			t.Errorf("ShortID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveTaskID_ExactMatch(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Resolve Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	found, err := ResolveTaskID(db, testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestResolveTaskID_SuffixMatch(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Suffix Resolve", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	suffix := ShortID(task.ID)
	found, err := ResolveTaskID(db, testWorkspace, suffix)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestResolveTaskID_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := ResolveTaskID(db, testWorkspace, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestResolveTaskIDGlobal_LocalMatch(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Local Global", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	found, warning, err := ResolveTaskIDGlobal(db, testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if warning != "" {
		t.Errorf("expected no warning for local match, got %q", warning)
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestResolveTaskIDGlobal_CrossWorkspace(t *testing.T) {
	db := testDB(t)

	otherWS := "/other/workspace"
	task, err := db.CreateTask(otherWS, "Cross WS", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	found, warning, err := ResolveTaskIDGlobal(db, testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if warning == "" {
		t.Error("expected warning for cross-workspace match")
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestResolveTaskIDGlobal_NotFound(t *testing.T) {
	db := testDB(t)

	_, _, err := ResolveTaskIDGlobal(db, testWorkspace, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewWorkspaceShortener(t *testing.T) {
	shorten := newWorkspaceShortener()

	// A non-home path should be returned as-is.
	result := shorten("/some/other/path")
	if result != "/some/other/path" {
		t.Errorf("expected unchanged path, got %q", result)
	}
}

func TestValidateSubtasksDone_AllDone(t *testing.T) {
	db := testDB(t)

	parent, err := db.CreateTask(testWorkspace, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	child, err := db.CreateTask(testWorkspace, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Complete the child.
	if _, err := db.UpdateTask(testWorkspace, child.ID, map[string]string{"status": "done"}, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	if err := validateSubtasksDone(db, testWorkspace, parent.ID); err != nil {
		t.Errorf("expected no error when all subtasks done, got: %v", err)
	}
}

func TestValidateSubtasksDone_NoSubtasks(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "No Children", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := validateSubtasksDone(db, testWorkspace, task.ID); err != nil {
		t.Errorf("expected no error for task with no subtasks, got: %v", err)
	}
}

func TestValidateDependencies_Satisfied(t *testing.T) {
	db := testDB(t)

	dep, err := db.CreateTask(testWorkspace, "Dep", "", StatusDone, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	task, err := db.CreateTask(testWorkspace, "Task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep.ID})
	if err != nil {
		t.Fatal(err)
	}

	if err := validateDependencies(db, testWorkspace, task.ID, "done"); err != nil {
		t.Errorf("expected no error when deps satisfied, got: %v", err)
	}
}

func TestValidateDependencies_Unsatisfied(t *testing.T) {
	db := testDB(t)

	dep, err := db.CreateTask(testWorkspace, "Dep", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	task, err := db.CreateTask(testWorkspace, "Task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep.ID})
	if err != nil {
		t.Fatal(err)
	}

	err = validateDependencies(db, testWorkspace, task.ID, "in_progress")
	if err == nil {
		t.Fatal("expected error for unsatisfied dependencies")
	}
}

func TestStyledStatus(t *testing.T) {
	// Just verify no panic and non-empty output for all statuses.
	for _, s := range []TaskStatus{StatusTodo, StatusInProgress, StatusDone, StatusBlocked} {
		result := StyledStatus(s)
		if result == "" {
			t.Errorf("StyledStatus(%q) returned empty", s)
		}
	}
}

func TestStyledPriority(t *testing.T) {
	for _, p := range []TaskPriority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical} {
		result := StyledPriority(p)
		if result == "" {
			t.Errorf("StyledPriority(%q) returned empty", p)
		}
	}
}
