package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func closeCmd() *cobra.Command {
	var (
		note      string
		workspace string
	)

	cmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Mark a task as done",
		Long:  "Marks a task as done from the command line. Appends a progress note and enforces dependency completion. Accepts a short ID suffix or full UUID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			if workspace == "" {
				var err error
				workspace, err = cmd.Flags().GetString("workspace")
				if err != nil || workspace == "" {
					wd, wdErr := getWorkingDir()
					if wdErr != nil {
						return wdErr
					}
					workspace = wd
				}
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			task, err := ResolveTaskID(db, workspace, input)
			if err != nil {
				return err
			}

			incomplete, err := db.CheckDependencies(workspace, task.ID)
			if err != nil {
				return err
			}
			if len(incomplete) > 0 {
				return fmt.Errorf("%s", formatDependencyError("done", incomplete))
			}

			timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
			closedNote := fmt.Sprintf("[%s] Closed manually via CLI", timestamp)

			notes := task.ProgressNotes
			if notes != "" {
				notes += "\n" + closedNote
			} else {
				notes = closedNote
			}

			if note != "" {
				noteTimestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
				entry := fmt.Sprintf("[%s] %s", noteTimestamp, note)
				notes += "\n" + entry
			}

			updates := map[string]string{
				"status":         string(StatusDone),
				"progress_notes": notes,
			}

			if _, err := db.UpdateTask(workspace, task.ID, updates, nil, nil, nil, nil); err != nil {
				return err
			}

			fmt.Printf("Closed task: %s (%s)\n", task.Title, ShortID(task.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&note, "note", "", "add a progress note when closing")
	cmd.Flags().StringVar(&workspace, "workspace", "", "override workspace (default: cwd)")

	return cmd
}
