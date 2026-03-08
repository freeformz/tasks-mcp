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
Tasks are scoped to the current workspace directory.
IMPORTANT: Any work involving more than one step OR involving a team of agents MUST be tracked with tasks. Context compaction can happen at any time — tasks are your persistent memory. Without them, you lose your plan and progress.
Before starting work, use task_list to check for existing tasks. If none exist for the current work, break it down and create them with task_create. Use subtasks (parent_id) to decompose complex tasks.
As you work, set tasks to "in_progress" with task_update when you start them. Log meaningful progress with task_add_note — what you did, what you learned, what's next.
When you finish a task, you MUST set its status to "done" with a final note via task_add_note. All subtasks must be completed before the parent task can be marked done — the server enforces this.
Dependencies are enforced: you cannot start or complete a task until its dependencies are done.
When coordinating with multiple agents, assign tasks using task_update with the agent's name. Assigned tasks are automatically surfaced to each agent at startup.
IMPORTANT: The stop hook fires after every response, not only when the session ends. If you receive a stop hook reminder about in-progress tasks and the user is still actively working with you, ignore the reminder and continue. Do NOT delete tasks in response to the stop hook. Only update task status when the session is genuinely ending.`),
	)

	registerTools(srv, db, workspace)
	registerPresenceTools(srv, db, workspace)
	return srv
}
