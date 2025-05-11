package orchestrator

import (
	"fmt"
	"strings"
	"time"
)

type Task struct {
	ID         string  `json:"id"`
	Expression string  `json:"expression"`
	Status     string  `json:"status"`
	Result     float64 `json:"result"`
	UserLogin  string  `json:"user_login"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

const (
	StatusPending    = "Pending"
	StatusNew        = "New"
	StatusInProgress = "In Progress"
	StatusCompleted  = "Completed"
	StatusFailed     = "Failed"
)

func CreateTask(expression string, userLogin string) (Task, error) {
	expression = strings.ReplaceAll(expression, " ", "")

	if expression == "" {
		return Task{}, fmt.Errorf("expression cannot be empty")
	}

	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)

	task := Task{
		ID:         GenerateTaskID(),
		Expression: expression,
		Status:     StatusPending,
		Result:     0,
		UserLogin:  userLogin,
		CreatedAt:  timestamp,
		UpdatedAt:  timestamp,
	}

	return task, nil
}
