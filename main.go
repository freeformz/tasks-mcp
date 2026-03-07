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
		}
	}

	runServer()
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

	fmt.Println("## Pending Tasks")
	fmt.Println()
	for _, t := range tasks {
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
		if len(t.Subtasks) > 0 {
			for _, st := range t.Subtasks {
				fmt.Printf("  - [%s] %s (id: %s)\n", st.Status, st.Title, st.ID)
			}
		}
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
