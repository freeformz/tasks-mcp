package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(srv *server.MCPServer, db *DB, workspace string) {
	srv.AddTool(
		mcp.NewTool("task_create",
			mcp.WithDescription("Create a new task. Use for tracking multi-step work, bugs, features, or any unit of work that needs to be done."),
			mcp.WithString("title", mcp.Description("Short descriptive title for the task"), mcp.Required()),
			mcp.WithString("description", mcp.Description("Detailed description of what needs to be done")),
			mcp.WithString("status", mcp.Description("Task status: todo, in_progress, done, blocked (default: todo)")),
			mcp.WithString("priority", mcp.Description("Task priority: low, medium, high, critical (default: medium)")),
			mcp.WithString("assignee", mcp.Description("Agent or team member name to assign this task to")),
			mcp.WithString("parent_id", mcp.Description("ID of parent task to create this as a subtask")),
			mcp.WithString("tags", mcp.Description("Comma-separated tags for categorization")),
			mcp.WithString("depends_on", mcp.Description("Comma-separated task IDs that must be completed before this task")),
		),
		handleTaskCreate(db, workspace),
	)

	srv.AddTool(
		mcp.NewTool("task_list",
			mcp.WithDescription("List tasks in the current workspace. Returns top-level tasks by default (not subtasks). Use parent_id to list subtasks of a specific task."),
			mcp.WithString("status", mcp.Description("Filter by status: todo, in_progress, done, blocked")),
			mcp.WithString("tag", mcp.Description("Filter by tag")),
			mcp.WithString("assignee", mcp.Description("Filter by assignee name")),
			mcp.WithString("parent_id", mcp.Description("List subtasks of this parent task ID")),
			mcp.WithBoolean("include_done", mcp.Description("Include completed tasks (default: false)")),
		),
		handleTaskList(db, workspace),
	)

	srv.AddTool(
		mcp.NewTool("task_get",
			mcp.WithDescription("Get full details of a task including subtasks, tags, and dependencies"),
			mcp.WithString("id", mcp.Description("Task ID"), mcp.Required()),
		),
		handleTaskGet(db, workspace),
	)

	srv.AddTool(
		mcp.NewTool("task_update",
			mcp.WithDescription("Update an existing task. Only specified fields are changed. IMPORTANT: Always set status to 'done' when a task is complete. Use task_add_note to add progress notes."),
			mcp.WithString("id", mcp.Description("Task ID"), mcp.Required()),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("status", mcp.Description("New status: todo, in_progress, done, blocked. Note: transitioning to in_progress or done will fail if dependencies are not yet done.")),
			mcp.WithString("priority", mcp.Description("New priority: low, medium, high, critical")),
			mcp.WithString("assignee", mcp.Description("New assignee name (use empty string to unassign)")),
			mcp.WithString("add_tags", mcp.Description("Comma-separated tags to add")),
			mcp.WithString("remove_tags", mcp.Description("Comma-separated tags to remove")),
			mcp.WithString("add_dependencies", mcp.Description("Comma-separated task IDs to add as dependencies")),
			mcp.WithString("remove_dependencies", mcp.Description("Comma-separated task IDs to remove from dependencies")),
		),
		handleTaskUpdate(db, workspace),
	)

	srv.AddTool(
		mcp.NewTool("task_add_note",
			mcp.WithDescription("Add a timestamped note to a task. Use for progress updates, decisions, blockers, or any information worth recording. Notes are append-only and cannot be edited or deleted."),
			mcp.WithString("id", mcp.Description("Task ID"), mcp.Required()),
			mcp.WithString("content", mcp.Description("Note content"), mcp.Required()),
			mcp.WithNumber("max_notes", mcp.Description("Number of recent notes to return (default: 5, 0 for all)")),
		),
		handleTaskAddNote(db, workspace),
	)

	srv.AddTool(
		mcp.NewTool("task_delete",
			mcp.WithDescription("Delete a task and all its subtasks"),
			mcp.WithString("id", mcp.Description("Task ID"), mcp.Required()),
		),
		handleTaskDelete(db, workspace),
	)
}

func handleTaskCreate(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := request.GetString("title", "")
		if title == "" {
			return errResult("title is required"), nil
		}

		description := request.GetString("description", "")
		assignee := request.GetString("assignee", "")

		status := TaskStatus(request.GetString("status", "todo"))
		if !status.Valid() {
			return errResult("invalid status: must be todo, in_progress, done, or blocked"), nil
		}

		priority := TaskPriority(request.GetString("priority", "medium"))
		if !priority.Valid() {
			return errResult("invalid priority: must be low, medium, high, or critical"), nil
		}

		parentID := request.GetString("parent_id", "")
		tags := splitCSV(request.GetString("tags", ""))
		deps := splitCSV(request.GetString("depends_on", ""))

		task, err := db.CreateTask(workspace, title, description, status, priority, assignee, parentID, tags, deps)
		if err != nil {
			return errResult(fmt.Sprintf("create task: %s", err)), nil
		}
		return jsonResult(task)
	}
}

func handleTaskList(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status := request.GetString("status", "")
		if status != "" && !TaskStatus(status).Valid() {
			return errResult("invalid status filter"), nil
		}

		filter := ListFilter{
			Status:      status,
			Tag:         request.GetString("tag", ""),
			ParentID:    request.GetString("parent_id", ""),
			Assignee:    request.GetString("assignee", ""),
			IncludeDone: request.GetBool("include_done", false),
		}

		tasks, err := db.ListTasks(workspace, filter)
		if err != nil {
			return errResult(fmt.Sprintf("list tasks: %s", err)), nil
		}

		if tasks == nil {
			tasks = []Task{}
		}
		return jsonResult(tasks)
	}
}

func handleTaskGet(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := request.GetString("id", "")
		if id == "" {
			return errResult("id is required"), nil
		}

		task, err := db.GetTask(workspace, id)
		if err != nil {
			return errResult(fmt.Sprintf("get task: %s", err)), nil
		}
		return jsonResult(task)
	}
}

func handleTaskUpdate(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := request.GetString("id", "")
		if id == "" {
			return errResult("id is required"), nil
		}

		updates := make(map[string]string)

		if title := request.GetString("title", ""); title != "" {
			updates["title"] = title
		}
		if desc := request.GetString("description", ""); desc != "" {
			updates["description"] = desc
		}

		newStatus := request.GetString("status", "")
		if newStatus != "" {
			if !TaskStatus(newStatus).Valid() {
				return errResult("invalid status"), nil
			}

			// Dependency enforcement: block in_progress/done if deps are incomplete.
			if newStatus == string(StatusInProgress) || newStatus == string(StatusDone) {
				if err := validateDependencies(db, workspace, id, newStatus); err != nil {
					return errResult(err.Error()), nil
				}
			}

			// Subtask enforcement: block done if any subtasks are not done.
			if newStatus == string(StatusDone) {
				if err := validateSubtasksDone(db, workspace, id); err != nil {
					return errResult(err.Error()), nil
				}
			}

			updates["status"] = newStatus
		}

		if priority := request.GetString("priority", ""); priority != "" {
			if !TaskPriority(priority).Valid() {
				return errResult("invalid priority"), nil
			}
			updates["priority"] = priority
		}

		args := request.GetArguments()
		if _, ok := args["assignee"]; ok {
			updates["assignee"] = request.GetString("assignee", "")
		}

		addTags := splitCSV(request.GetString("add_tags", ""))
		removeTags := splitCSV(request.GetString("remove_tags", ""))
		addDeps := splitCSV(request.GetString("add_dependencies", ""))
		removeDeps := splitCSV(request.GetString("remove_dependencies", ""))

		task, err := db.UpdateTask(workspace, id, updates, addTags, removeTags, addDeps, removeDeps)
		if err != nil {
			return errResult(fmt.Sprintf("update task: %s", err)), nil
		}

		result, err := jsonResult(task)
		if err != nil {
			return nil, err
		}

		if newStatus == string(StatusInProgress) {
			result.Content = append(result.Content, mcp.NewTextContent(
				"Reminder: set this task to 'done' when complete. Use task_add_note to log progress.",
			))
		}

		return result, nil
	}
}

func handleTaskAddNote(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := request.GetString("id", "")
		if id == "" {
			return errResult("id is required"), nil
		}
		content := request.GetString("content", "")
		if content == "" {
			return errResult("content is required"), nil
		}

		maxNotes := int(request.GetFloat("max_notes", 5))
		if maxNotes < 0 {
			return errResult("max_notes must be non-negative"), nil
		}

		// Verify task exists in this workspace with a lightweight check.
		if err := db.TaskExists(workspace, id); err != nil {
			return errResult(fmt.Sprintf("task not found: %s", err)), nil
		}

		if _, err := db.AddNote(id, content); err != nil {
			return errResult(fmt.Sprintf("add note: %s", err)), nil
		}

		// Return the updated task.
		task, err := db.GetTask(workspace, id)
		if err != nil {
			return errResult(fmt.Sprintf("get task: %s", err)), nil
		}

		// If max_notes differs from default 5, re-fetch notes.
		if maxNotes != 5 {
			notes, err := db.GetNotes(id, maxNotes)
			if err != nil {
				return errResult(fmt.Sprintf("get notes: %s", err)), nil
			}
			task.Notes = notes
		}

		return jsonResult(task)
	}
}

func handleTaskDelete(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := request.GetString("id", "")
		if id == "" {
			return errResult("id is required"), nil
		}

		if err := db.DeleteTask(workspace, id); err != nil {
			return errResult(fmt.Sprintf("delete task: %s", err)), nil
		}
		return mcp.NewToolResultText("task deleted"), nil
	}
}

const staleThreshold = 5 * time.Minute

func registerPresenceTools(srv *server.MCPServer, db *DB, workspace string) {
	srv.AddTool(
		mcp.NewTool("agent_presence",
			mcp.WithDescription("Track agent presence in a workspace. Use to register when starting, send heartbeats, deregister when stopping, or list active agents."),
			mcp.WithString("action", mcp.Description("Action to perform: register, heartbeat, deregister, list"), mcp.Required()),
			mcp.WithString("agent_name", mcp.Description("Agent name (required for register)")),
			mcp.WithString("session_id", mcp.Description("Session ID (required for heartbeat and deregister, returned by register)")),
		),
		handleAgentPresence(db, workspace),
	)
}

func handleAgentPresence(db *DB, workspace string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action := request.GetString("action", "")

		switch action {
		case "register":
			agentName := request.GetString("agent_name", "")
			if agentName == "" {
				return errResult("agent_name is required for register"), nil
			}
			sessionID := uuid.New().String()
			if err := db.RegisterPresence(workspace, agentName, sessionID); err != nil {
				return errResult(fmt.Sprintf("register presence: %s", err)), nil
			}
			return jsonResult(map[string]string{"session_id": sessionID})

		case "heartbeat":
			sessionID := request.GetString("session_id", "")
			if sessionID == "" {
				return errResult("session_id is required for heartbeat"), nil
			}
			if err := db.HeartbeatPresence(workspace, sessionID); err != nil {
				return errResult(fmt.Sprintf("heartbeat: %s", err)), nil
			}
			return mcp.NewToolResultText("ok"), nil

		case "deregister":
			sessionID := request.GetString("session_id", "")
			if sessionID == "" {
				return errResult("session_id is required for deregister"), nil
			}
			if err := db.DeregisterPresence(workspace, sessionID); err != nil {
				return errResult(fmt.Sprintf("deregister: %s", err)), nil
			}
			return mcp.NewToolResultText("ok"), nil

		case "list":
			agents, err := db.ListActivePresence(workspace, staleThreshold)
			if err != nil {
				return errResult(fmt.Sprintf("list presence: %s", err)), nil
			}
			if agents == nil {
				agents = []AgentPresence{}
			}
			return jsonResult(agents)

		default:
			return errResult("invalid action: must be register, heartbeat, deregister, or list"), nil
		}
	}
}

func formatDependencyError(targetStatus string, incomplete []Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "cannot set status to %s: the following dependencies are not yet done:\n", targetStatus)
	for _, t := range incomplete {
		fmt.Fprintf(&b, "- %s [%s] (id: %s)\n", t.Title, t.Status, t.ID)
	}
	b.WriteString("Complete or remove these dependencies first.")
	return b.String()
}

// trimSlice returns a new slice with each element trimmed of whitespace,
// dropping any elements that become empty.
func trimSlice(items []string) []string {
	var result []string
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return trimSlice(strings.Split(s, ","))
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(msg)},
		IsError: true,
	}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}
