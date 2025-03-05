package agent_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"distributed-calculator/internal/agent"
)

func TestAgent_ProcessTasks(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name               string
		mockOrchestrator   func() *httptest.Server
		initialTask        *agent.Task
		expectedTaskStatus string
		expectedTaskResult float64
		expectedError      bool
	}{
		{
			name: "Successful task processing",
			mockOrchestrator: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/internal/task/result":
						// Check result
						var req struct {
							ID     string  `json:"id"`
							Result float64 `json:"result"`
						}
						if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
							t.Errorf("Error decoding request: %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						if req.ID != "1" || req.Result != 4 {
							t.Errorf("Unexpected task result: got %+v, want ID=1, Result=4", req)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						w.WriteHeader(http.StatusOK)
					case "/internal/task/status":
						// Check status update
						var req struct {
							ID     string `json:"id"`
							Status string `json:"status"`
						}
						if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
							t.Errorf("Error decoding request: %v", err)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						if req.ID != "1" {
							t.Errorf("Unexpected task ID: got %s, want 1", req.ID)
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						w.WriteHeader(http.StatusOK)

					default:
						t.Errorf("Unexpected request to %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			initialTask:        &agent.Task{ID: "1", Expression: "2+2", Status: agent.StatusNew, Result: 0}, // Set initial task
			expectedTaskStatus: agent.StatusCompleted,
			expectedTaskResult: 4,
			expectedError:      false,
		},
		{
			name: "Failed to process task",
			mockOrchestrator: func() *httptest.Server {
				// Mock orchestrator that always fails
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError) // Simulate failure
				}))
			},
			initialTask:   &agent.Task{ID: "2", Expression: "2+2", Status: agent.StatusNew, Result: 0}, // Set initial task
			expectedError: true,
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize mock orchestrator
			mockOrchestrator := tc.mockOrchestrator()
			defer mockOrchestrator.Close()

			// You can add more assertions here to check task status etc. if needed
		})
	}
}
