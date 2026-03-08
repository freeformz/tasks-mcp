package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// hookInput represents the JSON passed to hooks on stdin by Claude Code.
type hookInput struct {
	CWD            string `json:"cwd"`
	AgentType      string `json:"agent_type"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// readHookInput reads and parses hook JSON from stdin.
func readHookInput() (hookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return hookInput{}, fmt.Errorf("read stdin: %w", err)
	}
	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return hookInput{}, fmt.Errorf("parse hook input: %w", err)
	}
	return input, nil
}

func hooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hooks",
		Short:  "Hook subcommands invoked by Claude Code",
		Hidden: true,
	}

	cmd.AddCommand(
		hooksSnapshotCmd(),
		hooksCheckActiveCmd(),
	)

	return cmd
}

func hooksSnapshotCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "snapshot",
		Short:         "Output task state for SessionStart hook",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readHookInput()
			if err != nil {
				return err
			}

			if input.CWD == "" {
				return nil
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			tasks, err := db.ListTasks(input.CWD, ListFilter{})
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				return nil
			}

			// Count tasks by status.
			counts := make(map[TaskStatus]int)
			for _, t := range tasks {
				counts[t.Status]++
			}

			// Separate assigned tasks (if agent_type is present) from the rest.
			var assigned, inProgress, other []Task
			for _, t := range tasks {
				if input.AgentType != "" && t.Assignee == input.AgentType {
					assigned = append(assigned, t)
				} else if t.Status == StatusInProgress {
					inProgress = append(inProgress, t)
				} else {
					other = append(other, t)
				}
			}

			// Print assigned tasks first if agent_type is present.
			if len(assigned) > 0 {
				fmt.Println("## Tasks Assigned to You")
				fmt.Println()
				for _, t := range assigned {
					printTask(t)
				}
				fmt.Println()
			}

			// Print status counts.
			var parts []string
			for _, s := range []TaskStatus{StatusInProgress, StatusTodo, StatusBlocked} {
				if c := counts[s]; c > 0 {
					parts = append(parts, fmt.Sprintf("%d %s", c, s))
				}
			}
			if len(parts) > 0 {
				fmt.Printf("Status: %s\n\n", strings.Join(parts, ", "))
			}

			// Print in-progress tasks.
			if len(inProgress) > 0 {
				fmt.Println("## In-Progress Tasks")
				fmt.Println()
				for _, t := range inProgress {
					printTask(t)
				}
				fmt.Println()
			}

			// Print remaining tasks.
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
}

func hooksCheckActiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "check-active",
		Short:         "Check for active tasks (Stop hook)",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readHookInput()
			if err != nil {
				return err
			}

			// Re-fire detection: if the stop hook already fired once, let Claude stop.
			if input.StopHookActive {
				fmt.Println(`{"decision":"approve"}`)
				return nil
			}

			if input.CWD == "" {
				return nil
			}

			db, err := OpenDB(dbPath())
			if err != nil {
				return err
			}
			defer db.Close()

			hasActive, err := db.HasActiveTasks(input.CWD)
			if err != nil {
				return err
			}

			if hasActive {
				tasks, err := db.ListTasks(input.CWD, ListFilter{Status: string(StatusInProgress)})
				if err != nil {
					return err
				}

				result := map[string]any{
					"decision": "block",
					"reason":   formatActiveTasksReminder(tasks),
				}
				if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
					return fmt.Errorf("encode result: %w", err)
				}
				os.Exit(2)
			}

			return nil
		},
	}
}
