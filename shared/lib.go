package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Request structures based on the A2A protocol
type TextPart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type Message struct {
	Role     string                 `json:"role"`
	Parts    []TextPart             `json:"parts"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type TaskSendParams struct {
	ID               string                 `json:"id"`
	SessionID        string                 `json:"sessionId,omitempty"`
	Message          Message                `json:"message"`
	PushNotification interface{}            `json:"pushNotification,omitempty"`
	HistoryLength    *int                   `json:"historyLength,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type SendTaskRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      interface{}    `json:"id"`
	Method  string         `json:"method"`
	Params  TaskSendParams `json:"params"`
}

type TaskStatus struct {
	State     string   `json:"state"`
	Message   *Message `json:"message,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
}

type Task struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"sessionId,omitempty"`
	Status    TaskStatus             `json:"status"`
	Artifacts []interface{}          `json:"artifacts,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SendTaskResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  *Task         `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// AIAgentClient represents a client for calling AI Agent API
type AIAgentClient struct {
	HTTPClient  *http.Client
	BearerToken string // Optional: for authentication if needed
}

// NewAIAgentClient creates a new AI Agent client
func NewAIAgentClient() *AIAgentClient {
	return &AIAgentClient{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Minute, // 10 minutes timeout
		},
	}
}

// SetBearerToken sets the bearer token for authentication
func (c *AIAgentClient) SetBearerToken(token string) {
	c.BearerToken = token
}

// SendMessage sends a message to an AI agent and returns the final result
func (c *AIAgentClient) SendMessage(agentURL string, taskID string, userMessage string) (*Task, error) {
	url := agentURL
	// Create the request payload
	request := SendTaskRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params: TaskSendParams{
			ID: taskID,
			Message: Message{
				Role: "user",
				Parts: []TextPart{
					{
						Type: "text",
						Text: userMessage,
					},
				},
			},
		},
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.BearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.BearerToken))
	}

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var response SendTaskResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %d - %s", response.Error.Code, response.Error.Message)
	}

	// Return the task result
	return response.Result, nil
}

// SendMessageAndWaitForCompletion sends a message and polls until the task is completed
func (c *AIAgentClient) SendMessageAndWaitForCompletion(agentID, taskID, userMessage string) (*Task, error) {
	// Send initial message
	task, err := c.SendMessage(agentID, taskID, userMessage)
	if err != nil {
		return nil, err
	}

	// If task is already completed, return it
	if task.Status.State == "completed" || task.Status.State == "failed" || task.Status.State == "canceled" {
		return task, nil
	}

	// Poll for completion
	for {
		time.Sleep(2 * time.Second) // Wait 2 seconds between polls

		// Get task status
		updatedTask, err := c.GetTaskStatus(agentID, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task status: %w", err)
		}

		// Check if task is completed
		if updatedTask.Status.State == "completed" || updatedTask.Status.State == "failed" || updatedTask.Status.State == "canceled" {
			return updatedTask, nil
		}

		// Continue polling...
	}
}

// GetTaskStatus gets the current status of a task
func (c *AIAgentClient) GetTaskStatus(agentURL string, taskID string) (*Task, error) {
	url := agentURL

	// Create the request payload for getting task status
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tasks/get",
		"params": map[string]interface{}{
			"id": taskID,
		},
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.BearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.BearerToken))
	}

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d - %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var response SendTaskResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %d - %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// ExtractFinalResponse extracts the final text response from a completed task
func ExtractFinalResponse(task *Task) string {
	if task == nil || task.Status.Message == nil {
		return ""
	}

	var result string
	for _, part := range task.Status.Message.Parts {
		if part.Type == "text" {
			result += part.Text
		}
	}
	return result
}
