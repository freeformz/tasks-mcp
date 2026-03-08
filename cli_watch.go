package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	watchHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	watchDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	watchDoneStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	watchWarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	selectedBg        = lipgloss.NewStyle().Background(lipgloss.Color("8"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	labelStyle        = lipgloss.NewStyle().Bold(true)
)

// tickMsg signals that the poll interval has elapsed and data should be refreshed.
type tickMsg time.Time

// watchView represents the current view state in list mode.
type watchView int

const (
	viewList watchView = iota
	viewDetail
	viewConfirmClose
	viewConfirmCloseSubtask
)

// taskDetailMsg is sent when a single task's details have been loaded.
type taskDetailMsg struct {
	task *Task
}

// taskClosedMsg is sent when a task has been successfully closed.
type taskClosedMsg struct{}

// subtaskClosedMsg is sent when a subtask has been successfully closed.
type subtaskClosedMsg struct{}

// errMsg is sent when an error occurs during an async operation.
type errMsg struct {
	err error
}

// watchModel is the bubbletea model for the watch command.
// It supports two modes:
//   - List mode (no task ID): interactive task list with navigation, details, and closing.
//   - Single-task mode (task ID provided): live-updating single task tree.
type watchModel struct {
	db               *DB
	workspace        string
	interval         time.Duration
	noExit           bool
	workspaceWarning string

	// Single-task mode fields
	taskID  string
	task    *Task
	allDone bool

	// List mode fields
	listMode      bool
	tasks         []Task
	cursor        int
	subtaskCursor int
	view          watchView
	detail        *Task
	quitting      bool
	width         int
	height        int

	err error
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// allTasksDone returns true if the root task and all its subtasks are done.
func allTasksDone(t *Task) bool {
	if t.Status != StatusDone {
		return false
	}
	for i := range t.Subtasks {
		if !allTasksDone(&t.Subtasks[i]) {
			return false
		}
	}
	return true
}

func (m watchModel) Init() tea.Cmd {
	return tickCmd(0) // fire immediately
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.listMode {
		return m.updateListMode(msg)
	}
	return m.updateSingleMode(msg)
}

// updateSingleMode handles updates for single-task watch mode.
func (m watchModel) updateSingleMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tickMsg:
		task, err := m.db.GetTask(m.workspace, m.taskID)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.task = task
		m.allDone = allTasksDone(task)

		if m.allDone && !m.noExit {
			return m, tea.Quit
		}
		return m, tickCmd(m.interval)
	}

	return m, nil
}

// updateListMode handles updates for interactive list mode.
func (m watchModel) updateListMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		tasks, err := m.db.ListTasks(m.workspace, ListFilter{})
		if err != nil {
			m.err = err
			return m, tickCmd(m.interval)
		}
		m.tasks = tasks
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		// If we're viewing detail, refresh it too.
		if m.view == viewDetail && m.detail != nil {
			task, err := m.db.GetTask(m.workspace, m.detail.ID)
			if err == nil {
				m.detail = task
				if m.subtaskCursor >= len(task.Subtasks) {
					m.subtaskCursor = max(0, len(task.Subtasks)-1)
				}
			}
		}
		return m, tickCmd(m.interval)

	case taskDetailMsg:
		m.detail = msg.task
		m.view = viewDetail
		// Clamp subtask cursor if the subtask list shrank (e.g., after a close).
		if m.subtaskCursor >= len(msg.task.Subtasks) {
			m.subtaskCursor = max(0, len(msg.task.Subtasks)-1)
		}
		return m, nil

	case taskClosedMsg:
		m.view = viewList
		m.detail = nil
		// Refresh immediately so the closed task disappears.
		return m, tickCmd(0)

	case subtaskClosedMsg:
		// Stay in detail view, refresh the detail to show updated subtask status.
		if m.detail != nil {
			return m, m.loadDetail(m.detail.ID)
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Clear errors on any keypress.
		m.err = nil

		switch m.view {
		case viewList:
			return m.updateListView(msg)
		case viewDetail:
			return m.updateDetailView(msg)
		case viewConfirmClose:
			return m.updateConfirmCloseView(msg)
		case viewConfirmCloseSubtask:
			return m.updateConfirmCloseSubtaskView(msg)
		}
	}

	return m, nil
}

func (m watchModel) updateListView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	}
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.tasks) > 0 {
			m.subtaskCursor = 0
			return m, m.loadDetail(m.tasks[m.cursor].ID)
		}
	case "c":
		if len(m.tasks) > 0 {
			m.view = viewConfirmClose
		}
	}
	return m, nil
}

func (m watchModel) updateDetailView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	}
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "backspace":
		m.view = viewList
		m.detail = nil
		m.subtaskCursor = 0
	case "up", "k":
		if m.detail != nil && m.subtaskCursor > 0 {
			m.subtaskCursor--
		}
	case "down", "j":
		if m.detail != nil && m.subtaskCursor < len(m.detail.Subtasks)-1 {
			m.subtaskCursor++
		}
	case "c":
		if m.detail != nil && len(m.detail.Subtasks) > 0 {
			m.view = viewConfirmCloseSubtask
		}
	}
	return m, nil
}

func (m watchModel) updateConfirmCloseView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	}
	switch msg.String() {
	case "y":
		if len(m.tasks) == 0 || m.cursor < 0 || m.cursor >= len(m.tasks) {
			m.view = viewList
			return m, nil
		}
		task := m.tasks[m.cursor]
		m.view = viewList
		return m, m.doCloseTask(task.ID, taskClosedMsg{})
	case "n", "esc":
		m.view = viewList
	}
	return m, nil
}

func (m watchModel) updateConfirmCloseSubtaskView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	}
	switch msg.String() {
	case "y":
		if m.detail == nil || len(m.detail.Subtasks) == 0 ||
			m.subtaskCursor < 0 || m.subtaskCursor >= len(m.detail.Subtasks) {
			m.view = viewDetail
			return m, nil
		}
		st := m.detail.Subtasks[m.subtaskCursor]
		m.view = viewDetail
		return m, m.doCloseTask(st.ID, subtaskClosedMsg{})
	case "n", "esc":
		m.view = viewDetail
	}
	return m, nil
}

func (m watchModel) loadDetail(id string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.db.GetTask(m.workspace, id)
		if err != nil {
			return errMsg{err: err}
		}
		return taskDetailMsg{task: task}
	}
}

// doCloseTask validates and closes a task, returning successMsg on success.
func (m watchModel) doCloseTask(id string, successMsg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.db.GetTask(m.workspace, id); err != nil {
			return errMsg{err: err}
		}

		if err := validateDependencies(m.db, m.workspace, id, "done"); err != nil {
			return errMsg{err: err}
		}

		if err := validateSubtasksDone(m.db, m.workspace, id); err != nil {
			return errMsg{err: err}
		}

		updates := map[string]string{
			"status": string(StatusDone),
		}

		if _, err := m.db.UpdateTask(m.workspace, id, updates, nil, nil, nil, nil); err != nil {
			return errMsg{err: err}
		}

		// Add note after successful status update to avoid misleading notes on failure.
		if _, err := m.db.AddNote(id, "Closed manually via CLI"); err != nil {
			return errMsg{err: err}
		}
		return successMsg
	}
}

func (m watchModel) View() string {
	if m.listMode {
		return m.viewListMode()
	}
	return m.viewSingleMode()
}

// viewSingleMode renders the single-task watch view.
func (m watchModel) viewSingleMode() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if m.task == nil {
		return "Loading...\n"
	}

	var b strings.Builder

	b.WriteString(watchHeaderStyle.Render("Watching: "+m.task.Title) + "\n")
	b.WriteString(watchDimStyle.Render("ID: "+m.task.ID) + "\n")
	if m.workspaceWarning != "" {
		b.WriteString(watchWarningStyle.Render(m.workspaceWarning) + "\n")
	}
	b.WriteString("\n")

	b.WriteString(renderTree(m.task, "", true))

	if len(m.task.Notes) > 0 {
		b.WriteString("\n" + watchDimStyle.Render(fmt.Sprintf("Notes (%d total):", m.task.NoteCount)) + "\n")
		// Notes are newest-first; display oldest-first for reading order.
		for i := len(m.task.Notes) - 1; i >= 0; i-- {
			n := m.task.Notes[i]
			ts := n.CreatedAt.Format("2006-01-02 15:04:05")
			b.WriteString(fmt.Sprintf("  [%s] %s\n", ts, n.Content))
		}
	}

	if m.allDone {
		b.WriteString("\n" + watchDoneStyle.Render("All tasks complete!") + "\n")
	} else {
		b.WriteString("\n" + watchDimStyle.Render("Press q to exit") + "\n")
	}

	return b.String()
}

// viewListMode renders the interactive list view.
func (m watchModel) viewListMode() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case viewDetail:
		return m.renderDetail()
	case viewConfirmClose:
		return m.renderConfirmClose()
	case viewConfirmCloseSubtask:
		return m.renderConfirmCloseSubtask()
	default:
		return m.renderList()
	}
}

func (m watchModel) renderList() string {
	var b strings.Builder

	b.WriteString(watchHeaderStyle.Render("Tasks"))
	b.WriteString("\n\n")

	if len(m.tasks) == 0 {
		b.WriteString("No tasks found.\n")
	} else {
		for i, t := range m.tasks {
			line := fmt.Sprintf(" %s  %s  %s  %s",
				ShortID(t.ID),
				StyledStatus(t.Status),
				StyledPriority(t.Priority),
				t.Title,
			)
			if t.Assignee != "" {
				line += fmt.Sprintf("  @%s", t.Assignee)
			}

			if i == m.cursor {
				line = selectedBg.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n")
	}
	b.WriteString("j/k: navigate  enter: details  c: close task  q: quit")

	return b.String()
}

func (m watchModel) renderDetail() string {
	if m.detail == nil {
		return "Loading..."
	}

	t := m.detail
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", watchHeaderStyle.Render(t.Title))
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("ID:"), t.ID)
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Status:"), StyledStatus(t.Status))
	fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Priority:"), StyledPriority(t.Priority))

	if t.Assignee != "" {
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Assignee:"), t.Assignee)
	}
	if len(t.Tags) > 0 {
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("Tags:"), strings.Join(t.Tags, ", "))
	}
	if t.Description != "" {
		fmt.Fprintf(&b, "\n%s\n%s\n", labelStyle.Render("Description:"), t.Description)
	}

	if len(t.Subtasks) > 0 {
		fmt.Fprintf(&b, "\n%s\n", labelStyle.Render("Subtasks:"))
		for i, st := range t.Subtasks {
			line := fmt.Sprintf("  %s  %s  %s",
				StyledStatus(st.Status),
				st.Title,
				ShortID(st.ID),
			)
			if i == m.subtaskCursor {
				line = selectedBg.Render(line)
			}
			b.WriteString(line + "\n")
		}
	}

	if len(t.Notes) > 0 {
		fmt.Fprintf(&b, "\n%s\n", labelStyle.Render(fmt.Sprintf("Notes (%d total):", t.NoteCount)))
		// Notes are newest-first; display oldest-first for reading order.
		for i := len(t.Notes) - 1; i >= 0; i-- {
			n := t.Notes[i]
			ts := n.CreatedAt.Format("2006-01-02 15:04:05")
			fmt.Fprintf(&b, "  [%s] %s\n", ts, n.Content)
		}
	}

	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n")
	}
	help := "esc/backspace: back"
	if len(t.Subtasks) > 0 {
		help += "  j/k: navigate subtasks  c: close subtask"
	}
	help += "  q: quit"
	b.WriteString(help)

	return b.String()
}

func (m watchModel) renderConfirmClose() string {
	if len(m.tasks) == 0 {
		return ""
	}
	task := m.tasks[m.cursor]

	var b strings.Builder
	fmt.Fprintf(&b, "Close task: %s? (y/n)", task.Title)

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("Error: "+m.err.Error()))
	}

	return b.String()
}

func (m watchModel) renderConfirmCloseSubtask() string {
	if m.detail == nil || len(m.detail.Subtasks) == 0 ||
		m.subtaskCursor < 0 || m.subtaskCursor >= len(m.detail.Subtasks) {
		return m.renderDetail()
	}
	st := m.detail.Subtasks[m.subtaskCursor]

	var b strings.Builder
	fmt.Fprintf(&b, "Close subtask: %s? (y/n)", st.Title)

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("Error: "+m.err.Error()))
	}

	return b.String()
}

// renderTree renders a task and its subtasks as an indented tree.
func renderTree(t *Task, prefix string, _ bool) string {
	var b strings.Builder

	b.WriteString(renderTaskLine(t) + "\n")

	for i := range t.Subtasks {
		isLast := i == len(t.Subtasks)-1

		var connector, childIndent string
		if isLast {
			connector = prefix + "  └─ "
			childIndent = prefix + "     "
		} else {
			connector = prefix + "  ├─ "
			childIndent = prefix + "  │  "
		}

		b.WriteString(connector)
		b.WriteString(renderTree(&t.Subtasks[i], childIndent, false))
	}

	return b.String()
}

// renderTaskLine renders a single task as "[status] title (priority) @assignee".
func renderTaskLine(t *Task) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s] %s", StyledStatus(t.Status), t.Title))
	parts = append(parts, fmt.Sprintf("(%s)", StyledPriority(t.Priority)))

	if t.Assignee != "" {
		parts = append(parts, "@"+t.Assignee)
	}

	return strings.Join(parts, " ")
}

func watchCmd() *cobra.Command {
	var (
		interval  string
		noExit    bool
		workspace string
	)

	cmd := &cobra.Command{
		Use:   "watch [id]",
		Short: "Watch tasks with live updates",
		Long: `Live-updating TUI that displays tasks with automatic polling.

With no arguments, shows an interactive task list where you can navigate,
view details, and close tasks. With a task ID argument, shows that task
and its full subtask tree.

List mode controls:
  j/k or arrows: navigate tasks
  enter: view task details
  c: close selected task
  q: quit

Detail view controls:
  j/k or arrows: navigate subtasks
  c: close selected subtask
  esc/backspace: back to list
  q: quit

Single-task mode:
  Polls the database for changes and re-renders automatically.
  Exits when all tasks are done (unless --no-exit is set).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dur := 5 * time.Second
			if interval != "" {
				d, err := time.ParseDuration(interval)
				if err != nil {
					return fmt.Errorf("invalid interval %q: %w", interval, err)
				}
				dur = d
			}

			var err error
			workspace, err = resolveWorkspace(workspace)
			if err != nil {
				return err
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			if len(args) == 0 {
				// List mode: interactive task list with polling.
				m := watchModel{
					db:       db,
					workspace: workspace,
					interval: dur,
					noExit:   true, // list mode doesn't auto-exit
					listMode: true,
				}

				p := tea.NewProgram(m, tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					return fmt.Errorf("watch list mode: %w", err)
				}
				return nil
			}

			// Single-task mode.
			taskInput := args[0]

			task, warning, err := ResolveTaskIDGlobal(db, workspace, taskInput)
			if err != nil {
				return err
			}

			// Use the task's actual workspace for polling.
			taskWorkspace := workspace
			if task.Workspace != workspace {
				taskWorkspace = task.Workspace
			}

			m := watchModel{
				db:               db,
				workspace:        taskWorkspace,
				taskID:           task.ID,
				task:             task,
				interval:         dur,
				noExit:           noExit,
				allDone:          allTasksDone(task),
				workspaceWarning: warning,
			}

			p := tea.NewProgram(m)
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("watch task mode: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&interval, "interval", "5s", "poll interval")
	cmd.Flags().BoolVar(&noExit, "no-exit", false, "stay running after all tasks done")
	cmd.Flags().StringVar(&workspace, "workspace", "", "override workspace (default: cwd)")

	return cmd
}
