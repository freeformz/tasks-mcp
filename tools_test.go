package main

import (
	"context"
	"encoding/json"
	"path/filepath"
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
		"id":            created.ID,
		"title":         "Updated",
		"status":        "in_progress",
		"assignee":      "agent-x",
		"progress_note": "started work",
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
	if updated.ProgressNotes == "" {
		t.Error("expected progress notes")
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
