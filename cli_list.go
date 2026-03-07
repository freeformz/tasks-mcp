package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		interactive bool
		showSubtasks bool
		includeDone  bool
		statusFilter string
		assigneeFilter string
		workspace    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks in current workspace",
		Long:  "Static table of open tasks. Use -i for interactive TUI mode with navigation, task details, and closing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				wd, err := getWorkingDir()
				if err != nil {
					return err
				}
				workspace = wd
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			if interactive {
				return runListInteractive(db, workspace)
			}

			filter := ListFilter{
				Status:      statusFilter,
				Assignee:    assigneeFilter,
				IncludeDone: includeDone,
			}

			tasks, err := db.ListTasks(workspace, filter)
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			printTaskTable(os.Stdout, tasks, showSubtasks, db, workspace)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "interactive TUI mode")
	cmd.Flags().BoolVar(&showSubtasks, "subtasks", false, "show subtasks nested under parents")
	cmd.Flags().BoolVar(&includeDone, "include-done", false, "include completed tasks")
	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (todo, in_progress, done, blocked)")
	cmd.Flags().StringVar(&assigneeFilter, "assignee", "", "filter by assignee name")
	cmd.Flags().StringVar(&workspace, "workspace", "", "override workspace (default: cwd)")

	return cmd
}

// printTaskTable writes a formatted task table to the given writer.
func printTaskTable(out *os.File, tasks []Task, showSubtasks bool, db *DB, workspace string) {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTITLE\tASSIGNEE\tTAGS")

	for _, t := range tasks {
		printTaskRow(w, t, "")
		if showSubtasks {
			subtasks, err := db.ListTasks(workspace, ListFilter{ParentID: t.ID, IncludeDone: true})
			if err != nil {
				log.Fatal(err)
			}
			for _, st := range subtasks {
				printTaskRow(w, st, "  ")
			}
		}
	}

	w.Flush()
}

func printTaskRow(w *tabwriter.Writer, t Task, prefix string) {
	tags := strings.Join(t.Tags, ",")
	fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%s\n",
		prefix,
		ShortID(t.ID),
		StyledStatus(t.Status),
		StyledPriority(t.Priority),
		t.Title,
		t.Assignee,
		tags,
	)
}

// --- Interactive TUI mode ---

type listView int

const (
	viewList listView = iota
	viewDetail
	viewConfirmClose
)

type listModel struct {
	db        *DB
	workspace string
	tasks     []Task
	cursor    int
	view      listView
	detail    *Task
	err       error
	quitting  bool
	width     int
	height    int
}

func newListModel(db *DB, workspace string) listModel {
	return listModel{
		db:        db,
		workspace: workspace,
	}
}

type tasksLoadedMsg struct {
	tasks []Task
}

type taskDetailMsg struct {
	task *Task
}

type taskClosedMsg struct{}

type errMsg struct {
	err error
}

func (m listModel) loadTasks() tea.Msg {
	tasks, err := m.db.ListTasks(m.workspace, ListFilter{})
	if err != nil {
		return errMsg{err: err}
	}
	return tasksLoadedMsg{tasks: tasks}
}

func (m listModel) loadDetail(id string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.db.GetTask(m.workspace, id)
		if err != nil {
			return errMsg{err: err}
		}
		return taskDetailMsg{task: task}
	}
}

func (m listModel) closeTask(id string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.db.GetTask(m.workspace, id)
		if err != nil {
			return errMsg{err: err}
		}

		incomplete, err := m.db.CheckDependencies(m.workspace, id)
		if err != nil {
			return errMsg{err: err}
		}
		if len(incomplete) > 0 {
			return errMsg{err: fmt.Errorf("%s", formatDependencyError("done", incomplete))}
		}

		timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
		closedNote := fmt.Sprintf("[%s] Closed manually via CLI", timestamp)

		notes := task.ProgressNotes
		if notes != "" {
			notes += "\n" + closedNote
		} else {
			notes = closedNote
		}

		updates := map[string]string{
			"status":         string(StatusDone),
			"progress_notes": notes,
		}

		if _, err := m.db.UpdateTask(m.workspace, id, updates, nil, nil, nil, nil); err != nil {
			return errMsg{err: err}
		}
		return taskClosedMsg{}
	}
}

func (m listModel) Init() tea.Cmd {
	return m.loadTasks
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tasksLoadedMsg:
		m.tasks = msg.tasks
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		return m, nil

	case taskDetailMsg:
		m.detail = msg.task
		m.view = viewDetail
		return m, nil

	case taskClosedMsg:
		m.view = viewList
		m.detail = nil
		return m, m.loadTasks

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Clear errors on any keypress.
		m.err = nil

		switch m.view {
		case viewList:
			return m.updateList(msg)
		case viewDetail:
			return m.updateDetail(msg)
		case viewConfirmClose:
			return m.updateConfirmClose(msg)
		}
	}

	return m, nil
}

func (m listModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
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
			return m, m.loadDetail(m.tasks[m.cursor].ID)
		}
	case "c":
		if len(m.tasks) > 0 {
			m.view = viewConfirmClose
		}
	}
	return m, nil
}

func (m listModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "backspace":
		m.view = viewList
		m.detail = nil
	}
	return m, nil
}

func (m listModel) updateConfirmClose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		task := m.tasks[m.cursor]
		m.view = viewList
		return m, m.closeTask(task.ID)
	case "n", "esc":
		m.view = viewList
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	selectedBg = lipgloss.NewStyle().Background(lipgloss.Color("8"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	labelStyle = lipgloss.NewStyle().Bold(true)
)

func (m listModel) View() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case viewDetail:
		return m.viewDetail()
	case viewConfirmClose:
		return m.viewConfirmClose()
	default:
		return m.viewList()
	}
}

func (m listModel) viewList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Tasks"))
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

func (m listModel) viewDetail() string {
	if m.detail == nil {
		return "Loading..."
	}

	t := m.detail
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(t.Title))
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
		for _, st := range t.Subtasks {
			fmt.Fprintf(&b, "  %s  %s  %s\n",
				StyledStatus(st.Status),
				st.Title,
				ShortID(st.ID),
			)
		}
	}

	if t.ProgressNotes != "" {
		fmt.Fprintf(&b, "\n%s\n%s\n", labelStyle.Render("Progress Notes:"), t.ProgressNotes)
	}

	b.WriteString("\nesc/backspace: back  q: quit")

	return b.String()
}

func (m listModel) viewConfirmClose() string {
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

func runListInteractive(db *DB, workspace string) error {
	m := newListModel(db, workspace)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
