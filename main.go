package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

var version = "dev"

func dbPath() string {
	if p := os.Getenv("TASKS_MCP_DB_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(home, ".local", "share", "tasks-mcp", "tasks.db")
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "pending":
			runPending()
			return
		case "check-active":
			runCheckActive()
			return
		case "list":
			runList()
			return
		case "watch":
			runWatch()
			return
		case "close":
			runClose()
			return
		case "-h", "--help", "help":
			printUsage()
			return
		case "--version", "version":
			fmt.Println("tasks-mcp", version)
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}

	runServer()
}

func printUsage() {
	fmt.Printf(`tasks-mcp %s — Task management MCP server for AI agents

Usage:
  tasks-mcp                          Start MCP server (stdio)
  tasks-mcp list [flags]             List tasks in current workspace
  tasks-mcp watch <id> [flags]       Watch a task and its subtask tree
  tasks-mcp close <id> [flags]       Mark a task as done

List flags:
  -i                Interactive TUI mode
  --subtasks        Show subtasks nested under parents
  --status <s>      Filter by status (todo, in_progress, done, blocked)
  --assignee <name> Filter by assignee
  --include-done    Include completed tasks
  --workspace <p>   Override workspace (default: cwd)

Watch flags:
  --interval <dur>  Poll interval (default: 5s)
  --no-exit         Stay running after all tasks done
  --workspace <p>   Override workspace (default: cwd)

Close flags:
  --note <text>     Add a progress note when closing
  --workspace <p>   Override workspace (default: cwd)

Hook subcommands (used by Claude Code hooks):
  tasks-mcp pending --workspace <path>
  tasks-mcp check-active --workspace <path>

Options:
  -h, --help        Show this help
  --version         Show version
`, version)
}

func runServer() {
	workspace, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	db, err := OpenDB(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	srv := NewServer(db, workspace)
	if err := server.ServeStdio(srv); err != nil {
		log.Fatal(err)
	}
}

func runPending() {
	workspace := flagValue("--workspace")
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}

	db, err := OpenDB(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tasks, err := db.PendingSummary(workspace)
	if err != nil {
		log.Fatal(err)
	}

	if len(tasks) == 0 {
		return
	}

	// Separate in-progress tasks for prominent display.
	var inProgress, other []Task
	for _, t := range tasks {
		if t.Status == StatusInProgress {
			inProgress = append(inProgress, t)
		} else {
			other = append(other, t)
		}
	}

	if len(inProgress) > 0 {
		fmt.Println("## In-Progress Tasks (update or complete these first)")
		fmt.Println()
		for _, t := range inProgress {
			printTask(t)
		}
		fmt.Println()
	}

	if len(other) > 0 {
		fmt.Println("## Pending Tasks")
		fmt.Println()
		for _, t := range other {
			printTask(t)
		}
	}
}

func printTask(t Task) {
	priority := ""
	if t.Priority == PriorityHigh || t.Priority == PriorityCritical {
		priority = fmt.Sprintf(" [%s]", strings.ToUpper(string(t.Priority)))
	}
	tags := ""
	if len(t.Tags) > 0 {
		tags = fmt.Sprintf(" (%s)", strings.Join(t.Tags, ", "))
	}
	assignee := ""
	if t.Assignee != "" {
		assignee = fmt.Sprintf(" @%s", t.Assignee)
	}
	fmt.Printf("- [%s] %s%s%s%s (id: %s)\n", t.Status, t.Title, priority, tags, assignee, t.ID)
	for _, st := range t.Subtasks {
		fmt.Printf("  - [%s] %s (id: %s)\n", st.Status, st.Title, st.ID)
	}
}

func runCheckActive() {
	workspace := flagValue("--workspace")
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}

	db, err := OpenDB(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	hasActive, err := db.HasActiveTasks(workspace)
	if err != nil {
		log.Fatal(err)
	}

	if hasActive {
		tasks, err := db.ListTasks(workspace, ListFilter{Status: string(StatusInProgress)})
		if err != nil {
			log.Fatal(err)
		}

		// Output JSON for the hook to use.
		result := map[string]any{
			"decision": "block",
			"reason":   formatActiveTasksReminder(tasks),
		}
		json.NewEncoder(os.Stdout).Encode(result)
	}
}

func formatActiveTasksReminder(tasks []Task) string {
	var b strings.Builder
	b.WriteString("You have in-progress tasks that should be updated before ending the session:\n")
	for _, t := range tasks {
		b.WriteString(fmt.Sprintf("- %s (id: %s)\n", t.Title, t.ID))
	}
	b.WriteString("\nPlease update these tasks with progress notes and appropriate status (done, blocked, or todo) before stopping.")
	return b.String()
}

func flagValue(name string) string {
	return flagValueFrom(os.Args, name)
}

func flagValueFrom(args []string, name string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
