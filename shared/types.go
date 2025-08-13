package shared

import (
	"github.com/google/uuid"
)

type ProcessJobArgs struct {
	JobID       uuid.UUID `json:"job_id"`
	TaskID      uuid.UUID `json:"task_id"`
	UserID      uuid.UUID `json:"user_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Payload     string    `json:"payload"`
}

type IntervalJobArgs struct {
	JobID       uuid.UUID  `json:"job_id"`
	UserID      uuid.UUID  `json:"user_id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	Payload     string     `json:"payload"`
	TaskID      *uuid.UUID `json:"task_id,omitempty"` // ✅ ADD: Optional task ID for recovery
}

func (args IntervalJobArgs) Kind() string {
	return "interval_job"
}

// IAgentTask represents a task in an agent plan
type IAgentTask struct {
	Step         int      `json:"step"`
	AgentName    string   `json:"agent_name"`
	AgentAddress string   `json:"agent_address"`
	TaskID       string   `json:"task_id"`
	Task         string   `json:"task"`
	Dependencies []string `json:"dependencies"`
}

// SSEMessage represents a Server-Sent Event message
type SSEMessage struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data,omitempty"`
}

// SSEHandler defines callback function for handling SSE messages
type SSEHandler func(message SSEMessage) error

// AIAgentResponse represents the response from AI agent
type AIAgentResponse struct {
	AgentName string `json:"agent_name"`
	TaskID    string `json:"task_id"`
	Content   string `json:"content"`
}

type ClientAgentRequest struct {
	Message string `json:"message"`
}
