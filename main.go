package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
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
	rootCmd := &cobra.Command{
		Use:   "tasks-mcp",
		Short: "Task management MCP server for AI agents",
		Long:  "tasks-mcp is a task management MCP server designed for AI coding agents. It provides persistent, workspace-scoped task tracking across sessions.",
	}

	rootCmd.AddCommand(
		mcpCmd(),
		listCmd(),
		watchCmd(),
		closeCmd(),
		hooksCmd(),
	)

	if err := fang.Execute(context.Background(), rootCmd,
		fang.WithVersion(version),
	); err != nil {
		os.Exit(1)
	}
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio)",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := os.Getwd()
			if err != nil {
				return err
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			srv := NewServer(db, workspace)
			return server.ServeStdio(srv)
		},
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

func formatActiveTasksReminder(tasks []Task) string {
	var b strings.Builder
	b.WriteString("You have in-progress tasks. If the session is ending, add notes via task_add_note and set an appropriate status (done, blocked, or todo).\n")
	b.WriteString("If you are still actively working with the user, ignore this reminder and continue.\n")
	b.WriteString("Do NOT delete tasks in response to this reminder.\n\n")
	for _, t := range tasks {
		fmt.Fprintf(&b, "- %s (id: %s)\n", t.Title, t.ID)
	}
	return b.String()
}
