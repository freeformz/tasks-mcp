package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
// indent is the prefix for continuation lines under the current node.
func renderTree(t *Task, prefix string, isRoot bool) string {
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

func runWatch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: tasks-mcp watch <task-id> [--interval <duration>] [--no-exit] [--workspace <path>]")
		os.Exit(1)
	}

	taskInput := os.Args[2]

	interval := 5 * time.Second
	if v := flagValue("--interval"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("invalid interval %q: %v", v, err)
		}
		interval = d
	}

	noExit := hasFlag(os.Args, "--no-exit")
	workspace := cliWorkspace()

	db, err := OpenDB(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	task, err := ResolveTaskID(db, workspace, taskInput)
	if err != nil {
		log.Fatal(err)
	}

	m := watchModel{
		db:        db,
		workspace: workspace,
		taskID:    task.ID,
		task:      task,
		interval:  interval,
		noExit:    noExit,
		allDone:   allTasksDone(task),
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

