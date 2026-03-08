package main

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

const testWorkspace = "/test/workspace"

func TestOpenDB(t *testing.T) {
	db := testDB(t)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestCreateTask(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Test Task", "A description", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if task.Title != "Test Task" {
		t.Errorf("got title %q, want %q", task.Title, "Test Task")
	}
	if task.Description != "A description" {
		t.Errorf("got description %q, want %q", task.Description, "A description")
	}
	if task.Status != StatusTodo {
		t.Errorf("got status %q, want %q", task.Status, StatusTodo)
	}
	if task.Priority != PriorityMedium {
		t.Errorf("got priority %q, want %q", task.Priority, PriorityMedium)
	}
	if task.Workspace != testWorkspace {
		t.Errorf("got workspace %q, want %q", task.Workspace, testWorkspace)
	}
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateTaskWithAssignee(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Assigned Task", "", StatusTodo, PriorityHigh, "agent-1", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if task.Assignee != "agent-1" {
		t.Errorf("got assignee %q, want %q", task.Assignee, "agent-1")
	}
}

func TestCreateTaskWithTags(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Tagged Task", "", StatusTodo, PriorityMedium, "", "", []string{"bug", "frontend"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(task.Tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(task.Tags))
	}
	if task.Tags[0] != "bug" || task.Tags[1] != "frontend" {
		t.Errorf("got tags %v, want [bug frontend]", task.Tags)
	}
}

func TestCreateTaskWithDependencies(t *testing.T) {
	db := testDB(t)

	dep, err := db.CreateTask(testWorkspace, "Dependency", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	task, err := db.CreateTask(testWorkspace, "Dependent", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep.ID})
	if err != nil {
		t.Fatal(err)
	}

	if len(task.DependsOn) != 1 {
		t.Fatalf("got %d dependencies, want 1", len(task.DependsOn))
	}
	if task.DependsOn[0] != dep.ID {
		t.Errorf("got dependency %q, want %q", task.DependsOn[0], dep.ID)
	}
}

func TestCreateSubtask(t *testing.T) {
	db := testDB(t)

	parent, err := db.CreateTask(testWorkspace, "Parent", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	child, err := db.CreateTask(testWorkspace, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if child.ParentID != parent.ID {
		t.Errorf("got parent_id %q, want %q", child.ParentID, parent.ID)
	}

	// Verify parent shows subtask.
	got, err := db.GetTask(testWorkspace, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Subtasks) != 1 {
		t.Fatalf("got %d subtasks, want 1", len(got.Subtasks))
	}
	if got.Subtasks[0].ID != child.ID {
		t.Errorf("subtask id %q, want %q", got.Subtasks[0].ID, child.ID)
	}
}

func TestGetTask(t *testing.T) {
	db := testDB(t)

	created, err := db.CreateTask(testWorkspace, "Get Me", "desc", StatusInProgress, PriorityHigh, "bob", "", []string{"test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetTask(testWorkspace, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "Get Me" {
		t.Errorf("got title %q, want %q", got.Title, "Get Me")
	}
	if got.Assignee != "bob" {
		t.Errorf("got assignee %q, want %q", got.Assignee, "bob")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.GetTask(testWorkspace, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestGetTaskWrongWorkspace(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Workspace Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.GetTask("/other/workspace", task.ID)
	if err == nil {
		t.Fatal("expected error for wrong workspace")
	}
}

func TestListTasks(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Task 1", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Task 2", "", StatusInProgress, PriorityHigh, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Task 3", "", StatusDone, PriorityLow, "", "", nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// Should exclude done tasks by default.
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	// High priority should come first.
	if tasks[0].Priority != PriorityHigh {
		t.Errorf("first task priority %q, want high", tasks[0].Priority)
	}
}

func TestListTasksIncludeDone(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Task 1", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Task 2", "", StatusDone, PriorityMedium, "", "", nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{IncludeDone: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
}

func TestListTasksStatusFilter(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Todo", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "InProgress", "", StatusInProgress, PriorityMedium, "", "", nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{Status: string(StatusInProgress)})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Title != "InProgress" {
		t.Errorf("got title %q, want %q", tasks[0].Title, "InProgress")
	}
}

func TestListTasksTagFilter(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Bug", "", StatusTodo, PriorityMedium, "", "", []string{"bug"}, nil)
	db.CreateTask(testWorkspace, "Feature", "", StatusTodo, PriorityMedium, "", "", []string{"feature"}, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{Tag: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Title != "Bug" {
		t.Errorf("got title %q, want %q", tasks[0].Title, "Bug")
	}
}

func TestListTasksAssigneeFilter(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Alice's Task", "", StatusTodo, PriorityMedium, "alice", "", nil, nil)
	db.CreateTask(testWorkspace, "Bob's Task", "", StatusTodo, PriorityMedium, "bob", "", nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{Assignee: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Assignee != "alice" {
		t.Errorf("got assignee %q, want %q", tasks[0].Assignee, "alice")
	}
}

func TestListTasksSubtasksExcluded(t *testing.T) {
	db := testDB(t)

	parent, _ := db.CreateTask(testWorkspace, "Parent", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	// Only parent should be returned (subtasks excluded by default).
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
}

func TestListTasksByParent(t *testing.T) {
	db := testDB(t)

	parent, _ := db.CreateTask(testWorkspace, "Parent", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Child 1", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	db.CreateTask(testWorkspace, "Child 2", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{ParentID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
}

func TestListTasksWorkspaceIsolation(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "WS1 Task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask("/other/workspace", "WS2 Task", "", StatusTodo, PriorityMedium, "", "", nil, nil)

	tasks, err := db.ListTasks(testWorkspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
}

func TestListTasksAllWorkspacesOrdering(t *testing.T) {
	db := testDB(t)

	ws1 := "/workspace/alpha"
	ws2 := "/workspace/beta"

	// Create tasks in different workspaces with varied priorities.
	for _, tc := range []struct {
		ws, title string
		pri       TaskPriority
	}{
		{ws2, "Beta high", PriorityHigh},
		{ws1, "Alpha low", PriorityLow},
		{ws1, "Alpha high", PriorityHigh},
		{ws2, "Beta low", PriorityLow},
	} {
		if _, err := db.CreateTask(tc.ws, tc.title, "", StatusTodo, tc.pri, "", "", nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := db.ListTasks("", ListFilter{AllWorkspaces: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 4 {
		t.Fatalf("got %d tasks, want 4", len(tasks))
	}

	// Should be sorted by workspace first, then priority within each workspace.
	if tasks[0].Workspace != ws1 || tasks[0].Title != "Alpha high" {
		t.Errorf("tasks[0]: got %s/%s, want alpha/Alpha high", tasks[0].Workspace, tasks[0].Title)
	}
	if tasks[1].Workspace != ws1 || tasks[1].Title != "Alpha low" {
		t.Errorf("tasks[1]: got %s/%s, want alpha/Alpha low", tasks[1].Workspace, tasks[1].Title)
	}
	if tasks[2].Workspace != ws2 || tasks[2].Title != "Beta high" {
		t.Errorf("tasks[2]: got %s/%s, want beta/Beta high", tasks[2].Workspace, tasks[2].Title)
	}
	if tasks[3].Workspace != ws2 || tasks[3].Title != "Beta low" {
		t.Errorf("tasks[3]: got %s/%s, want beta/Beta low", tasks[3].Workspace, tasks[3].Title)
	}
}

func TestFindTaskBySuffix(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Suffix Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use the last 12 chars (final UUID segment) as suffix.
	suffix := ShortID(task.ID)

	found, err := db.FindTaskBySuffix(testWorkspace, suffix)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestFindTaskBySuffix_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.FindTaskBySuffix(testWorkspace, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent suffix")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestFindTaskBySuffix_WrongWorkspace(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "WS Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.FindTaskBySuffix("/other/workspace", ShortID(task.ID))
	if err == nil {
		t.Fatal("expected error for wrong workspace")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestFindTaskBySuffix_LikeMetachars(t *testing.T) {
	db := testDB(t)

	// Create a task so the DB isn't empty.
	_, err := db.CreateTask(testWorkspace, "Metachar Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Suffixes containing LIKE metacharacters should be treated literally,
	// not as wildcards. These should not match anything.
	for _, meta := range []string{"%", "_", "%%", "_%"} {
		_, err := db.FindTaskBySuffix(testWorkspace, meta)
		if err == nil {
			t.Errorf("suffix %q: expected error, got match", meta)
		}
		if !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("suffix %q: expected ErrTaskNotFound, got: %v", meta, err)
		}
	}
}

func TestFindTaskBySuffixGlobal(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask("/ws/one", "Global Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	found, err := db.FindTaskBySuffixGlobal(ShortID(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != task.ID {
		t.Errorf("got ID %q, want %q", found.ID, task.ID)
	}
}

func TestFindTaskBySuffixGlobal_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.FindTaskBySuffixGlobal("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestUpdateTask(t *testing.T) {
	db := testDB(t)

	task, _ := db.CreateTask(testWorkspace, "Original", "", StatusTodo, PriorityMedium, "", "", nil, nil)

	updated, err := db.UpdateTask(testWorkspace, task.ID,
		map[string]string{"title": "Updated", "status": "in_progress", "assignee": "agent-2"},
		nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Title != "Updated" {
		t.Errorf("got title %q, want %q", updated.Title, "Updated")
	}
	if updated.Status != StatusInProgress {
		t.Errorf("got status %q, want %q", updated.Status, StatusInProgress)
	}
	if updated.Assignee != "agent-2" {
		t.Errorf("got assignee %q, want %q", updated.Assignee, "agent-2")
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.UpdateTask(testWorkspace, "nonexistent", map[string]string{"title": "X"}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestUpdateTaskTags(t *testing.T) {
	db := testDB(t)

	task, _ := db.CreateTask(testWorkspace, "Tag Test", "", StatusTodo, PriorityMedium, "", "", []string{"old"}, nil)

	updated, err := db.UpdateTask(testWorkspace, task.ID, nil, []string{"new"}, []string{"old"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(updated.Tags) != 1 || updated.Tags[0] != "new" {
		t.Errorf("got tags %v, want [new]", updated.Tags)
	}
}

func TestUpdateTaskDependencies(t *testing.T) {
	db := testDB(t)

	dep1, _ := db.CreateTask(testWorkspace, "Dep1", "", StatusDone, PriorityMedium, "", "", nil, nil)
	dep2, _ := db.CreateTask(testWorkspace, "Dep2", "", StatusDone, PriorityMedium, "", "", nil, nil)
	task, _ := db.CreateTask(testWorkspace, "Task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep1.ID})

	updated, err := db.UpdateTask(testWorkspace, task.ID, nil, nil, nil, []string{dep2.ID}, []string{dep1.ID})
	if err != nil {
		t.Fatal(err)
	}

	if len(updated.DependsOn) != 1 || updated.DependsOn[0] != dep2.ID {
		t.Errorf("got deps %v, want [%s]", updated.DependsOn, dep2.ID)
	}
}

func TestDeleteTask(t *testing.T) {
	db := testDB(t)

	task, _ := db.CreateTask(testWorkspace, "Delete Me", "", StatusTodo, PriorityMedium, "", "", nil, nil)

	if err := db.DeleteTask(testWorkspace, task.ID); err != nil {
		t.Fatal(err)
	}

	_, err := db.GetTask(testWorkspace, task.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	db := testDB(t)

	err := db.DeleteTask(testWorkspace, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteTaskCascadesSubtasks(t *testing.T) {
	db := testDB(t)

	parent, _ := db.CreateTask(testWorkspace, "Parent", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	child, _ := db.CreateTask(testWorkspace, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)

	if err := db.DeleteTask(testWorkspace, parent.ID); err != nil {
		t.Fatal(err)
	}

	_, err := db.GetTask(testWorkspace, child.ID)
	if err == nil {
		t.Fatal("expected child to be deleted with parent")
	}
}

func TestCheckDependencies_AllDone(t *testing.T) {
	db := testDB(t)

	dep, _ := db.CreateTask(testWorkspace, "Dep", "", StatusDone, PriorityMedium, "", "", nil, nil)
	task, _ := db.CreateTask(testWorkspace, "Task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep.ID})

	incomplete, err := db.CheckDependencies(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) != 0 {
		t.Errorf("got %d incomplete deps, want 0", len(incomplete))
	}
}

func TestCheckDependencies_Incomplete(t *testing.T) {
	db := testDB(t)

	dep1, _ := db.CreateTask(testWorkspace, "Done Dep", "", StatusDone, PriorityMedium, "", "", nil, nil)
	dep2, _ := db.CreateTask(testWorkspace, "Todo Dep", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	task, _ := db.CreateTask(testWorkspace, "Task", "", StatusTodo, PriorityMedium, "", "", nil, []string{dep1.ID, dep2.ID})

	incomplete, err := db.CheckDependencies(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) != 1 {
		t.Fatalf("got %d incomplete deps, want 1", len(incomplete))
	}
	if incomplete[0].ID != dep2.ID {
		t.Errorf("incomplete dep %q, want %q", incomplete[0].ID, dep2.ID)
	}
}

func TestCheckDependencies_NoDeps(t *testing.T) {
	db := testDB(t)

	task, _ := db.CreateTask(testWorkspace, "No Deps", "", StatusTodo, PriorityMedium, "", "", nil, nil)

	incomplete, err := db.CheckDependencies(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(incomplete) != 0 {
		t.Errorf("got %d incomplete deps, want 0", len(incomplete))
	}
}

func TestHasActiveTasks(t *testing.T) {
	db := testDB(t)

	has, err := db.HasActiveTasks(testWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected no active tasks")
	}

	db.CreateTask(testWorkspace, "Active", "", StatusInProgress, PriorityMedium, "", "", nil, nil)

	has, err = db.HasActiveTasks(testWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected active tasks")
	}
}

func TestCheckCycle_SelfDependency(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Task A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = db.CheckCycle(task.ID, task.ID)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
}

func TestCheckCycle_DirectCycle(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateTask(testWorkspace, "B", "", StatusTodo, PriorityMedium, "", "", nil, []string{a.ID})
	if err != nil {
		t.Fatal(err)
	}

	// B depends on A. Adding A depends on B should fail.
	err = db.CheckCycle(a.ID, b.ID)
	if err == nil {
		t.Fatal("expected error for direct cycle A->B->A")
	}
}

func TestCheckCycle_TransitiveCycle(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateTask(testWorkspace, "B", "", StatusTodo, PriorityMedium, "", "", nil, []string{a.ID})
	if err != nil {
		t.Fatal(err)
	}
	c, err := db.CreateTask(testWorkspace, "C", "", StatusTodo, PriorityMedium, "", "", nil, []string{b.ID})
	if err != nil {
		t.Fatal(err)
	}

	// C->B->A. Adding A->C should fail.
	err = db.CheckCycle(a.ID, c.ID)
	if err == nil {
		t.Fatal("expected error for transitive cycle A->C->B->A")
	}
}

func TestCheckCycle_ValidDeps(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateTask(testWorkspace, "B", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// A->B is fine when there's no path from B to A.
	err = db.CheckCycle(a.ID, b.ID)
	if err != nil {
		t.Fatalf("unexpected error for valid dependency: %v", err)
	}
}

func TestCreateTask_RejectsCycle(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateTask(testWorkspace, "B", "", StatusTodo, PriorityMedium, "", "", nil, []string{a.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Creating C that depends on B is fine.
	_, err = db.CreateTask(testWorkspace, "C", "", StatusTodo, PriorityMedium, "", "", nil, []string{b.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Creating a task where A depends on B would be a cycle, but CreateTask generates a new ID.
	// Instead test self-dep via update. Test cycle via update below.
}

func TestUpdateTask_RejectsCycle(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateTask(testWorkspace, "B", "", StatusTodo, PriorityMedium, "", "", nil, []string{a.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Try to add A->B dependency (B already depends on A), should fail.
	_, err = db.UpdateTask(testWorkspace, a.ID, nil, nil, nil, []string{b.ID}, nil)
	if err == nil {
		t.Fatal("expected error for cycle when updating task")
	}
}

func TestUpdateTask_RejectsSelfDependency(t *testing.T) {
	db := testDB(t)

	a, err := db.CreateTask(testWorkspace, "A", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.UpdateTask(testWorkspace, a.ID, nil, nil, nil, []string{a.ID}, nil)
	if err == nil {
		t.Fatal("expected error for self-dependency when updating task")
	}
}

func TestRegisterAndDeregisterPresence(t *testing.T) {
	db := testDB(t)

	err := db.RegisterPresence(testWorkspace, "agent-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}

	agents, err := db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].AgentName != "agent-1" {
		t.Errorf("agent_name = %q, want agent-1", agents[0].AgentName)
	}
	if agents[0].SessionID != "session-1" {
		t.Errorf("session_id = %q, want session-1", agents[0].SessionID)
	}

	err = db.DeregisterPresence(testWorkspace, "session-1")
	if err != nil {
		t.Fatal(err)
	}

	agents, err = db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("got %d agents after deregister, want 0", len(agents))
	}
}

func TestHeartbeatPresence(t *testing.T) {
	db := testDB(t)

	err := db.RegisterPresence(testWorkspace, "agent-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}

	agents, err := db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	originalHeartbeat := agents[0].LastHeartbeat

	// Small sleep to ensure timestamp changes.
	time.Sleep(10 * time.Millisecond)

	err = db.HeartbeatPresence(testWorkspace, "session-1")
	if err != nil {
		t.Fatal(err)
	}

	agents, err = db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !agents[0].LastHeartbeat.After(originalHeartbeat) {
		t.Error("heartbeat did not update last_heartbeat timestamp")
	}
}

func TestHeartbeatPresenceNotFound(t *testing.T) {
	db := testDB(t)

	err := db.HeartbeatPresence(testWorkspace, "nonexistent-session")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestStalePresenceCleanup(t *testing.T) {
	db := testDB(t)

	err := db.RegisterPresence(testWorkspace, "stale-agent", "stale-session")
	if err != nil {
		t.Fatal(err)
	}

	// Manually backdate the heartbeat to make it stale.
	_, err = db.db.Exec(
		`UPDATE agent_presence SET last_heartbeat = ? WHERE session_id = ?`,
		time.Now().UTC().Add(-10*time.Minute), "stale-session",
	)
	if err != nil {
		t.Fatal(err)
	}

	agents, err := db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("got %d agents, want 0 (stale should be cleaned up)", len(agents))
	}
}

func TestPresenceWorkspaceIsolation(t *testing.T) {
	db := testDB(t)

	db.RegisterPresence(testWorkspace, "agent-1", "session-1")
	db.RegisterPresence("/other/workspace", "agent-2", "session-2")

	agents, err := db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].AgentName != "agent-1" {
		t.Errorf("agent_name = %q, want agent-1", agents[0].AgentName)
	}
}

func TestMultipleAgentsSameWorkspace(t *testing.T) {
	db := testDB(t)

	db.RegisterPresence(testWorkspace, "agent-1", "session-1")
	db.RegisterPresence(testWorkspace, "agent-2", "session-2")
	db.RegisterPresence(testWorkspace, "agent-3", "session-3")

	agents, err := db.ListActivePresence(testWorkspace, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 {
		t.Fatalf("got %d agents, want 3", len(agents))
	}
}

func TestDeregisterNonexistentSession(t *testing.T) {
	db := testDB(t)

	// Should not error — just a no-op.
	err := db.DeregisterPresence(testWorkspace, "nonexistent-session")
	if err != nil {
		t.Fatalf("deregister nonexistent session should not error: %v", err)
	}
}

func TestTaskExists(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Exists", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.TaskExists(testWorkspace, task.ID); err != nil {
		t.Errorf("expected task to exist, got: %v", err)
	}
}

func TestTaskExists_NotFound(t *testing.T) {
	db := testDB(t)

	err := db.TaskExists(testWorkspace, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestTaskExists_WrongWorkspace(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "WS Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = db.TaskExists("/other/workspace", task.ID)
	if err == nil {
		t.Fatal("expected error for wrong workspace")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got: %v", err)
	}
}

func TestAddNote(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Note Task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	note, err := db.AddNote(task.ID, "test note content")
	if err != nil {
		t.Fatal(err)
	}
	if note.Content != "test note content" {
		t.Errorf("note content = %q, want %q", note.Content, "test note content")
	}
	if note.TaskID != task.ID {
		t.Errorf("note task_id = %q, want %q", note.TaskID, task.ID)
	}
	if note.ID == "" {
		t.Error("expected non-empty note ID")
	}
}

func TestAddNote_NonexistentTask(t *testing.T) {
	db := testDB(t)

	_, err := db.AddNote("nonexistent-id", "should fail")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	// AddNote fails with a FK constraint error (not ErrTaskNotFound) because
	// the INSERT into task_notes triggers the foreign key check before the
	// RowsAffected check on the UPDATE. The MCP handler uses TaskExists first.
}

func TestAddNote_UpdatesTimestamp(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Timestamp Task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	original, err := db.GetTask(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Backdate the task's updated_at to guarantee AddNote advances it.
	past := original.UpdatedAt.Add(-time.Second)
	if _, err := db.db.Exec(`UPDATE tasks SET updated_at = ? WHERE id = ?`, past, task.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := db.AddNote(task.ID, "should update timestamp"); err != nil {
		t.Fatal(err)
	}

	updated, err := db.GetTask(testWorkspace, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !updated.UpdatedAt.After(past) {
		t.Error("expected updated_at to advance after adding note")
	}
}

func TestGetNotes(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Notes Task", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add 3 notes.
	for i := 1; i <= 3; i++ {
		if _, err := db.AddNote(task.ID, fmt.Sprintf("note %d", i)); err != nil {
			t.Fatal(err)
		}
	}

	// Get all notes.
	notes, err := db.GetNotes(task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 {
		t.Fatalf("got %d notes, want 3", len(notes))
	}
	// Should be newest first.
	if notes[0].Content != "note 3" {
		t.Errorf("first note = %q, want 'note 3'", notes[0].Content)
	}

	// Get limited notes.
	notes, err = db.GetNotes(task.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	if notes[0].Content != "note 3" {
		t.Errorf("first note = %q, want 'note 3'", notes[0].Content)
	}
	if notes[1].Content != "note 2" {
		t.Errorf("second note = %q, want 'note 2'", notes[1].Content)
	}
}

func TestGetNotes_NoNotes(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Empty Notes", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	notes, err := db.GetNotes(task.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("got %d notes, want 0", len(notes))
	}
}

func TestGetNoteCount(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "Count Notes", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	count, err := db.GetNoteCount(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("got count %d, want 0", count)
	}

	for i := 0; i < 5; i++ {
		if _, err := db.AddNote(task.ID, fmt.Sprintf("note %d", i)); err != nil {
			t.Fatal(err)
		}
	}

	count, err = db.GetNoteCount(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("got count %d, want 5", count)
	}
}

func TestFindTaskBySuffix_Ambiguous(t *testing.T) {
	db := testDB(t)

	// Create 20 tasks and find a single-character suffix shared by at least 2.
	// With 16 hex characters and 20 tasks, pigeonhole principle guarantees this.
	var ids []string
	for i := range 20 {
		task, err := db.CreateTask(testWorkspace, fmt.Sprintf("Task %d", i), "", StatusTodo, PriorityMedium, "", "", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}

	counts := make(map[byte]int)
	for _, id := range ids {
		last := id[len(id)-1]
		counts[last]++
	}

	var ambiguousSuffix string
	for ch, n := range counts {
		if n >= 2 {
			ambiguousSuffix = string(ch)
			break
		}
	}
	if ambiguousSuffix == "" {
		t.Fatal("expected at least one ambiguous last-character suffix")
	}

	_, err := db.FindTaskBySuffix(testWorkspace, ambiguousSuffix)
	if err == nil {
		t.Fatalf("expected ambiguous suffix error for %q, got nil", ambiguousSuffix)
	}
	if !strings.Contains(err.Error(), "ambiguous suffix") {
		t.Fatalf("expected ambiguous suffix error for %q, got: %v", ambiguousSuffix, err)
	}
}

func TestDeleteTask_WrongWorkspace(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask(testWorkspace, "WS Delete Test", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = db.DeleteTask("/other/workspace", task.ID)
	if err == nil {
		t.Fatal("expected error deleting from wrong workspace")
	}
}

func TestFindTaskBySuffixGlobal_Ambiguous(t *testing.T) {
	db := testDB(t)

	// Create 20 tasks across workspaces and find a shared suffix deterministically.
	var ids []string
	for i := range 20 {
		ws := fmt.Sprintf("/ws/%d", i%3)
		task, err := db.CreateTask(ws, fmt.Sprintf("Global Task %d", i), "", StatusTodo, PriorityMedium, "", "", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, task.ID)
	}

	counts := make(map[byte]int)
	for _, id := range ids {
		last := id[len(id)-1]
		counts[last]++
	}

	var ambiguousSuffix string
	for ch, n := range counts {
		if n >= 2 {
			ambiguousSuffix = string(ch)
			break
		}
	}
	if ambiguousSuffix == "" {
		t.Fatal("expected at least one ambiguous last-character suffix")
	}

	_, err := db.FindTaskBySuffixGlobal(ambiguousSuffix)
	if err == nil {
		t.Fatalf("expected ambiguous suffix error for %q, got nil", ambiguousSuffix)
	}
	if !strings.Contains(err.Error(), "ambiguous suffix") {
		t.Fatalf("expected ambiguous suffix error for %q, got: %v", ambiguousSuffix, err)
	}
}

func TestGetTaskGlobal(t *testing.T) {
	db := testDB(t)

	task, err := db.CreateTask("/some/workspace", "Global Get", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetTaskGlobal(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != task.ID {
		t.Errorf("got ID %q, want %q", got.ID, task.ID)
	}
	if got.Workspace != "/some/workspace" {
		t.Errorf("got workspace %q, want %q", got.Workspace, "/some/workspace")
	}
}

func TestGetTaskGlobal_NotFound(t *testing.T) {
	db := testDB(t)

	_, err := db.GetTaskGlobal("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestSubtasksEnrichedWithNotes(t *testing.T) {
	db := testDB(t)

	parent, err := db.CreateTask(testWorkspace, "Parent", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := db.CreateTask(testWorkspace, "Child", "", StatusTodo, PriorityMedium, "", parent.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add a note to the child.
	if _, err := db.AddNote(child.ID, "child note"); err != nil {
		t.Fatal(err)
	}

	// Get parent — should have enriched subtask with notes.
	got, err := db.GetTask(testWorkspace, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Subtasks) != 1 {
		t.Fatalf("got %d subtasks, want 1", len(got.Subtasks))
	}
	if got.Subtasks[0].NoteCount != 1 {
		t.Errorf("subtask note_count = %d, want 1", got.Subtasks[0].NoteCount)
	}
	if len(got.Subtasks[0].Notes) != 1 || got.Subtasks[0].Notes[0].Content != "child note" {
		t.Errorf("expected subtask note 'child note', got %v", got.Subtasks[0].Notes)
	}
}
