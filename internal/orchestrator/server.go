package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	storage     *SQLiteStorage
	secretKey   []byte
	userStorage UserStorage
}

func NewServer() (*Server, error) {
	storage, err := NewSQLiteStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %v", err)
	}

	return &Server{
		storage:     storage,
		secretKey:   []byte("your-secret-key"),
		userStorage: NewUserStorage(),
	}, nil
}

func (s *Server) Start() {
	http.HandleFunc("/api/v1/register", s.handleRegister)
	http.HandleFunc("/api/v1/login", s.handleLogin)

	http.HandleFunc("/api/v1/calculate", s.authMiddleware(s.handleCalculate))
	http.HandleFunc("/api/v1/expressions", s.authMiddleware(s.handleGetExpressions))
	http.HandleFunc("/api/v1/expressions/", s.authMiddleware(s.handleGetExpressionByID))

	http.HandleFunc("/internal/task", s.handleGetTask)
	http.HandleFunc("/internal/task/", s.handleUpdateTask)
	http.HandleFunc("/internal/task/result", s.handlePostTaskResult)
	http.HandleFunc("/internal/task/status", s.handlePostTaskStatus)
	http.HandleFunc("/internal/task/complete", s.handleCompleteTask)
	http.HandleFunc("/internal/task/fail", s.handleFailTask)

	log.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return s.secretKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	if _, err := s.userStorage.GetUser(req.Login); err == nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	user := &User{
		Login:        req.Login,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}

	if err := s.userStorage.CreateUser(user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	user, err := s.userStorage.GetUser(req.Login)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.Login,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	response := struct {
		Token string `json:"token"`
	}{
		Token: tokenString,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string  `json:"id"`
		Result float64 `json:"result,omitempty"`
		Status string  `json:"status,omitempty"`
		Error  string  `json:"error,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	task, err := s.storage.GetTaskByID(req.ID, "")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if req.Result != 0 {
		task.Result = req.Result
	}
	if req.Status != "" {
		task.Status = req.Status
	}
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.storage.UpdateTask(task); err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userLogin, err := s.getUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	result, err := evaluateExpression(req.Expression)
	if err != nil {
		http.Error(w, fmt.Sprintf("Calculation error: %v", err), http.StatusBadRequest)
		return
	}

	task := Task{
		ID:         GenerateTaskID(),
		Expression: req.Expression,
		Status:     StatusCompleted,
		Result:     result,
		UserLogin:  userLogin,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.storage.AddTask(&task); err != nil {
		http.Error(w, "Failed to save task", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"id": task.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func evaluateExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	if len(expr) == 0 {
		return 0, fmt.Errorf("empty expression")
	}

	expression, err := govaluate.NewEvaluableExpression(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %v", err)
	}

	result, err := expression.Evaluate(nil)
	if err != nil {
		return 0, fmt.Errorf("calculation error: %v", err)
	}

	switch v := result.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("unexpected result type")
	}
}

func (s *Server) getUserFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims["sub"].(string), nil
	}

	return "", fmt.Errorf("invalid token")
}

func (s *Server) handleGetExpressions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userLogin, err := s.getUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tasks, err := s.storage.GetUserTasks(ctx, userLogin)
	if err != nil {
		log.Printf("Failed to get user tasks: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := struct {
		Expressions []*Task `json:"expressions"`
	}{
		Expressions: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (s *Server) handleGetExpressionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userLogin, err := s.getUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	expressionID := strings.TrimPrefix(r.URL.Path, "/api/v1/expressions/")

	user_login, err := s.getUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	task, err := s.storage.GetTaskByID(expressionID, user_login)
	if err != nil {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	if task.UserLogin != userLogin {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	response := struct {
		Expression *Task `json:"expression"`
	}{
		Expression: task,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	task, erro := s.storage.GetPendingTask()
	if erro != nil {
		http.Error(w, "No tasks available", http.StatusNotFound)
		return
	}

	task.Status = StatusInProgress
	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task status to 'In Progress' for task %s: %v", task.ID, err)
		http.Error(w, "Failed to update task status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]*Task{"task": task})
}

func (s *Server) handleCompleteTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	task, err := s.storage.GetTaskByID(req.ID, "")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Result = req.Result
	task.Status = StatusCompleted
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.storage.UpdateTask(task); err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleFailTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	task, err := s.storage.GetTaskByID(req.ID, "")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Status = StatusFailed
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := s.storage.UpdateTask(task); err != nil {
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

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
	user_login, _ := s.getUserFromRequest(r)

	task, _ := s.storage.GetTaskByID(req.ID, user_login)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Result = req.Result
	task.Status = StatusCompleted

	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task %s with result: %v", task.ID, err)
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

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
	user_login, _ := s.getUserFromRequest(r)

	task, _ := s.storage.GetTaskByID(req.ID, user_login)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Status = req.Status

	err := s.storage.UpdateTask(task)
	if err != nil {
		log.Printf("Failed to update task %s with status %s: %v", task.ID, req.Status, err)
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type User struct {
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

type UserStorage interface {
	CreateUser(user *User) error
	GetUser(login string) (*User, error)
}

type InMemoryUserStorage struct {
	users map[string]*User
}

func NewUserStorage() UserStorage {
	return &InMemoryUserStorage{
		users: make(map[string]*User),
	}
}

func (s *InMemoryUserStorage) CreateUser(user *User) error {
	s.users[user.Login] = user
	return nil
}

func (s *InMemoryUserStorage) GetUser(login string) (*User, error) {
	user, exists := s.users[login]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func GenerateTaskID() string {
	return uuid.New().String()
}
