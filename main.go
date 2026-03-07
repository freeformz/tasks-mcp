package main

import (
	"context"
	"encoding/json"
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
		pendingCmd(),
		checkActiveCmd(),
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

func pendingCmd() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:    "pending",
		Short:  "Print pending tasks (used by hooks)",
		Hidden: true,
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				var err error
				workspace, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			tasks, err := db.PendingSummary(workspace)
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				return nil
			}

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

			return nil
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace path")

	return cmd
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

func checkActiveCmd() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:    "check-active",
		Short:  "Check for active tasks (used by hooks)",
		Hidden: true,
		SilenceUsage: true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace == "" {
				var err error
				workspace, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			hasActive, err := db.HasActiveTasks(workspace)
			if err != nil {
				return err
			}

			if hasActive {
				tasks, err := db.ListTasks(workspace, ListFilter{Status: string(StatusInProgress)})
				if err != nil {
					return err
				}

				result := map[string]any{
					"decision": "block",
					"reason":   formatActiveTasksReminder(tasks),
				}
				json.NewEncoder(os.Stdout).Encode(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace path")

	return cmd
}

func formatActiveTasksReminder(tasks []Task) string {
	var b strings.Builder
	b.WriteString("You have in-progress tasks that should be updated before ending the session:\n")
	for _, t := range tasks {
		fmt.Fprintf(&b, "- %s (id: %s)\n", t.Title, t.ID)
	}
	b.WriteString("\nPlease update these tasks with progress notes and appropriate status (done, blocked, or todo) before stopping.")
	return b.String()
}
