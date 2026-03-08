package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// errBlockStop is returned by hooksCheckActiveCmd to signal that the stop
// hook should block with exit code 2. Handled in main().
var errBlockStop = errors.New("block stop")

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

			writeSnapshot(os.Stdout, tasks, input.AgentType)
			return nil
		},
	}
}

// writeSnapshot formats and writes the task snapshot to w.
func writeSnapshot(w io.Writer, tasks []Task, agentType string) {
	// Count tasks by status and separate into categories in a single pass.
	counts := make(map[TaskStatus]int)
	var assigned, inProgress, other []Task
	for _, t := range tasks {
		counts[t.Status]++
		if agentType != "" && t.Assignee == agentType {
			assigned = append(assigned, t)
		} else if t.Status == StatusInProgress {
			inProgress = append(inProgress, t)
		} else {
			other = append(other, t)
		}
	}

	// Print assigned tasks first if agent_type is present.
	if len(assigned) > 0 {
		fmt.Fprintln(w, "## Tasks Assigned to You")
		fmt.Fprintln(w)
		for _, t := range assigned {
			printTaskTo(w, t)
		}
		fmt.Fprintln(w)
	}

	// Print status counts.
	var parts []string
	for _, s := range []TaskStatus{StatusInProgress, StatusTodo, StatusBlocked} {
		if c := counts[s]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, s))
		}
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, "Status: %s\n\n", strings.Join(parts, ", "))
	}

	// Print in-progress tasks.
	if len(inProgress) > 0 {
		fmt.Fprintln(w, "## In-Progress Tasks")
		fmt.Fprintln(w)
		for _, t := range inProgress {
			printTaskTo(w, t)
		}
		fmt.Fprintln(w)
	}

	// Print remaining tasks.
	if len(other) > 0 {
		fmt.Fprintln(w, "## Pending Tasks")
		fmt.Fprintln(w)
		for _, t := range other {
			printTaskTo(w, t)
		}
	}
}

// printTaskTo writes a task line to w (same format as printTask but to a writer).
func printTaskTo(w io.Writer, t Task) {
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
	fmt.Fprintf(w, "- [%s] %s%s%s%s (id: %s)\n", t.Status, t.Title, priority, tags, assignee, t.ID)
	for _, st := range t.Subtasks {
		fmt.Fprintf(w, "  - [%s] %s (id: %s)\n", st.Status, st.Title, st.ID)
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

			tasks, err := db.ListTasks(input.CWD, ListFilter{Status: string(StatusInProgress)})
			if err != nil {
				return err
			}

			if len(tasks) > 0 {

				result := map[string]any{
					"decision": "block",
					"reason":   formatActiveTasksReminder(tasks),
				}
				if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
					return fmt.Errorf("encode result: %w", err)
				}
				return errBlockStop
			}

			return nil
		},
	}
}
