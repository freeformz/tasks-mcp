package main

import (
	"github.com/mark3labs/mcp-go/server"
)

func NewServer(db *DB, workspace string) *server.MCPServer {
	srv := server.NewMCPServer(
		"tasks-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(`Task management MCP server for tracking work across sessions.
Use these tools to create, track, and manage tasks for multi-step work.
Tasks are scoped to the current workspace directory.
Always check for existing tasks (task_list) at the start of a session before creating new ones.
Update task status as you make progress. Use task_add_note to log progress.
IMPORTANT: When you finish a task, you MUST set its status to "done" with a final note via task_add_note.
Also mark any subtasks as "done" before marking the parent task.
Assign tasks to team members using the assignee field when working in agent teams.
Dependencies are enforced: you cannot start or complete a task until its dependencies are done.
IMPORTANT: The stop hook fires after every response, not only when the session ends. If you receive a stop hook reminder about in-progress tasks and the user is still actively working with you, ignore the reminder and continue. Do NOT delete tasks in response to the stop hook. Only update task status when the session is genuinely ending.`),
	)

	registerTools(srv, db, workspace)
	registerPresenceTools(srv, db, workspace)
	return srv
}
