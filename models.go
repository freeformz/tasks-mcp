package main

import (
	"errors"
	"time"
)

// ErrTaskNotFound indicates that a task could not be found.
var ErrTaskNotFound = errors.New("task not found")

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusBlocked    TaskStatus = "blocked"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone, StatusBlocked:
		return true
	}
	return false
}

type TaskPriority string

const (
	PriorityLow      TaskPriority = "low"
	PriorityMedium   TaskPriority = "medium"
	PriorityHigh     TaskPriority = "high"
	PriorityCritical TaskPriority = "critical"
)

func (p TaskPriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical:
		return true
	}
	return false
}

type Task struct {
	ID            string       `json:"id"`
	Workspace     string       `json:"workspace"`
	Title         string       `json:"title"`
	Description   string       `json:"description,omitempty"`
	Status        TaskStatus   `json:"status"`
	Priority      TaskPriority `json:"priority"`
	Assignee      string       `json:"assignee,omitempty"`
	ParentID      string       `json:"parent_id,omitempty"`
	ProgressNotes string       `json:"progress_notes,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	Tags          []string     `json:"tags,omitempty"`
	DependsOn     []string     `json:"depends_on,omitempty"`
	Subtasks      []Task       `json:"subtasks,omitempty"`
}

type ListFilter struct {
	Status        string
	Tag           string
	ParentID      string
	Assignee      string
	IncludeDone   bool
	AllWorkspaces bool
}

type AgentPresence struct {
	ID            string    `json:"id"`
	Workspace     string    `json:"workspace"`
	AgentName     string    `json:"agent_name"`
	SessionID     string    `json:"session_id"`
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}
