package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func runClose() {
	if len(os.Args) < 3 {
		log.Fatal("usage: tasks-mcp close <id> [--note <note>] [--workspace <path>]")
	}

	input := os.Args[2]
	note := flagValue("--note")
	workspace := cliWorkspace()

	db, err := OpenDB(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	task, err := ResolveTaskID(db, workspace, input)
	if err != nil {
		log.Fatal(err)
	}

	// Dependency enforcement.
	incomplete, err := db.CheckDependencies(workspace, task.ID)
	if err != nil {
		log.Fatal(err)
	}
	if len(incomplete) > 0 {
		log.Fatal(formatDependencyError("done", incomplete))
	}

	// Build progress notes.
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
		log.Fatal(err)
	}

	fmt.Printf("Closed task: %s (%s)\n", task.Title, ShortID(task.ID))
}
