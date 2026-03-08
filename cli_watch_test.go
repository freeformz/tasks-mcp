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

// --- Single-task mode tests ---

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

// --- List mode tests ---

func testWatchListDB(t *testing.T) (*DB, string) {
	t.Helper()
	db := testDB(t)
	return db, "/test/watch-list-workspace"
}

func newListModeModel(db *DB, workspace string) watchModel {
	return watchModel{
		db:        db,
		workspace: workspace,
		listMode:  true,
		noExit:    true,
		interval:  0,
	}
}

func TestListModeLoadsTasks(t *testing.T) {
	db, ws := testWatchListDB(t)

	_, err := db.CreateTask(ws, "Task A", "", StatusTodo, PriorityHigh, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Task B", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Simulate a tick which loads tasks in list mode.
	updated, cmd := m.Update(tickMsg{})
	um := updated.(watchModel)

	if len(um.tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(um.tasks))
	}

	// Should schedule another tick.
	if cmd == nil {
		t.Fatal("expected tick command after loading tasks")
	}
}

func TestListModeRendersTasks(t *testing.T) {
	db, ws := testWatchListDB(t)

	_, err := db.CreateTask(ws, "Visible task", "", StatusTodo, PriorityHigh, "bob", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	view := m.View()

	if !strings.Contains(view, "Tasks") {
		t.Error("expected 'Tasks' header in view")
	}
	if !strings.Contains(view, "Visible task") {
		t.Error("expected task title in view")
	}
	if !strings.Contains(view, "@bob") {
		t.Error("expected assignee in view")
	}
	if !strings.Contains(view, "j/k: navigate") {
		t.Error("expected help text in view")
	}
}

func TestListModeNavigation(t *testing.T) {
	db, ws := testWatchListDB(t)

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

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	if len(m.tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(m.tasks))
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down with j.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", m.cursor)
	}

	// Move down again.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 after second j, got %d", m.cursor)
	}

	// Move down at bottom should stay.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.cursor)
	}

	// Move up with k.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(watchModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", m.cursor)
	}

	// Arrow keys should also work.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(watchModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 after up arrow, got %d", m.cursor)
	}

	// Up at top should stay.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(watchModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.cursor)
	}
}

func TestListModeDetailView(t *testing.T) {
	db, ws := testWatchListDB(t)

	task, err := db.CreateTask(ws, "Expandable task", "details here", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	if m.view != viewList {
		t.Fatalf("expected viewList, got %d", m.view)
	}

	// Press enter to view details.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)

	// Execute the command to load detail.
	if cmd != nil {
		detailMsg := cmd()
		updated, _ = m.Update(detailMsg)
		m = updated.(watchModel)
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

	// Verify detail view content.
	view := m.View()
	if !strings.Contains(view, "Expandable task") {
		t.Error("expected task title in detail view")
	}
	if !strings.Contains(view, "details here") {
		t.Error("expected description in detail view")
	}
	if !strings.Contains(view, "esc/backspace: back") {
		t.Error("expected help text in detail view")
	}

	// Press esc to go back.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(watchModel)

	if m.view != viewList {
		t.Errorf("expected viewList after esc, got %d", m.view)
	}
	if m.detail != nil {
		t.Error("expected detail to be nil after going back")
	}
}

func TestListModeConfirmClose(t *testing.T) {
	db, ws := testWatchListDB(t)

	_, err := db.CreateTask(ws, "Task to close", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	// Press c to enter confirm mode.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)

	if m.view != viewConfirmClose {
		t.Errorf("expected viewConfirmClose after c, got %d", m.view)
	}

	// Verify confirm view content.
	view := m.View()
	if !strings.Contains(view, "Close task") {
		t.Error("expected close confirmation text")
	}

	// Press n to cancel.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(watchModel)

	if m.view != viewList {
		t.Errorf("expected viewList after n, got %d", m.view)
	}
}

func TestListModeCloseTask(t *testing.T) {
	db, ws := testWatchListDB(t)

	task, err := db.CreateTask(ws, "Task to close", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	// Press c then y.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(watchModel)

	// Execute close command.
	if cmd != nil {
		closeMsg := cmd()
		updated, _ = m.Update(closeMsg)
		m = updated.(watchModel)
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

func TestListModeCloseBlockedBySubtask(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent with child", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Incomplete child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	// Press c then y to confirm close.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(watchModel)

	// Execute close command — should fail.
	if cmd != nil {
		closeMsg := cmd()
		updated, _ = m.Update(closeMsg)
		m = updated.(watchModel)
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

func TestListModeEmptyView(t *testing.T) {
	db, ws := testWatchListDB(t)
	_ = db // no tasks created

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	view := m.View()
	if !strings.Contains(view, "No tasks found") {
		t.Error("expected 'No tasks found' in empty list view")
	}
}

func TestDetailModeSubtaskNavigation(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child A", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child B", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	if m.view != viewDetail {
		t.Fatalf("expected viewDetail, got %d", m.view)
	}
	if m.subtaskCursor != 0 {
		t.Errorf("expected subtaskCursor 0, got %d", m.subtaskCursor)
	}

	// Navigate down.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.subtaskCursor != 1 {
		t.Errorf("expected subtaskCursor 1 after j, got %d", m.subtaskCursor)
	}

	// Navigate past end — should stay at 1.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.subtaskCursor != 1 {
		t.Errorf("expected subtaskCursor 1 at end, got %d", m.subtaskCursor)
	}

	// Navigate up.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(watchModel)
	if m.subtaskCursor != 0 {
		t.Errorf("expected subtaskCursor 0 after k, got %d", m.subtaskCursor)
	}

	// Navigate past start — should stay at 0.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(watchModel)
	if m.subtaskCursor != 0 {
		t.Errorf("expected subtaskCursor 0 at start, got %d", m.subtaskCursor)
	}
}

func TestDetailModeCloseSubtask(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateTask(ws, "Child to close", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	// Press c to close subtask.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)
	if m.view != viewConfirmCloseSubtask {
		t.Errorf("expected viewConfirmCloseSubtask, got %d", m.view)
	}

	// Verify confirm text.
	view := m.View()
	if !strings.Contains(view, "Close subtask") {
		t.Error("expected 'Close subtask' in confirm view")
	}

	// Confirm with y.
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(watchModel)

	// Execute close command.
	if cmd != nil {
		closeMsg := cmd()
		updated, cmd = m.Update(closeMsg)
		m = updated.(watchModel)
	}

	// Verify subtask was closed in DB.
	closed, err := db.GetTask(ws, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != StatusDone {
		t.Errorf("expected subtask status done, got %s", closed.Status)
	}
	if !strings.Contains(closed.ProgressNotes, "Closed manually via CLI") {
		t.Error("expected progress note about CLI closure")
	}

	// Should still be in detail view (not list).
	if m.view != viewDetail {
		t.Errorf("expected viewDetail after subtask close, got %d", m.view)
	}

	// Execute the refresh command (loadDetail triggered by subtaskClosedMsg).
	if cmd != nil {
		refreshMsg := cmd()
		updated, _ = m.Update(refreshMsg)
		m = updated.(watchModel)
	}

	// Verify detail was refreshed with updated subtask status.
	if m.detail == nil {
		t.Fatal("expected detail to be refreshed")
	}
	if len(m.detail.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(m.detail.Subtasks))
	}
	if m.detail.Subtasks[0].Status != StatusDone {
		t.Errorf("expected refreshed subtask status done, got %s", m.detail.Subtasks[0].Status)
	}
}

func TestDetailModeCloseBlockedSubtask(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateTask(ws, "Child with grandchild", "", StatusInProgress, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Add a grandchild that's not done — blocks closing the child.
	_, err = db.CreateTask(ws, "Incomplete grandchild", "", StatusTodo, PriorityMedium, "", child.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	// Press c then y to try closing the child subtask.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(watchModel)

	// Execute close command — should fail.
	if cmd != nil {
		closeMsg := cmd()
		updated, _ = m.Update(closeMsg)
		m = updated.(watchModel)
	}

	// Subtask should NOT be closed.
	task, err := db.GetTask(ws, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status == StatusDone {
		t.Error("expected subtask to remain open when grandchild is incomplete")
	}

	// Model should have an error.
	if m.err == nil {
		t.Error("expected error in model for incomplete grandchild")
	}

	// Error should be visible in the detail view.
	view := m.View()
	if !strings.Contains(view, "Error:") {
		t.Error("expected error to be visible in detail view")
	}
}

func TestDetailModeCancelCloseSubtask(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	// Press c then n to cancel.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)
	if m.view != viewConfirmCloseSubtask {
		t.Errorf("expected viewConfirmCloseSubtask, got %d", m.view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(watchModel)
	if m.view != viewDetail {
		t.Errorf("expected viewDetail after cancel, got %d", m.view)
	}
}

func TestDetailModeNoSubtasksNoCloseOption(t *testing.T) {
	db, ws := testWatchListDB(t)

	_, err := db.CreateTask(ws, "No children", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	// Press c — should NOT enter confirm close subtask (no subtasks).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(watchModel)
	if m.view != viewDetail {
		t.Errorf("expected viewDetail when no subtasks, got %d", m.view)
	}
}

func TestDetailModeSubtaskCursorReset(t *testing.T) {
	db, ws := testWatchListDB(t)

	parent, err := db.CreateTask(ws, "Parent", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child A", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(ws, "Child B", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newListModeModel(db, ws)

	// Load tasks and enter detail.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(watchModel)
	if cmd != nil {
		msg := cmd()
		updated, _ = m.Update(msg)
		m = updated.(watchModel)
	}

	// Move cursor to second subtask.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(watchModel)
	if m.subtaskCursor != 1 {
		t.Fatalf("expected subtaskCursor 1, got %d", m.subtaskCursor)
	}

	// Go back to list.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(watchModel)

	if m.subtaskCursor != 0 {
		t.Errorf("expected subtaskCursor reset to 0 on back, got %d", m.subtaskCursor)
	}
}

func TestListModeQuit(t *testing.T) {
	db, ws := testWatchListDB(t)

	m := newListModeModel(db, ws)

	// Load tasks via tick.
	updated, _ := m.Update(tickMsg{})
	m = updated.(watchModel)

	// Press q to quit.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(watchModel)

	if !m.quitting {
		t.Error("expected quitting to be true")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}

	// Quitting view should be empty.
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when quitting, got %q", view)
	}
}
