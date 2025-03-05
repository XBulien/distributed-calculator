package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings" // Import для обработки URL path
)

type Server struct {
	storage *Storage
}

func NewServer() *Server {
	return &Server{
		storage: NewStorage(),
	}
}

func (s *Server) Start() {
	http.HandleFunc("/api/v1/calculate", s.handleCalculate)
	http.HandleFunc("/api/v1/expressions", s.handleGetExpressions)
	http.HandleFunc("/api/v1/expressions/", s.handleGetExpressionByID) // Важно: слеш в конце!

	// Для внутренних задач
	http.HandleFunc("/internal/task", s.handleGetTask)
	http.HandleFunc("/internal/task/result", s.handlePostTaskResult) // Убрал слеш в конце, так как не используется
	http.HandleFunc("/internal/task/status", s.handlePostTaskStatus) // Добавил обработчик для обновления статуса

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	// Создаем задачу
	task := CreateTask(req.Expression)

	// Сохраняем задачу
	s.storage.AddTask(&task)

	response := map[string]string{
		"id": task.ID, // Возвращаем ID созданной задачи
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetExpressions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tasks := s.storage.GetAllTasks()
	response := struct {
		Expressions []*Task `json:"expressions"`
	}{
		Expressions: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetExpressionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	expressionID := strings.TrimPrefix(r.URL.Path, "/api/v1/expressions/") // Используем TrimPrefix

	tasks := s.storage.GetTasksByExpressionID(expressionID)
	if len(tasks) == 0 {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	response := struct {
		Expression []*Task `json:"expression"`
	}{
		Expression: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetTask возвращает задачу для агента.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	task := s.storage.GetPendingTask()
	if task == nil {
		http.Error(w, "No tasks available", http.StatusNotFound)
		return
	}

	// Устанавливаем статус задачи в "In Progress"
	task.Status = StatusInProgress // Используем константу
	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task status to 'In Progress' for task %s: %v", task.ID, err)
		http.Error(w, "Failed to update task status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]*Task{"task": task})
}

// handlePostTaskResult обрабатывает результат задачи от агента.
func (s *Server) handlePostTaskResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем задачу по ID
	task := s.storage.GetTaskByID(req.ID)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Записываем результат в задачу
	task.Result = req.Result
	task.Status = StatusCompleted // Используем константу

	// Обновляем задачу в хранилище
	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task %s with result: %v", task.ID, err)
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handlePostTaskStatus обрабатывает обновление статуса задачи от агента.
func (s *Server) handlePostTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем задачу по ID
	task := s.storage.GetTaskByID(req.ID)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Обновляем статус задачи
	task.Status = req.Status

	// Обновляем задачу в хранилище
	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task %s with status %s: %v", task.ID, req.Status, err)
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
