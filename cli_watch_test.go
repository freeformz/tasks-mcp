package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAllTasksDone(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want bool
	}{
		{
			name: "single done task",
			task: Task{Status: StatusDone},
			want: true,
		},
		{
			name: "single in-progress task",
			task: Task{Status: StatusInProgress},
			want: false,
		},
		{
			name: "done root with done subtasks",
			task: Task{
				Status: StatusDone,
				Subtasks: []Task{
					{Status: StatusDone},
					{Status: StatusDone},
				},
			},
			want: true,
		},
		{
			name: "done root with incomplete subtask",
			task: Task{
				Status: StatusDone,
				Subtasks: []Task{
					{Status: StatusDone},
					{Status: StatusInProgress},
				},
			},
			want: false,
		},
		{
			name: "in-progress root with done subtasks",
			task: Task{
				Status: StatusInProgress,
				Subtasks: []Task{
					{Status: StatusDone},
				},
			},
			want: false,
		},
		{
			name: "nested subtasks all done",
			task: Task{
				Status: StatusDone,
				Subtasks: []Task{
					{
						Status: StatusDone,
						Subtasks: []Task{
							{Status: StatusDone},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "nested subtask not done",
			task: Task{
				Status: StatusDone,
				Subtasks: []Task{
					{
						Status: StatusDone,
						Subtasks: []Task{
							{Status: StatusTodo},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allTasksDone(&tt.task)
			if got != tt.want {
				t.Errorf("allTasksDone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderTaskLine(t *testing.T) {
	task := Task{
		Title:    "Test task",
		Status:   StatusInProgress,
		Priority: PriorityHigh,
		Assignee: "agent-1",
	}

	line := renderTaskLine(&task)

	if !strings.Contains(line, "Test task") {
		t.Errorf("expected task title in line, got %q", line)
	}
	if !strings.Contains(line, "@agent-1") {
		t.Errorf("expected assignee in line, got %q", line)
	}
}

func TestRenderTaskLineNoAssignee(t *testing.T) {
	task := Task{
		Title:    "No assignee",
		Status:   StatusTodo,
		Priority: PriorityMedium,
	}

	line := renderTaskLine(&task)

	if strings.Contains(line, "@") {
		t.Errorf("expected no assignee marker, got %q", line)
	}
}

func TestRenderTreeSingleTask(t *testing.T) {
	task := Task{
		Title:    "Root task",
		Status:   StatusInProgress,
		Priority: PriorityMedium,
	}

	output := renderTree(&task, "", true)

	if !strings.Contains(output, "Root task") {
		t.Errorf("expected root task title, got %q", output)
	}
	// No tree connectors for a single task.
	if strings.Contains(output, "├─") || strings.Contains(output, "└─") {
		t.Errorf("expected no tree connectors for single task, got %q", output)
	}
}

func TestRenderTreeWithSubtasks(t *testing.T) {
	task := Task{
		Title:    "Parent",
		Status:   StatusInProgress,
		Priority: PriorityHigh,
		Subtasks: []Task{
			{Title: "Child 1", Status: StatusDone, Priority: PriorityMedium},
			{Title: "Child 2", Status: StatusInProgress, Priority: PriorityMedium},
			{Title: "Child 3", Status: StatusTodo, Priority: PriorityMedium},
		},
	}

	output := renderTree(&task, "", true)

	if !strings.Contains(output, "Parent") {
		t.Errorf("expected parent title, got %q", output)
	}
	if !strings.Contains(output, "Child 1") {
		t.Errorf("expected child 1, got %q", output)
	}
	if !strings.Contains(output, "Child 3") {
		t.Errorf("expected child 3, got %q", output)
	}
	// First two children should use ├─, last should use └─.
	if !strings.Contains(output, "├─") {
		t.Errorf("expected ├─ connector, got %q", output)
	}
	if !strings.Contains(output, "└─") {
		t.Errorf("expected └─ connector, got %q", output)
	}
}

func TestWatchModelTickRefreshesTask(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Watch me", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := watchModel{
		db:        db,
		workspace: testWorkspace,
		taskID:    task.ID,
		task:      task,
		interval:  0,
		noExit:    false,
	}

	// Update the task in the DB.
	_, err = db.UpdateTask(testWorkspace, task.ID, map[string]string{"title": "Updated title"}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a tick.
	updated, _ := m.Update(tickMsg{})
	um := updated.(watchModel)

	if um.task.Title != "Updated title" {
		t.Errorf("expected updated title, got %q", um.task.Title)
	}
}

func TestWatchModelAllDoneExits(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Done task", "", StatusDone, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := watchModel{
		db:        db,
		workspace: testWorkspace,
		taskID:    task.ID,
		task:      task,
		interval:  0,
		noExit:    false,
	}

	updated, cmd := m.Update(tickMsg{})
	um := updated.(watchModel)

	if !um.allDone {
		t.Error("expected allDone to be true")
	}
	// Should return tea.Quit when all done and noExit is false.
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestWatchModelAllDoneNoExit(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Done task", "", StatusDone, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := watchModel{
		db:        db,
		workspace: testWorkspace,
		taskID:    task.ID,
		task:      task,
		interval:  0,
		noExit:    true,
	}

	updated, cmd := m.Update(tickMsg{})
	um := updated.(watchModel)

	if !um.allDone {
		t.Error("expected allDone to be true")
	}
	// With noExit, should NOT quit — should return a tick command instead.
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tick)")
	}
}

func TestWatchModelQuitOnKeyQ(t *testing.T) {
	m := watchModel{
		task: &Task{Title: "test", Status: StatusInProgress, Priority: PriorityMedium},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestWatchModelViewLoading(t *testing.T) {
	m := watchModel{}
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("expected loading message, got %q", view)
	}
}

func TestWatchModelViewError(t *testing.T) {
	m := watchModel{
		err: fmt.Errorf("test error"),
	}
	view := m.View()
	if !strings.Contains(view, "test error") {
		t.Errorf("expected error message, got %q", view)
	}
}

func TestWatchModelViewAllDone(t *testing.T) {
	m := watchModel{
		task:    &Task{Title: "Done task", Status: StatusDone, Priority: PriorityMedium, ID: "test-id"},
		allDone: true,
	}
	view := m.View()
	if !strings.Contains(view, "All tasks complete!") {
		t.Errorf("expected completion message, got %q", view)
	}
}

func TestWatchModelViewWorkspaceWarning(t *testing.T) {
	m := watchModel{
		task:             &Task{Title: "Remote task", Status: StatusInProgress, Priority: PriorityMedium, ID: "test-id"},
		workspaceWarning: "⚠ Task is from workspace: /other/project",
	}
	view := m.View()
	if !strings.Contains(view, "Task is from workspace: /other/project") {
		t.Errorf("expected workspace warning, got %q", view)
	}
}

func TestResolveTaskIDGlobal(t *testing.T) {
	db := testDB(t)

	ws1 := "/workspace/one"
	ws2 := "/workspace/two"

	task1, err := db.CreateTask(ws1, "Task in ws1", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	task2, err := db.CreateTask(ws2, "Task in ws2", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve task in its own workspace — no warning.
	resolved, warning, err := ResolveTaskIDGlobal(db, ws1, task1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != task1.ID {
		t.Errorf("expected task %s, got %s", task1.ID, resolved.ID)
	}
	if warning != "" {
		t.Errorf("expected no warning, got %q", warning)
	}

	// Resolve task from different workspace — should produce warning.
	resolved, warning, err = ResolveTaskIDGlobal(db, ws1, task2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != task2.ID {
		t.Errorf("expected task %s, got %s", task2.ID, resolved.ID)
	}
	if warning == "" {
		t.Error("expected workspace warning for cross-workspace resolution")
	}
	if !strings.Contains(warning, ws2) {
		t.Errorf("expected warning to mention %s, got %q", ws2, warning)
	}

	// Resolve by suffix from different workspace.
	suffix := ShortID(task2.ID)
	resolved, warning, err = ResolveTaskIDGlobal(db, ws1, suffix)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != task2.ID {
		t.Errorf("expected task %s, got %s", task2.ID, resolved.ID)
	}
	if warning == "" {
		t.Error("expected workspace warning for cross-workspace suffix resolution")
	}
}
