package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func newTestToolEnv(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, testWorkspace
}

func makeRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestHandleTaskCreate(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskCreate(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"title":       "Test Task",
		"description": "A test",
		"priority":    "high",
		"assignee":    "agent-1",
		"tags":        "bug,test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var task Task
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &task); err != nil {
		t.Fatal(err)
	}
	if task.Title != "Test Task" {
		t.Errorf("title = %q, want %q", task.Title, "Test Task")
	}
	if task.Assignee != "agent-1" {
		t.Errorf("assignee = %q, want %q", task.Assignee, "agent-1")
	}
	if len(task.Tags) != 2 {
		t.Errorf("tags = %v, want 2 tags", task.Tags)
	}
}

func TestHandleTaskCreateMissingTitle(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskCreate(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing title")
	}
}

func TestHandleTaskCreateInvalidStatus(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskCreate(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"title":  "Bad Status",
		"status": "invalid",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid status")
	}
}

func TestHandleTaskCreateInvalidPriority(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskCreate(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"title":    "Bad Priority",
		"priority": "invalid",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid priority")
	}
}

func TestHandleTaskList(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	listHandler := handleTaskList(db, ws)

	createHandler(t.Context(), makeRequest(map[string]any{"title": "Task 1", "assignee": "alice"}))
	createHandler(t.Context(), makeRequest(map[string]any{"title": "Task 2", "assignee": "bob"}))

	result, err := listHandler(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}

	var tasks []Task
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &tasks); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
}

func TestHandleTaskListByAssignee(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	listHandler := handleTaskList(db, ws)

	createHandler(t.Context(), makeRequest(map[string]any{"title": "Alice Task", "assignee": "alice"}))
	createHandler(t.Context(), makeRequest(map[string]any{"title": "Bob Task", "assignee": "bob"}))

	result, err := listHandler(t.Context(), makeRequest(map[string]any{"assignee": "alice"}))
	if err != nil {
		t.Fatal(err)
	}

	var tasks []Task
	text := result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &tasks)
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Assignee != "alice" {
		t.Errorf("assignee = %q, want alice", tasks[0].Assignee)
	}
}

func TestHandleTaskListInvalidStatus(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskList(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{"status": "invalid"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid status filter")
	}
}

func TestHandleTaskGet(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	getHandler := handleTaskGet(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Get Test"}))
	var created Task
	text := createResult.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &created)

	result, err := getHandler(t.Context(), makeRequest(map[string]any{"id": created.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("unexpected error")
	}
}

func TestHandleTaskGetMissingID(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskGet(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleTaskUpdate(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Update Test"}))
	var created Task
	text := createResult.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &created)

	result, err := updateHandler(t.Context(), makeRequest(map[string]any{
		"id":       created.ID,
		"title":    "Updated",
		"status":   "in_progress",
		"assignee": "agent-x",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var updated Task
	text = result.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &updated)

	if updated.Title != "Updated" {
		t.Errorf("title = %q, want Updated", updated.Title)
	}
	if updated.Status != StatusInProgress {
		t.Errorf("status = %q, want in_progress", updated.Status)
	}
	if updated.Assignee != "agent-x" {
		t.Errorf("assignee = %q, want agent-x", updated.Assignee)
	}
}

func TestHandleTaskAddNote(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	addNoteHandler := handleTaskAddNote(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Note Test"}))
	var created Task
	text := createResult.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &created)

	result, err := addNoteHandler(t.Context(), makeRequest(map[string]any{
		"id":      created.ID,
		"content": "first note",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var updated Task
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &updated)

	if updated.NoteCount != 1 {
		t.Errorf("note_count = %d, want 1", updated.NoteCount)
	}
	if len(updated.Notes) != 1 {
		t.Fatalf("notes len = %d, want 1", len(updated.Notes))
	}
	if updated.Notes[0].Content != "first note" {
		t.Errorf("note content = %q, want 'first note'", updated.Notes[0].Content)
	}
}

func TestHandleTaskAddNoteMissingFields(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskAddNote(db, ws)

	// Missing id.
	result, err := handler(t.Context(), makeRequest(map[string]any{"content": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing id")
	}

	// Missing content.
	result, err = handler(t.Context(), makeRequest(map[string]any{"id": "fake"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing content")
	}
}

func TestHandleTaskAddNoteInvalidMaxNotes(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	addNoteHandler := handleTaskAddNote(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Max Notes Test"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)

	tests := []struct {
		name      string
		maxNotes  float64
		wantError bool
	}{
		{"negative", -1, true},
		{"fractional", 2.5, true},
		{"too large", 10001, true},
		{"valid zero", 0, false},
		{"valid positive", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := addNoteHandler(t.Context(), makeRequest(map[string]any{
				"id":        created.ID,
				"content":   "note for " + tt.name,
				"max_notes": tt.maxNotes,
			}))
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantError && !result.IsError {
				t.Errorf("expected error for max_notes=%v", tt.maxNotes)
			}
			if !tt.wantError && result.IsError {
				t.Errorf("unexpected error for max_notes=%v: %v", tt.maxNotes, result.Content)
			}
		})
	}
}

func TestHandleTaskAddNoteTaskNotFound(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskAddNote(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"id":      "nonexistent-id",
		"content": "should fail",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for nonexistent task")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got %q", text)
	}
}

func TestHandleTaskAddNoteCustomMaxNotes(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	addNoteHandler := handleTaskAddNote(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Custom Max"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)

	// Add 3 notes.
	for i := 1; i <= 3; i++ {
		addNoteHandler(t.Context(), makeRequest(map[string]any{
			"id":      created.ID,
			"content": "note",
		}))
	}

	// Request with max_notes=2 — should only return 2 notes.
	result, err := addNoteHandler(t.Context(), makeRequest(map[string]any{
		"id":        created.ID,
		"content":   "note 4",
		"max_notes": float64(2),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var task Task
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &task)
	if len(task.Notes) != 2 {
		t.Errorf("got %d notes, want 2 (max_notes=2)", len(task.Notes))
	}
	if task.NoteCount != 4 {
		t.Errorf("note_count = %d, want 4", task.NoteCount)
	}
}

func TestHandleTaskListIncludeDone(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)
	listHandler := handleTaskList(db, ws)

	// Create a task and mark it done.
	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "Done Task"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)
	updateHandler(t.Context(), makeRequest(map[string]any{"id": created.ID, "status": "done"}))

	// Without include_done.
	result, _ := listHandler(t.Context(), makeRequest(map[string]any{}))
	var tasks []Task
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &tasks)
	if len(tasks) != 0 {
		t.Errorf("got %d tasks without include_done, want 0", len(tasks))
	}

	// With include_done.
	result, _ = listHandler(t.Context(), makeRequest(map[string]any{"include_done": true}))
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &tasks)
	if len(tasks) != 1 {
		t.Errorf("got %d tasks with include_done, want 1", len(tasks))
	}
}

func TestHandleTaskUpdateMissingID(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskUpdate(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleTaskUpdateInvalidStatus(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "X"}))
	var created Task
	text := createResult.Content[0].(mcp.TextContent).Text
	json.Unmarshal([]byte(text), &created)

	result, err := updateHandler(t.Context(), makeRequest(map[string]any{
		"id":     created.ID,
		"status": "invalid",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid status")
	}
}

func TestHandleTaskUpdateDependencyEnforcement(t *testing.T) {
	db, ws := newTestToolEnv(t)
	ctx := t.Context()
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	// Create a dependency that's not done.
	depResult, _ := createHandler(ctx, makeRequest(map[string]any{"title": "Dependency"}))
	var dep Task
	json.Unmarshal([]byte(depResult.Content[0].(mcp.TextContent).Text), &dep)

	// Create task that depends on it.
	taskResult, _ := createHandler(ctx, makeRequest(map[string]any{
		"title":      "Blocked Task",
		"depends_on": dep.ID,
	}))
	var task Task
	json.Unmarshal([]byte(taskResult.Content[0].(mcp.TextContent).Text), &task)

	// Try to set to in_progress — should fail.
	result, err := updateHandler(ctx, makeRequest(map[string]any{
		"id":     task.ID,
		"status": "in_progress",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error: dependency not done")
	}

	// Try to set to done — should also fail.
	result, err = updateHandler(ctx, makeRequest(map[string]any{
		"id":     task.ID,
		"status": "done",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error: dependency not done")
	}

	// Complete the dependency.
	updateHandler(ctx, makeRequest(map[string]any{
		"id":     dep.ID,
		"status": "done",
	}))

	// Now setting to in_progress should succeed.
	result, err = updateHandler(ctx, makeRequest(map[string]any{
		"id":     task.ID,
		"status": "in_progress",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error after dep completed: %v", result.Content)
	}
}

func TestHandleTaskUpdateSubtaskEnforcement(t *testing.T) {
	db, ws := newTestToolEnv(t)
	ctx := t.Context()
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	// Create a parent task.
	parentResult, err := createHandler(ctx, makeRequest(map[string]any{"title": "Parent"}))
	if err != nil {
		t.Fatal(err)
	}
	if parentResult.IsError {
		t.Fatalf("unexpected error creating parent: %v", parentResult.Content)
	}
	var parent Task
	if err := json.Unmarshal([]byte(parentResult.Content[0].(mcp.TextContent).Text), &parent); err != nil {
		t.Fatalf("unmarshal parent: %v", err)
	}

	// Create a subtask that's not done.
	childResult, err := createHandler(ctx, makeRequest(map[string]any{"title": "Child", "parent_id": parent.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if childResult.IsError {
		t.Fatalf("unexpected error creating child: %v", childResult.Content)
	}

	// Try to set parent to done — should fail.
	result, err := updateHandler(ctx, makeRequest(map[string]any{
		"id":     parent.ID,
		"status": "done",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error: subtask not done")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "subtasks are not yet done") {
		t.Errorf("expected subtask error message, got %q", text)
	}

	// Complete the subtask.
	subtasks, err := db.ListTasks(ws, ListFilter{ParentID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(subtasks) == 0 {
		t.Fatal("expected at least one subtask")
	}
	result, err = updateHandler(ctx, makeRequest(map[string]any{
		"id":     subtasks[0].ID,
		"status": "done",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error completing subtask: %v", result.Content)
	}

	// Now setting parent to done should succeed.
	result, err = updateHandler(ctx, makeRequest(map[string]any{
		"id":     parent.ID,
		"status": "done",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error after subtask completed: %v", result.Content)
	}
}

func TestHandleTaskUpdateUnassign(t *testing.T) {
	db, ws := newTestToolEnv(t)
	ctx := t.Context()
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	createResult, _ := createHandler(ctx, makeRequest(map[string]any{"title": "Assigned", "assignee": "alice"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)

	result, _ := updateHandler(ctx, makeRequest(map[string]any{
		"id":       created.ID,
		"assignee": "",
	}))

	var updated Task
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &updated)
	if updated.Assignee != "" {
		t.Errorf("assignee = %q, want empty", updated.Assignee)
	}
}

func TestHandleTaskDelete(t *testing.T) {
	db, ws := newTestToolEnv(t)
	ctx := t.Context()
	createHandler := handleTaskCreate(db, ws)
	deleteHandler := handleTaskDelete(db, ws)
	getHandler := handleTaskGet(db, ws)

	createResult, _ := createHandler(ctx, makeRequest(map[string]any{"title": "Delete Me"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)

	result, err := deleteHandler(ctx, makeRequest(map[string]any{"id": created.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("unexpected error")
	}

	// Verify deleted.
	getResult, _ := getHandler(ctx, makeRequest(map[string]any{"id": created.ID}))
	if !getResult.IsError {
		t.Fatal("expected error getting deleted task")
	}
}

func TestHandleTaskDeleteNotFound(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskDelete(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{"id": "nonexistent"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestHandleTaskUpdateInvalidPriority(t *testing.T) {
	db, ws := newTestToolEnv(t)
	createHandler := handleTaskCreate(db, ws)
	updateHandler := handleTaskUpdate(db, ws)

	createResult, _ := createHandler(t.Context(), makeRequest(map[string]any{"title": "X"}))
	var created Task
	json.Unmarshal([]byte(createResult.Content[0].(mcp.TextContent).Text), &created)

	result, err := updateHandler(t.Context(), makeRequest(map[string]any{
		"id":       created.ID,
		"priority": "invalid",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid priority")
	}
}

func TestHandleTaskGetNotFound(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskGet(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{"id": "nonexistent"}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestHandleTaskDeleteMissingID(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleTaskDelete(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing id")
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a,b,c", 3},
		{" a , b , c ", 3},
		{"a,,b", 2},
		{",,,", 0},
	}

	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitCSV(%q) = %v (len %d), want len %d", tt.input, got, len(got), tt.want)
		}
	}
}

func TestFormatDependencyError(t *testing.T) {
	tasks := []Task{
		{ID: "abc", Title: "Task A", Status: StatusTodo},
	}
	msg := formatDependencyError("in_progress", tasks)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

// Verify context is passed through (not used in current handlers, but ensure no panic).
func TestHandlersAcceptContext(t *testing.T) {
	db, ws := newTestToolEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancelled context

	// Handlers should still work (SQLite doesn't use context for simple queries).
	handler := handleTaskCreate(db, ws)
	result, err := handler(ctx, makeRequest(map[string]any{"title": "Context Test"}))
	if err != nil {
		t.Fatal(err)
	}
	// May or may not error depending on SQLite behavior with cancelled context.
	_ = result
}

func TestHandleAgentPresenceRegister(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"action":     "register",
		"agent_name": "test-agent",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result.Content)
	}

	var resp map[string]string
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["session_id"] == "" {
		t.Error("expected non-empty session_id")
	}
}

func TestHandleAgentPresenceRegisterMissingName(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"action": "register",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing agent_name")
	}
}

func TestHandleAgentPresenceLifecycle(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	// Register.
	regResult, _ := handler(t.Context(), makeRequest(map[string]any{
		"action":     "register",
		"agent_name": "lifecycle-agent",
	}))
	var resp map[string]string
	json.Unmarshal([]byte(regResult.Content[0].(mcp.TextContent).Text), &resp)
	sessionID := resp["session_id"]

	// List — should see the agent.
	listResult, _ := handler(t.Context(), makeRequest(map[string]any{"action": "list"}))
	var agents []AgentPresence
	json.Unmarshal([]byte(listResult.Content[0].(mcp.TextContent).Text), &agents)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}

	// Heartbeat.
	hbResult, err := handler(t.Context(), makeRequest(map[string]any{
		"action":     "heartbeat",
		"session_id": sessionID,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if hbResult.IsError {
		t.Fatalf("heartbeat error: %v", hbResult.Content)
	}

	// Deregister.
	deregResult, err := handler(t.Context(), makeRequest(map[string]any{
		"action":     "deregister",
		"session_id": sessionID,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if deregResult.IsError {
		t.Fatalf("deregister error: %v", deregResult.Content)
	}

	// List — should be empty.
	listResult, _ = handler(t.Context(), makeRequest(map[string]any{"action": "list"}))
	json.Unmarshal([]byte(listResult.Content[0].(mcp.TextContent).Text), &agents)
	if len(agents) != 0 {
		t.Fatalf("got %d agents after deregister, want 0", len(agents))
	}
}

func TestHandleAgentPresenceInvalidAction(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"action": "invalid",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid action")
	}
}

func TestHandleAgentPresenceHeartbeatMissingSession(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"action": "heartbeat",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing session_id")
	}
}

func TestHandleAgentPresenceDeregisterMissingSession(t *testing.T) {
	db, ws := newTestToolEnv(t)
	handler := handleAgentPresence(db, ws)

	result, err := handler(t.Context(), makeRequest(map[string]any{
		"action": "deregister",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing session_id")
	}
}
