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
Update task status as you make progress. Add progress notes to track what was done.
Assign tasks to team members using the assignee field when working in agent teams.
Dependencies are enforced: you cannot start or complete a task until its dependencies are done.`),
	)

	registerTools(srv, db, workspace)
	return srv
}
