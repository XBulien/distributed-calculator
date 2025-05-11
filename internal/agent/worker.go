package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Knetic/govaluate"
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
	StatusNew        = "new"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Agent struct {
	NumWorkers          int
	OrchestratorAddress string
	HTTPClient          *http.Client
}

func NewAgent(numWorkers int, orchestratorAddress string) *Agent {
	return &Agent{
		NumWorkers:          numWorkers,
		OrchestratorAddress: orchestratorAddress,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *Agent) Start() {
	for i := 0; i < a.NumWorkers; i++ {
		go a.worker()
	}
}

func (a *Agent) worker() {
	for {
		task, err := a.getTask()
		if err != nil {
			log.Printf("Error getting task: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if task == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		result, err := a.calculate(task.Expression)
		if err != nil {
			log.Printf("Calculation failed: %v", err)
			if err := a.saveTaskResult(task, 0, StatusFailed, err.Error()); err != nil {
				log.Printf("Failed to save failed task: %v", err)
			}
			continue
		}

		if err := a.saveTaskResult(task, result, StatusCompleted, ""); err != nil {
			log.Printf("Failed to save completed task: %v", err)
		}
	}
}

func (a *Agent) saveCompletedTask(task *Task, result float64) error {
	payload := struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}{
		ID:     task.ID,
		Result: result,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		a.OrchestratorAddress+"/internal/task",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *Agent) saveFailedTask(task *Task, errorMsg string) error {
	payload := struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}{
		ID:     task.ID,
		Status: StatusFailed,
		Error:  errorMsg,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	resp, err := a.HTTPClient.Post(
		a.OrchestratorAddress+"/internal/task/fail",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func (a *Agent) getTask() (*Task, error) {
	resp, err := a.HTTPClient.Get(a.OrchestratorAddress + "/internal/task")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response struct {
		Task *Task `json:"task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return response.Task, nil
}

func (a *Agent) updateTaskStatus(taskID, status string) error {
	payload := struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		ID:     taskID,
		Status: status,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	resp, err := a.HTTPClient.Post(
		a.OrchestratorAddress+"/internal/task/status",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (a *Agent) sendResult(taskID string, result float64) error {
	payload := struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}{
		ID:     taskID,
		Result: result,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	resp, err := a.HTTPClient.Post(
		a.OrchestratorAddress+"/internal/task/result",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (a *Agent) calculate(expression string) (float64, error) {
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return 0, fmt.Errorf("expression parse failed: %w", err)
	}

	result, err := expr.Evaluate(nil)
	if err != nil {
		return 0, fmt.Errorf("evaluation failed: %w", err)
	}

	switch v := result.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}
}

func (a *Agent) saveTaskResult(task *Task, result float64, status string, errorMsg string) error {
	payload := struct {
		ID     string  `json:"id"`
		Result float64 `json:"result,omitempty"`
		Status string  `json:"status"`
		Error  string  `json:"error,omitempty"`
	}{
		ID:     task.ID,
		Result: result,
		Status: status,
		Error:  errorMsg,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		a.OrchestratorAddress+"/internal/task",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
