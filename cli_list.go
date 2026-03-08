package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		showSubtasks   bool
		includeDone    bool
		statusFilter   string
		assigneeFilter string
		workspace      string
		allWorkspaces  bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks (current workspace or all with -a)",
		Long:  "Static table of open tasks. Use -a to list across all workspaces. Use 'watch' for interactive TUI mode.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			filter := ListFilter{
				Status:        statusFilter,
				Assignee:      assigneeFilter,
				IncludeDone:   includeDone,
				AllWorkspaces: allWorkspaces,
			}

			tasks, err := db.ListTasks(workspace, filter)
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			return printTaskTable(os.Stdout, tasks, showSubtasks, allWorkspaces, db)
		},
	}

	cmd.Flags().BoolVar(&showSubtasks, "subtasks", false, "show subtasks nested under parents")
	cmd.Flags().BoolVar(&includeDone, "include-done", false, "include completed tasks")
	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (todo, in_progress, done, blocked)")
	cmd.Flags().StringVar(&assigneeFilter, "assignee", "", "filter by assignee name")
	cmd.Flags().StringVar(&workspace, "workspace", "", "override workspace (default: cwd)")
	cmd.Flags().BoolVarP(&allWorkspaces, "all", "a", false, "show tasks across all workspaces")
	cmd.MarkFlagsMutuallyExclusive("all", "workspace")

	return cmd
}

// printTaskTable writes a formatted task table to the given file.
func printTaskTable(out *os.File, tasks []Task, showSubtasks, showWorkspace bool, db *DB) error {
	var shorten func(string) string
	if showWorkspace {
		shorten = newWorkspaceShortener()
	}

	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if showWorkspace {
		fmt.Fprintln(w, "WORKSPACE\tID\tSTATUS\tPRIORITY\tTITLE\tASSIGNEE\tTAGS")
	} else {
		fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTITLE\tASSIGNEE\tTAGS")
	}

	for _, t := range tasks {
		printTaskRow(w, t, "", shorten)
		if showSubtasks {
			subtaskFilter := ListFilter{ParentID: t.ID, IncludeDone: true}
			subtasks, err := db.ListTasks(t.Workspace, subtaskFilter)
			if err != nil {
				return fmt.Errorf("list subtasks: %w", err)
			}
			for _, st := range subtasks {
				printTaskRow(w, st, "  ", shorten)
			}
		}
	}

	return w.Flush()
}

func printTaskRow(w *tabwriter.Writer, t Task, prefix string, shorten func(string) string) {
	tags := strings.Join(t.Tags, ",")
	if shorten != nil {
		fmt.Fprintf(w, "%s\t%s%s\t%s\t%s\t%s\t%s\t%s\n",
			shorten(t.Workspace),
			prefix,
			ShortID(t.ID),
			StyledStatus(t.Status),
			StyledPriority(t.Priority),
			t.Title,
			t.Assignee,
			tags,
		)
	} else {
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
}
