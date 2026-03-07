# Product Requirements Document: tasks-mcp

## Overview

tasks-mcp is a task management MCP (Model Context Protocol) server designed for AI coding agents. It provides persistent, workspace-scoped task tracking across sessions, enabling agents to plan, track progress, and resume multi-step work reliably. It supports agent teams through task assignment and dependency enforcement.

## Problem Statement

AI coding agents operate in sessions that are inherently ephemeral. When working on complex, multi-step tasks:

- Context is lost between sessions
- There is no record of what was attempted, what succeeded, or what remains
- Multiple agents working in different projects have no isolation of their task state
- Agents cannot plan ahead or break work into manageable pieces with ordering constraints
- Agent teams have no way to coordinate work or avoid duplicating effort

## Goals

1. **Persistent task tracking** — Tasks survive across sessions and context compactions
2. **Workspace isolation** — Each project directory has its own task scope; agents in different projects never interfere
3. **Session continuity** — Agents are automatically reminded of pending work on session start and prompted to update tasks before stopping
4. **Structured decomposition** — Support subtasks and dependencies so agents can break complex work into ordered steps
5. **Progress logging** — Timestamped notes create an audit trail of what was done and when
6. **Zero configuration** — Works out of the box with sensible defaults; database is auto-created
7. **Dependency enforcement** — Prevent agents from starting or completing tasks whose dependencies are not yet done
8. **Agent team support** — Task assignment enables agent teams to coordinate, claim work, and avoid conflicts

## Non-Goals

- User-facing task management UI (this is agent-to-agent infrastructure)
- Cross-machine synchronization (single-machine SQLite)
- Real-time collaboration between agents (tasks are eventually consistent via SQLite WAL)
- Time tracking or scheduling (no due dates, estimates, or calendars)

## Target Users

- AI coding agents (Claude Code, similar MCP-compatible agents)
- Agent teams working collaboratively on the same project
- Developers who use AI agents for multi-step software engineering tasks

## Architecture

### System Design

```
┌─────────────────┐     stdio      ┌──────────────────┐
│  Claude Code    │◄──────────────►│  tasks-mcp       │
│  (AI Agent)     │    MCP JSON    │  (Go binary)     │
└─────────────────┘                └────────┬─────────┘
                                            │
┌─────────────────┐     stdio      ┌────────┤
│  Agent Team     │◄──────────────►│        │
│  (Teammate)     │    MCP JSON    │        │
└─────────────────┘                │        │
                                   ┌────────▼─────────┐
                                   │  SQLite (WAL)    │
                                   │  ~/.local/share/ │
                                   │  tasks-mcp/      │
                                   │  tasks.db        │
                                   └──────────────────┘
```

Multiple MCP server instances can share the same database safely via SQLite WAL mode.

### Components

| Component | Purpose |
|-----------|---------|
| MCP Server | Stdio transport, tool registration, request handling |
| Database Layer | SQLite with WAL mode, schema migration, CRUD operations |
| Dependency Checker | Validates dependency completion and circular dependency detection |
| Presence Tracker | Tracks active agents per workspace with heartbeat-based expiry |
| CLI Subcommands | `pending` and `check-active` for hook scripts |
| Hook Scripts | Shell scripts for SessionStart and Stop integration |
| Rules File | Markdown instructions guiding agent behavior |

### Database

- **Engine**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Location**: `~/.local/share/tasks-mcp/tasks.db`
- **Mode**: WAL (Write-Ahead Logging) for concurrent multi-process access
- **Busy timeout**: 5 seconds
- **Foreign keys**: Enabled (cascade deletes for subtasks)
- **Migrations**: Versioned via `PRAGMA user_version`, sequential and idempotent (see `migrations.go`)

### Workspace Isolation

Tasks are scoped by the absolute path of the project directory. The MCP server determines its workspace from `os.Getwd()` at startup. Multiple MCP server instances (serving different projects) share the same SQLite database but only see tasks matching their workspace.

## Data Model

### Task

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| id | UUID string | auto | generated | Unique identifier |
| workspace | string | auto | cwd | Absolute project directory path |
| title | string | yes | — | Short descriptive title |
| description | string | no | "" | Detailed description of the work |
| status | enum | no | "todo" | Current state (see below) |
| priority | enum | no | "medium" | Importance level (see below) |
| assignee | string | no | "" | Agent or team member name |
| parent_id | UUID string | no | null | Parent task ID (makes this a subtask) |
| progress_notes | text | no | "" | Timestamped log of progress entries |
| created_at | datetime | auto | now | Creation timestamp (UTC) |
| updated_at | datetime | auto | now | Last modification timestamp (UTC) |

### Task Status

| Status | Meaning |
|--------|---------|
| `todo` | Not yet started |
| `in_progress` | Currently being worked on |
| `done` | Completed |
| `blocked` | Cannot proceed; waiting on something |

### Task Priority

| Priority | Meaning |
|----------|---------|
| `low` | Nice to have |
| `medium` | Standard priority |
| `high` | Important, should be addressed soon |
| `critical` | Must be addressed immediately |

### Tags

Many-to-many relationship between tasks and string labels. Stored in `task_tags` junction table. Used for categorization and filtering (e.g., "bug", "feature", "refactor").

### Dependencies

Many-to-many relationship between tasks. Stored in `task_dependencies` junction table. Represents "task A depends on task B" — meaning B should be completed before A.

**Enforcement:** The server enforces dependencies when transitioning task status:
- Setting status to `in_progress` is blocked if any dependency is not `done`
- Setting status to `done` is blocked if any dependency is not `done`
- The error response lists which dependencies are incomplete
- Dependencies can be removed via `remove_dependencies` to unblock if needed

**Circular dependency detection:** When adding a dependency, the server performs a BFS walk of the dependency graph to detect cycles. Self-dependencies and transitive cycles (A→B→C→A) are rejected with a clear error message.

### Subtasks

Hierarchical parent-child relationship via `parent_id`. Subtasks are returned nested under their parent in `task_get`. Deleting a parent cascades to all subtasks.

### Task Assignment

Tasks can be assigned to agents or team members via the `assignee` field:
- Free-form string identifier (agent name, team member name, etc.)
- Tasks can be filtered by assignee in `task_list`
- Unassigned tasks (empty assignee) are available for any team member
- Assignee can be changed or cleared via `task_update`

## MCP Tools

### task_create

Creates a new task in the current workspace.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| title | string | yes | Short descriptive title |
| description | string | no | Detailed description |
| status | string | no | Initial status (default: "todo") |
| priority | string | no | Priority level (default: "medium") |
| assignee | string | no | Agent or team member to assign to |
| parent_id | string | no | Parent task ID for subtask creation |
| tags | string | no | Comma-separated tags |
| depends_on | string | no | Comma-separated task IDs |

**Returns:** Full task object with generated ID.

### task_list

Lists tasks in the current workspace. Returns top-level tasks by default (excludes subtasks unless `parent_id` is specified).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| status | string | no | Filter by status |
| tag | string | no | Filter by tag |
| assignee | string | no | Filter by assignee name |
| parent_id | string | no | List subtasks of this parent |
| include_done | boolean | no | Include completed tasks (default: false) |

**Returns:** Array of task objects, ordered by priority (critical first) then creation time.

### task_get

Gets full details of a single task including subtasks, tags, and dependencies.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | string | yes | Task ID |

**Returns:** Full task object with nested subtasks.

### task_update

Updates an existing task. Only specified fields are modified. Enforces dependency completion for status transitions to `in_progress` or `done`.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | string | yes | Task ID |
| title | string | no | New title |
| description | string | no | New description |
| status | string | no | New status (dependency enforcement applies) |
| priority | string | no | New priority |
| assignee | string | no | New assignee (empty string to unassign) |
| progress_note | string | no | Note appended with timestamp |
| add_tags | string | no | Comma-separated tags to add |
| remove_tags | string | no | Comma-separated tags to remove |
| add_dependencies | string | no | Comma-separated task IDs to add |
| remove_dependencies | string | no | Comma-separated task IDs to remove |

**Returns:** Updated task object.

**Dependency enforcement:** If `status` is set to `in_progress` or `done` and any dependency task is not `done`, the update is rejected with an error listing the incomplete dependencies.

### task_delete

Deletes a task and all its subtasks (cascade).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | string | yes | Task ID |

**Returns:** Confirmation message.

### agent_presence

Track agent presence in a workspace. Supports registration, heartbeats, deregistration, and listing active agents. Stale sessions (no heartbeat in 5+ minutes) are automatically cleaned up.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| action | string | yes | Action: `register`, `heartbeat`, `deregister`, `list` |
| agent_name | string | for register | Agent or team member name |
| session_id | string | for heartbeat/deregister | Session ID returned by register |

**Returns:** For `register`: session ID. For `list`: array of active agents with name, session ID, and timestamps.

## Claude Code Integration

### Hooks

#### SessionStart (startup | resume)

Fires when a Claude Code session starts or resumes. The hook script:

1. Reads JSON input from stdin to extract `cwd`
2. Runs `tasks-mcp pending --workspace <cwd>`
3. Outputs pending tasks as markdown to stdout (including assignee info)
4. Claude receives this as injected context

**Effect:** The agent immediately knows what tasks are pending, who they're assigned to, and what the team is working on.

#### Stop

Fires when Claude is about to stop responding. The hook script:

1. Reads JSON input from stdin to extract `cwd`
2. Runs `tasks-mcp check-active --workspace <cwd>`
3. If in-progress tasks exist, outputs a JSON decision to block stopping and exits with code 2

**Effect:** The agent is reminded to update task status before ending the session.

### Rules

A rules file (`.claude/rules/taskqueue.md`) instructs the agent:

- When to create tasks (multi-step work, cross-session projects)
- How to manage the task lifecycle (create -> in_progress -> progress notes -> done)
- How to use task assignment in agent teams
- How dependency enforcement works
- What fields are available and how to use them
- Best practices (don't create trivial tasks, use tags consistently, assign tasks in teams)

### MCP Server Instructions

The MCP server provides inline instructions via the `WithInstructions` option, giving the agent baseline guidance even without the rules file installed.

## CLI Subcommands

The binary supports two subcommands used by hook scripts:

| Command | Purpose |
|---------|---------|
| `tasks-mcp pending --workspace <path>` | Print markdown summary of non-done tasks with assignee info |
| `tasks-mcp check-active --workspace <path>` | Output JSON decision if in-progress tasks exist |

Running with no arguments starts the MCP server (default mode).

## Technical Requirements

- **Language**: Go 1.26+
- **Database**: SQLite via modernc.org/sqlite (pure Go)
- **MCP SDK**: github.com/mark3labs/mcp-go
- **Transport**: stdio (stdin/stdout JSON)
- **Platform**: macOS, Linux (anywhere Go compiles)
- **Dependencies**: jq required for hook scripts
- **Test coverage**: Minimum 70% statement coverage

## Future Considerations

These are explicitly out of scope for the current version but may be considered later:

- **Task templates** — Pre-defined task structures for common workflows
- **Cross-workspace views** — Query tasks across all workspaces
- **Task archival** — Move old completed tasks out of the active database
- **HTTP transport** — For remote or shared agent setups
- **Notifications** — Proactive reminders for blocked or stale tasks
