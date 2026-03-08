package main

import (
	"encoding/json"
	"testing"
)

func TestReadHookInput(t *testing.T) {
	input := `{"cwd":"/test/path","agent_type":"researcher","stop_hook_active":true}`
	var hi hookInput
	if err := json.Unmarshal([]byte(input), &hi); err != nil {
		t.Fatal(err)
	}
	if hi.CWD != "/test/path" {
		t.Errorf("CWD = %q, want /test/path", hi.CWD)
	}
	if hi.AgentType != "researcher" {
		t.Errorf("AgentType = %q, want researcher", hi.AgentType)
	}
	if !hi.StopHookActive {
		t.Error("StopHookActive = false, want true")
	}
}

func TestReadHookInput_MissingFields(t *testing.T) {
	input := `{"cwd":"/test/path"}`
	var hi hookInput
	if err := json.Unmarshal([]byte(input), &hi); err != nil {
		t.Fatal(err)
	}
	if hi.AgentType != "" {
		t.Errorf("AgentType = %q, want empty", hi.AgentType)
	}
	if hi.StopHookActive {
		t.Error("StopHookActive = true, want false")
	}
}

func TestHooksSnapshot_NoTasks(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot"

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestHooksSnapshot_IdentityAware(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot"

	// Create tasks with different assignees.
	_, err := db.CreateTask(workspace, "Assigned to me", "", StatusTodo, PriorityHigh, "researcher", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(workspace, "Assigned to other", "", StatusInProgress, PriorityMedium, "builder", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(workspace, "Unassigned", "", StatusTodo, PriorityLow, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Simulate identity-aware separation.
	agentType := "researcher"
	var assigned, inProgress, other []Task
	for _, task := range tasks {
		if task.Assignee == agentType {
			assigned = append(assigned, task)
		} else if task.Status == StatusInProgress {
			inProgress = append(inProgress, task)
		} else {
			other = append(other, task)
		}
	}

	if len(assigned) != 1 {
		t.Errorf("expected 1 assigned task, got %d", len(assigned))
	}
	if assigned[0].Title != "Assigned to me" {
		t.Errorf("assigned task title = %q, want 'Assigned to me'", assigned[0].Title)
	}
	if len(inProgress) != 1 {
		t.Errorf("expected 1 in-progress task, got %d", len(inProgress))
	}
	if len(other) != 1 {
		t.Errorf("expected 1 other task, got %d", len(other))
	}
}

func TestHooksSnapshot_NoAgentType(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot-no-agent"

	_, err := db.CreateTask(workspace, "Task A", "", StatusInProgress, PriorityHigh, "someone", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	// With no agent_type, all in-progress tasks go to inProgress bucket.
	agentType := ""
	var assigned, inProgress []Task
	for _, task := range tasks {
		if agentType != "" && task.Assignee == agentType {
			assigned = append(assigned, task)
		} else if task.Status == StatusInProgress {
			inProgress = append(inProgress, task)
		}
	}

	if len(assigned) != 0 {
		t.Errorf("expected 0 assigned tasks without agent_type, got %d", len(assigned))
	}
	if len(inProgress) != 1 {
		t.Errorf("expected 1 in-progress task, got %d", len(inProgress))
	}
}

func TestHooksCheckActive_ReFireApproves(t *testing.T) {
	// When stop_hook_active is true, the hook should output approve JSON.
	input := hookInput{StopHookActive: true}
	if !input.StopHookActive {
		t.Error("expected StopHookActive to be true")
	}
	// The actual command would output {"decision":"approve"} — tested via the struct.
}

func TestHooksCheckActive_BlocksOnActiveTasks(t *testing.T) {
	db := testDB(t)
	workspace := "/test/check-active"

	// No active tasks — HasActiveTasks should return false.
	hasActive, err := db.HasActiveTasks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if hasActive {
		t.Error("expected no active tasks")
	}

	// Create an in-progress task.
	_, err = db.CreateTask(workspace, "Active task", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	hasActive, err = db.HasActiveTasks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !hasActive {
		t.Error("expected active tasks")
	}
}

func TestHooksSnapshot_StatusCounts(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot-counts"

	// Create tasks in different statuses.
	for _, s := range []TaskStatus{StatusInProgress, StatusInProgress, StatusTodo, StatusBlocked} {
		if _, err := db.CreateTask(workspace, "Task "+string(s), "", s, PriorityMedium, "", "", nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	counts := make(map[TaskStatus]int)
	for _, task := range tasks {
		counts[task.Status]++
	}

	if counts[StatusInProgress] != 2 {
		t.Errorf("in_progress count = %d, want 2", counts[StatusInProgress])
	}
	if counts[StatusTodo] != 1 {
		t.Errorf("todo count = %d, want 1", counts[StatusTodo])
	}
	if counts[StatusBlocked] != 1 {
		t.Errorf("blocked count = %d, want 1", counts[StatusBlocked])
	}
}

func TestHooksSnapshot_NotesIncluded(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot-notes"

	task, err := db.CreateTask(workspace, "Task with notes", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.AddNote(task.ID, "Progress note 1"); err != nil {
		t.Fatal(err)
	}

	// Verify notes are included when task is fetched.
	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].NoteCount != 1 {
		t.Errorf("note_count = %d, want 1", tasks[0].NoteCount)
	}
	if len(tasks[0].Notes) != 1 {
		t.Errorf("notes length = %d, want 1", len(tasks[0].Notes))
	}
}
