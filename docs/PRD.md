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

- Full-featured task management application (the CLI is for monitoring and light interaction, not primary task management)
- Cross-machine synchronization (single-machine SQLite)
- Real-time collaboration between agents (tasks are eventually consistent via SQLite WAL)
- Time tracking or scheduling (no due dates, estimates, or calendars)

## Target Users

- AI coding agents (Claude Code, similar MCP-compatible agents)
- Agent teams working collaboratively on the same project
- Developers who use AI agents for multi-step software engineering tasks
- Developers who want to monitor agent progress and manage tasks from the terminal

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
| CLI (Hook Support) | `hooks` subcommand with nested hook commands (`hooks snapshot`, `hooks check-active`, etc.) |
| CLI (Interactive) | `list`, `watch`, `close` subcommands for human interaction (bubbletea TUI) |
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

**Enforcement:** A parent task cannot be set to `done` if any of its subtasks are not `done`. The error response lists which subtasks are incomplete. Subtasks must be completed or deleted before the parent can be closed. This is enforced in the MCP tool handler, the CLI `close` command, and the interactive TUI.

### Task Assignment

Tasks can be assigned to agents or team members via the `assignee` field:
- Free-form string identifier (agent name, team member name, etc.)
- Tasks can be filtered by assignee in `task_list`
- Unassigned tasks (empty assignee) are available for any team member
- Assignee can be changed or cleared via `task_update`

### Task Notes

Structured, append-only notes stored in the `task_notes` table. Each note has an auto-generated UUID, task ID, content, and creation timestamp.

- Notes are added via `task_add_note` — they cannot be edited or deleted
- Both `task_list` and `task_get` return the last 5 notes and total note count by default
- `task_add_note` accepts a `max_notes` parameter to control how many notes are returned (0 for all)
- Notes are ordered newest-first in the API response
- Notes replace the former `progress_notes` text field, providing structured storage and efficient querying

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
| add_tags | string | no | Comma-separated tags to add |
| remove_tags | string | no | Comma-separated tags to remove |
| add_dependencies | string | no | Comma-separated task IDs to add |
| remove_dependencies | string | no | Comma-separated task IDs to remove |

**Returns:** Updated task object.

**Dependency enforcement:** If `status` is set to `in_progress` or `done` and any dependency task is not `done`, the update is rejected with an error listing the incomplete dependencies.

### task_add_note

Adds a timestamped note to a task. Notes are append-only and cannot be edited or deleted. Use for progress updates, decisions, blockers, or any information worth recording.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | string | yes | Task ID |
| content | string | yes | Note content |
| max_notes | number | no | Non-negative integer; number of recent notes to return (default: 5, 0 for all, max: 10000) |

**Returns:** Updated task object with notes array and note count.

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

Fires when a Claude Code session starts or resumes. The `hooks snapshot` subcommand is invoked directly as the hook command:

1. Reads JSON input from stdin to extract `cwd` and `agent_type` (if present)
2. Queries the database for non-done tasks in the workspace
3. If `agent_type` is present, outputs tasks assigned to this agent first
4. Outputs task state summary to stdout
5. Claude receives this as injected context

**Effect:** The agent immediately knows what tasks exist and which are assigned to it. For subagents, assigned tasks appear first so they can start working immediately.

#### Stop

Fires when Claude is about to stop responding. The `hooks check-active` subcommand is invoked directly as the hook command:

1. Reads JSON input from stdin to extract `cwd` and `stop_hook_active`
2. If `stop_hook_active` is true (re-fire), outputs `{"decision":"approve"}` and exits
3. Queries the database for in-progress tasks in the workspace
4. If in-progress tasks exist, outputs a JSON decision to block stopping and exits with code 2

**Effect:** The agent is reminded to update task status before ending the session.

### Rules

A rules file (`.claude/rules/taskqueue.md`) instructs the agent:

- When to create tasks (multi-step work, cross-session projects)
- How to manage the task lifecycle (create -> in_progress -> notes -> done)
- How to use task assignment in agent teams
- How dependency enforcement works
- What fields are available and how to use them
- Best practices (don't create trivial tasks, use tags consistently, assign tasks in teams)

### MCP Server Instructions

The MCP server provides inline instructions via the `WithInstructions` option. These instructions are re-sent with every MCP tool call, making them the most persistent mechanism for behavioral guidance — they survive context compaction without relying on hooks. The instructions include:

- Task lifecycle guidance (create, update, add notes, mark done)
- Mandatory task creation for multi-step work
- Multi-agent coordination (assign tasks, automatic surfacing at startup)
- Dependency enforcement rules
- Stop hook behavior (don't delete tasks on re-fire)

## CLI

Built with [Cobra](https://github.com/spf13/cobra) and [Fang](https://github.com/charmbracelet/fang) for styled help output, shell completions, and version display. Running with no arguments displays help. The `mcp` subcommand starts the MCP server.

### Hook Subcommands

All hook commands live under `tasks-mcp hooks`. They read hook JSON from stdin and output the appropriate response to stdout. Not intended for direct human use.

| Command | Purpose |
|---------|---------|
| `tasks-mcp hooks snapshot` | Output task state summary for SessionStart (reads `cwd` and `agent_type` from stdin JSON) |
| `tasks-mcp hooks check-active` | Output JSON decision if in-progress tasks exist (reads `cwd` and `stop_hook_active` from stdin JSON) |

### Human-Facing Subcommands

Interactive CLI for developers to monitor agent work and manage tasks. Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [bubbles](https://github.com/charmbracelet/bubbles). Uses [lipgloss](https://github.com/charmbracelet/lipgloss) for styling with status-based coloring (green=done, yellow=in_progress, red=blocked, dim=todo).

**ID display:** Task IDs are full UUIDs (36 chars). In table output, IDs are shown as the last segment (final 12 hex characters after the last `-`) for readability and easy copying. Full IDs are available in `watch` detail views. Commands like `watch` and `close` accept either the short suffix or full UUID, using suffix matching to resolve.

**Code organization:** CLI/TUI code lives in separate files in the root package (`cli_list.go`, `cli_watch.go`, `cli_close.go`). TUI model logic is tested; view rendering is not.

**Notes display:** Notes are shown with timestamps in the detail and watch views. Last 5 notes shown by default with total count.

#### `tasks-mcp list`

Static table of open tasks in the current workspace. Columns: ID (short prefix), Status, Priority, Title, Assignee, Tags. Ordered by priority then creation time. When `--all` is used, includes a Workspace column and shows tasks from all workspaces, sorted by workspace then priority.

- Excludes done tasks by default
- Shows top-level tasks only by default
- Output is plain text, suitable for piping

**Flags:**

| Flag | Description |
|------|-------------|
| `-a`, `--all` | Show tasks across all workspaces. Adds a Workspace column. Mutually exclusive with `--workspace` |
| `--subtasks` | Show subtasks nested under parent tasks |
| `--status <status>` | Filter by status |
| `--assignee <name>` | Filter by assignee |
| `--include-done` | Include completed tasks |
| `--workspace <path>` | Override workspace (default: cwd) |

#### `tasks-mcp watch [id]`

Interactive TUI for monitoring tasks. Works in two modes depending on whether an ID is provided:

**List mode** (no arguments: `tasks-mcp watch`):

Opens a full-screen interactive task list showing all open top-level tasks in the current workspace. Polls for updates at the configured interval.

- Navigate with arrow keys or j/k
- Press Enter to expand a task and see subtasks, notes, and details
- Press `c` to close (mark done) the selected task — prompts for confirmation before closing
- In detail view, navigate subtasks with j/k and press `c` to close a subtask
- Press Esc or Backspace to go back from detail view to the list
- Press `q` to quit

**Task mode** (with ID: `tasks-mcp watch <id>`):

Live-updating TUI that displays a task and its full subtask tree. Polls the database for changes and re-renders automatically. Exits when all tasks in the tree are done.

- Shows task title, status, priority, assignee for each node
- Subtasks displayed as an indented tree
- Status changes are reflected in real-time (via polling)
- New subtasks added by agents appear automatically
- Notes shown with timestamps

**Flags:**

| Flag | Description |
|------|-------------|
| `--interval <duration>` | Poll interval (default: `5s`) |
| `--no-exit` | Stay running after all tasks are done (continue watching for changes) |
| `--workspace <path>` | Override workspace (default: cwd) |

**Task mode behavior:**

- Task ID is resolved across all workspaces, not just the current one. If the task belongs to a different workspace, the TUI displays a warning: `⚠ Task is from workspace: <path>`
- If the task has no subtasks, shows the single task and waits for it to complete (subtasks may be added later by agents)
- Tree updates as agents add subtasks, change status, or add notes
- Displays a summary when all tasks in the tree reach `done`
- Press `q` or Ctrl+C to exit early

#### `tasks-mcp close <id>`

Marks a task as done from the command line.

- Sets status to `done`
- Automatically adds a note: "Closed manually via CLI"
- If `--note` is provided, that note is added in addition to the automatic one
- Prints confirmation with task title
- Fails with error if dependencies are not yet done

**Agent interaction:** If an agent is currently working on a task that is closed via CLI, the task status changes to `done` in the database. The agent will see the updated status (and the "Closed manually via CLI" note) on its next read, and should stop working on it. There is no real-time notification to the agent — it discovers the change on its next database access.

**Flags:**

| Flag | Description |
|------|-------------|
| `--note <text>` | Add a note when closing |
| `--workspace <path>` | Override workspace (default: cwd) |

## Technical Requirements

- **Language**: Go 1.26+
- **Database**: SQLite via modernc.org/sqlite (pure Go)
- **MCP SDK**: github.com/mark3labs/mcp-go
- **CLI**: github.com/spf13/cobra, github.com/charmbracelet/fang (styled help, version, completions)
- **TUI**: github.com/charmbracelet/bubbletea, github.com/charmbracelet/bubbles, github.com/charmbracelet/lipgloss
- **Transport**: stdio (stdin/stdout JSON) for MCP; terminal for CLI
- **Platform**: macOS, Linux (anywhere Go compiles)
- **Test coverage**: Minimum 70% statement coverage. CLI/TUI: test model logic and DB interactions, not view rendering

## Future Considerations

These are explicitly out of scope for the current version but may be considered later:

- **Task templates** — Pre-defined task structures for common workflows
- **Task archival** — Move old completed tasks out of the active database
- **HTTP transport** — For remote or shared agent setups
- **Notifications** — Proactive reminders for blocked or stale tasks
- **Hook-specific busy timeout** — Use a shorter SQLite busy timeout (e.g., 500ms) for hook subcommands to prevent UX hangs when the database is under contention

## Future: Deep Hook Integration

This section describes planned enhancements to hook integration. The P0 migration from shell scripts to Go subcommands is complete — hooks now run as `tasks-mcp hooks snapshot` and `tasks-mcp hooks check-active` directly, with identity-aware output and strengthened MCP server instructions. The layered approach is: MCP server instructions for persistent behavioral guidance, hooks for state injection and enforcement, and rules files for detailed judgment.

### Design Principles

1. **Layered guidance** — Behavioral guidance lives in MCP `WithInstructions` (persistent, re-sent with every tool call). Hooks handle state injection and enforcement. Rules files provide detailed judgment guidance. Each layer serves a distinct purpose
2. **Compact context** — Injected context must be token-efficient; prefer structured summaries over verbose markdown
3. **Fail loudly** — Hook failures should surface errors, not silently swallow them
4. **No new languages** — All hook implementations must be Go subcommands compiled into the existing binary; no shell scripts, Python, Rust, or other runtimes

### Migration: Shell Scripts to Go Subcommands (Completed)

The former shell scripts (`hooks/session-start.sh`, `hooks/on-stop.sh`) and their `jq` dependency have been replaced by Go subcommands under `tasks-mcp hooks` that read stdin directly:

- `hooks/session-start.sh` + `pending` → `tasks-mcp hooks snapshot`
- `hooks/on-stop.sh` + `check-active` → `tasks-mcp hooks check-active`

The `hooks/` directory, shell scripts, and old top-level `pending`/`check-active` commands have been removed. The `.claude/settings.json` hook configuration invokes the binary directly.

### Hook Subcommand: `hooks snapshot` (Implemented)

Replaces the former `pending` subcommand. Used by the SessionStart hook. Reads hook JSON from stdin to extract `cwd` and, if present, `agent_type` (the agent's name, provided by Claude Code for subagents).

**Identity-aware output:** Claude Code includes `agent_type` in hook input for subagents and sessions started with `--agent`, but not for plain `claude` sessions. If `agent_type` is present, tasks assigned to that agent are shown first ("Tasks assigned to you"), followed by remaining workspace tasks for context. If absent, all workspace tasks are shown without assignment filtering. This gives subagents immediate awareness of their assignments at startup without requiring them to call `task_list` with an assignee filter.

**Output includes:**

- Tasks assigned to this agent (if `agent_type` is present, shown first)
- Count of tasks by status (e.g., `2 in_progress, 3 todo, 1 blocked`)
- All non-done tasks with titles, IDs, status, priority, and assignee
- All notes on active tasks (last 5, as returned by the database)

Note: Behavioral guidance (task creation, lifecycle, multi-agent coordination) is handled by the MCP server's `WithInstructions`, which persists across context compactions via MCP tool calls. The snapshot focuses on task state, not behavioral rules.

**Database access:** If the database file does not exist, it is created (same as the MCP server). If the database is locked, the standard busy timeout applies (retry with backoff). If accessing the database errors, the error is returned.

### Hook: Stop (Enhanced)

**Purpose:** Extend the current stop hook with automatic progress notes.

**Trigger:** When the agent is about to stop responding (existing hook).

**Enhancements:**
1. In addition to blocking on in-progress tasks, output a structured summary of what was done this session
2. If in a git repository, optionally include files modified (from `git diff --name-only`); skip gracefully in non-git workspaces
3. Suggest the agent add this summary as a note to active tasks before stopping

**Rationale:** The current stop hook only blocks; it doesn't help the agent write useful notes. Providing a pre-formatted summary makes it easier for the agent to leave a useful trail.

**Limitations:** The hook can only suggest — it cannot force the agent to call `task_add_note`. The git summary is best-effort and must not add significant latency to the stop hook, which fires after every response.

**Dependencies:** `git` is optional (not required). The hook must detect whether the workspace is a git repo and skip the diff summary if not.

### New MCP Tool: `task_claim`

**Purpose:** Atomic task assignment for agent team coordination.

**Behavior:** Sets the assignee on a task only if the task is currently unassigned. Returns success if claimed, error if already assigned to another agent. This prevents the race condition where two agents both read an unassigned task, both decide to claim it, and both set themselves as assignee.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | string | yes | Task ID |
| assignee | string | yes | Agent name to assign |

**Returns:** Updated task object if claimed successfully; error with current assignee if already taken.

**Rationale:** The current `task_update` tool allows setting assignee unconditionally, which creates race conditions in team scenarios. An atomic claim operation is the minimum mechanism needed for reliable team coordination.

### Implementation Priorities

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| ~~P0~~ | ~~Shell script → Go subcommand migration~~ | ~~Medium~~ | ~~Done~~ |
| ~~P0~~ | ~~`hooks snapshot` subcommand~~ | ~~Medium~~ | ~~Done~~ |
| ~~P0~~ | ~~`hooks check-active` subcommand~~ | ~~Low~~ | ~~Done~~ |
| P1 | Enhanced Stop hook | Low | Better session-end behavior |
| P1 | `task_claim` MCP tool | Low | Reliable team task assignment |
| P3 | SubagentStart/Stop hooks | Medium | Agent team coordination (speculative — see below) |

### Deferred Ideas

These ideas were considered but deferred due to poor cost-benefit or feasibility concerns:

- **Compact snapshot format:** A token-efficient variant of `hooks snapshot` that caps blocked/todo categories and truncates notes, for use cases where full output is too verbose (e.g., per-prompt injection). Add a `--format compact` flag when a consumer exists.

- **UserPromptSubmit (state injection):** Injecting task state on every prompt. SessionStart already fires on `compact` events and re-injects task state, and behavioral guidance persists via MCP `WithInstructions` (re-sent with every tool call). Per-prompt state injection adds overhead on every message for a problem already solved by these two mechanisms. Revisit if agents are still losing track of tasks in practice, potentially with a caching approach (full injection on first prompt, condensed reminders after). Would use the compact snapshot format if implemented.

- **PreToolUse nudge (Edit/Write):** Nudging agents to create tasks before file edits has a poor signal-to-noise ratio. The agent cannot distinguish trivial one-off edits from multi-step work, and the nudge fires at the worst moment (mid-execution). The rules file guidance "don't create tasks for trivial operations" contradicts an automatic nudge. Revisit only if a reliable heuristic for "significant work" emerges.

- **PreCompact snapshot:** Writing task state to a temp file before compaction is redundant. SessionStart already fires on `compact` events (configured via `"matcher": "startup|resume|compact"`). A file-based caching layer adds staleness risk and filesystem state management without meaningful benefit over querying the database directly.

- **SubagentStart/SubagentStop hooks:** Automatic presence registration and orphan detection are valuable concepts, but these hook events may not be available in all Claude Code versions. Implement only after confirming hook event availability. The orphaned-task detection (SubagentStop) is higher value than the registration (SubagentStart) — if only one is built, prioritize orphan detection.

### Constraints and Risks

- **Hook latency:** Each hook adds overhead. The `hooks snapshot` subcommand must complete in under 100ms to avoid perceptible delay. SQLite queries on the small task database should be well within this budget.
- **Testing:** Unit tests should cover the hook subcommands (especially `hooks snapshot` output and identity-aware filtering). Since hooks are Go subcommands, they can be tested with standard Go test patterns using stdin/stdout pipes.
