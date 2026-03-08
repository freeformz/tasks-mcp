## Task Management (tasks-mcp)

You have access to a task management MCP server for tracking multi-step work across sessions.

### When to create tasks

- When starting work that involves multiple steps or files
- When the user describes a feature, bug fix, or project that won't be done in one shot
- When resuming a session, check existing tasks before creating duplicates
- Break complex work into subtasks with parent_id for better tracking

### How to use tasks

- **Start of session**: Call `task_list` to see pending work. The SessionStart hook may have already shown pending tasks in context.
- **Starting work**: Create tasks with `task_create` or update existing ones to `in_progress`
- **Making progress**: Use `task_update` with `progress_note` to log what was done
- **Blocked**: Set status to `blocked` with a progress note explaining why
- **Completing work**: Set status to `done` with a final progress note
- **Dependencies**: Use `depends_on` when creating tasks that require other tasks to finish first. You cannot start or complete a task until all its dependencies are done.

### Agent teams

When working as part of an agent team:

- **Assign tasks** using the `assignee` field with the agent/team member name
- **Filter by assignee** using `task_list` with the `assignee` parameter to see your own tasks
- **Claim tasks** by updating the assignee to your name before starting work
- **Check team workload** by listing all tasks without an assignee filter to see the full picture
- Unassigned tasks are available for any team member to pick up

### Task fields

- **status**: `todo`, `in_progress`, `done`, `blocked`
- **priority**: `low`, `medium`, `high`, `critical`
- **assignee**: Agent or team member name (free-form string)
- **tags**: Comma-separated labels for categorization (e.g., "bug,frontend,urgent")
- **depends_on**: Comma-separated task IDs that must complete before this task
- **parent_id**: Create subtasks by referencing a parent task
- **progress_note**: Timestamped notes appended to the task log

### Dependency enforcement

- You **cannot** set a task to `in_progress` or `done` if its dependencies are not all `done`
- If blocked by dependencies, the error message will list which dependencies are incomplete
- Either complete the dependencies first, or remove them with `remove_dependencies`

### Stop hook behavior

The Stop hook fires whenever you finish responding, not only when the session ends. It reminds you about in-progress tasks.

- **Do NOT delete tasks** in response to the stop hook — it is a reminder, not an instruction to clean up
- If the user is still actively chatting, simply acknowledge the reminder and continue working
- Only update task status (to `done`, `blocked`, or `todo` with a progress note) when the session is genuinely ending and you are done with the work

### Best practices

- Keep task titles short and descriptive
- Use progress notes to leave a trail for future sessions
- Update task status before ending a session
- Use tags consistently across tasks (e.g., "bug", "feature", "refactor", "test")
- Don't create tasks for trivial one-off operations
- Assign tasks to yourself when working in a team to avoid duplicate work
