package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	watchHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	watchDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	watchDoneStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
)

// tickMsg signals that the poll interval has elapsed and the task should be refreshed.
type tickMsg time.Time

// watchModel is the bubbletea model for the watch command.
type watchModel struct {
	db        *DB
	workspace string
	taskID    string
	task      *Task
	interval  time.Duration
	noExit    bool
	allDone   bool
	err       error
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

func (m watchModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if m.task == nil {
		return "Loading...\n"
	}

	var b strings.Builder

	b.WriteString(watchHeaderStyle.Render("Watching: "+m.task.Title) + "\n")
	b.WriteString(watchDimStyle.Render("ID: "+m.task.ID) + "\n\n")

	b.WriteString(renderTree(m.task, "", true))

	if m.task.ProgressNotes != "" {
		b.WriteString("\n" + watchDimStyle.Render("Progress Notes:") + "\n")
		notes := m.task.ProgressNotes
		lines := strings.Split(notes, "\n")
		const maxNotes = 10
		if len(lines) > maxNotes {
			lines = lines[len(lines)-maxNotes:]
			b.WriteString(watchDimStyle.Render("  ...") + "\n")
		}
		for _, line := range lines {
			if line != "" {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	if m.allDone {
		b.WriteString("\n" + watchDoneStyle.Render("All tasks complete!") + "\n")
	} else {
		b.WriteString("\n" + watchDimStyle.Render("Press q to exit") + "\n")
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
		Use:   "watch <id>",
		Short: "Watch a task and its subtask tree",
		Long:  "Live-updating TUI that displays a task and its full subtask tree. Polls the database for changes and re-renders automatically. Exits when all tasks are done.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskInput := args[0]

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

			task, err := ResolveTaskID(db, workspace, taskInput)
			if err != nil {
				return err
			}

			m := watchModel{
				db:        db,
				workspace: workspace,
				taskID:    task.ID,
				task:      task,
				interval:  dur,
				noExit:    noExit,
				allDone:   allTasksDone(task),
			}

			p := tea.NewProgram(m)
			if _, err := p.Run(); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&interval, "interval", "5s", "poll interval")
	cmd.Flags().BoolVar(&noExit, "no-exit", false, "stay running after all tasks done")
	cmd.Flags().StringVar(&workspace, "workspace", "", "override workspace (default: cwd)")

	return cmd
}
