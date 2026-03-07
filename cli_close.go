package main

import (
	"fmt"

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

			task, err := ResolveTaskID(db, workspace, input)
			if err != nil {
				return err
			}

			if err := validateDependencies(db, workspace, task.ID, "done"); err != nil {
				return err
			}

			notes := appendProgressNote(task.ProgressNotes, formatProgressNote("Closed manually via CLI"))

			if note != "" {
				notes = appendProgressNote(notes, formatProgressNote(note))
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
