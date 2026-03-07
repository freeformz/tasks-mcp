package main

import (
	"path/filepath"
	"testing"
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

func TestPendingSummary(t *testing.T) {
	db := testDB(t)

	db.CreateTask(testWorkspace, "Todo", "", StatusTodo, PriorityMedium, "", "", nil, nil)
	db.CreateTask(testWorkspace, "Done", "", StatusDone, PriorityMedium, "", "", nil, nil)

	tasks, err := db.PendingSummary(testWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
}
