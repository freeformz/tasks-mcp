package main

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestWriteSnapshot_IdentityAware(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot"

	_, err := db.CreateTask(workspace, "Assigned to me", "", StatusTodo, PriorityHigh, "researcher", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(workspace, "In progress other", "", StatusInProgress, PriorityMedium, "builder", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateTask(workspace, "Unassigned todo", "", StatusTodo, PriorityLow, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	writeSnapshot(&buf, tasks, "researcher")
	output := buf.String()

	if !strings.Contains(output, "## Tasks Assigned to You") {
		t.Error("expected '## Tasks Assigned to You' section")
	}
	if !strings.Contains(output, "Assigned to me") {
		t.Error("expected assigned task in output")
	}
	if !strings.Contains(output, "## In-Progress Tasks") {
		t.Error("expected '## In-Progress Tasks' section")
	}
	if !strings.Contains(output, "In progress other") {
		t.Error("expected in-progress task in output")
	}
	if !strings.Contains(output, "## Pending Tasks") {
		t.Error("expected '## Pending Tasks' section")
	}
	if !strings.Contains(output, "Unassigned todo") {
		t.Error("expected pending task in output")
	}

	// Assigned section should appear before In-Progress section.
	assignedIdx := strings.Index(output, "## Tasks Assigned to You")
	inProgressIdx := strings.Index(output, "## In-Progress Tasks")
	if assignedIdx >= inProgressIdx {
		t.Error("expected assigned section before in-progress section")
	}
}

func TestWriteSnapshot_NoAgentType(t *testing.T) {
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

	var buf bytes.Buffer
	writeSnapshot(&buf, tasks, "")
	output := buf.String()

	// Without agent_type, no assigned section should appear.
	if strings.Contains(output, "## Tasks Assigned to You") {
		t.Error("expected no assigned section without agent_type")
	}
	if !strings.Contains(output, "## In-Progress Tasks") {
		t.Error("expected in-progress section")
	}
}

func TestWriteSnapshot_StatusCounts(t *testing.T) {
	db := testDB(t)
	workspace := "/test/snapshot-counts"

	for _, s := range []TaskStatus{StatusInProgress, StatusInProgress, StatusTodo, StatusBlocked} {
		if _, err := db.CreateTask(workspace, "Task "+string(s), "", s, PriorityMedium, "", "", nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := db.ListTasks(workspace, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	writeSnapshot(&buf, tasks, "")
	output := buf.String()

	if !strings.Contains(output, "2 in_progress") {
		t.Errorf("expected '2 in_progress' in status line, got:\n%s", output)
	}
	if !strings.Contains(output, "1 todo") {
		t.Errorf("expected '1 todo' in status line, got:\n%s", output)
	}
	if !strings.Contains(output, "1 blocked") {
		t.Errorf("expected '1 blocked' in status line, got:\n%s", output)
	}
}

func TestHooksCheckActive_BlocksOnActiveTasks(t *testing.T) {
	db := testDB(t)
	workspace := "/test/check-active"

	// No active tasks initially.
	tasks, err := db.ListTasks(workspace, ListFilter{Status: string(StatusInProgress)})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Error("expected no active tasks")
	}

	// Create an in-progress task.
	_, err = db.CreateTask(workspace, "Active task", "", StatusInProgress, PriorityMedium, "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err = db.ListTasks(workspace, ListFilter{Status: string(StatusInProgress)})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 active task, got %d", len(tasks))
	}
}

func TestFormatActiveTasksReminder_Output(t *testing.T) {
	tasks := []Task{
		{ID: "abc-123", Title: "Fix the bug"},
		{ID: "def-456", Title: "Write tests"},
	}
	output := formatActiveTasksReminder(tasks)

	if !strings.Contains(output, "You have in-progress tasks") {
		t.Error("expected reminder header")
	}
	if !strings.Contains(output, "Fix the bug (id: abc-123)") {
		t.Error("expected first task in reminder")
	}
	if !strings.Contains(output, "Write tests (id: def-456)") {
		t.Error("expected second task in reminder")
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

func TestErrBlockStop(t *testing.T) {
	// Verify the sentinel error exists and has expected message.
	if errBlockStop.Error() != "block stop" {
		t.Errorf("errBlockStop = %q, want %q", errBlockStop.Error(), "block stop")
	}
}
