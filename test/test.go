// Типы тестов в этом файле:
//
// 1. Интеграционные тесты (тестируют взаимодействие компонентов):
//    - TestServer_RegisterAndLogin - проверяет полный цикл регистрации и аутентификации
//    - TestServer_Calculate - тестирует обработку выражения через HTTP API
//    - TestStorage_CRUD - проверяет работу с реальной базой данных
//
// 2. Модульные тесты (unit tests, тестируют изолированные компоненты):
//    - TestAuthMiddleware - проверяет middleware авторизации с mock-запросами
//    - Частично TestStorage_CRUD - тестирование отдельных методов хранилища

package orchestrator

import (
	"bytes"
	"context"
	"distributed-calculator/internal/orchestrator"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RegisterAndLogin(t *testing.T) {
	server, err := orchestrator.NewServer()
	require.NoError(t, err)
	ts := httptest.NewServer(http.HandlerFunc(server.handleRegister))
	defer ts.Close()

	registerData := map[string]string{
		"login":    "testuser",
		"password": "testpass",
	}
	body, _ := json.Marshal(registerData)
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ts = httptest.NewServer(http.HandlerFunc(server.handleLogin))
	loginData := map[string]string{
		"login":    "testuser",
		"password": "testpass",
	}
	body, _ = json.Marshal(loginData)
	resp, err = http.Post(ts.URL, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

func TestServer_Calculate(t *testing.T) {
	server, err := orchestrator.NewServer()
	require.NoError(t, err)

	err = server.userStorage.CreateUser(&User{
		Login:        "calcuser",
		PasswordHash: "hashedpass",
		CreatedAt:    time.Now(),
	})
	require.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "calcuser",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(server.secretKey)
	require.NoError(t, err)

	reqData := map[string]string{"expression": "2+2*2"}
	body, _ := json.Marshal(reqData)

	req := httptest.NewRequest("POST", "/api/v1/calculate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	w := httptest.NewRecorder()
	server.authMiddleware(server.handleCalculate)(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
}

func TestStorage_CRUD(t *testing.T) {
	storage, err := orchestrator.NewSQLiteStorage()
	require.NoError(t, err)

	ctx := context.Background()

	task := &orchestrator.Task{
		ID:         "test-task",
		Expression: "1+1",
		Status:     orchestrator.StatusCompleted,
		Result:     2,
		UserLogin:  "testuser",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	err = storage.AddTask(task)
	require.NoError(t, err)

	retrieved, err := storage.GetTaskByID("test-task", "testuser")
	require.NoError(t, err)
	assert.Equal(t, task.ID, retrieved.ID)
	assert.Equal(t, task.Expression, retrieved.Expression)

	tasks, err := storage.GetUserTasks(ctx, "testuser")
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task.ID, tasks[0].ID)

	task.Result = 3
	err = storage.UpdateTask(task)
	require.NoError(t, err)

	updated, err := storage.GetTaskByID("test-task", "testuser")
	require.NoError(t, err)
	assert.Equal(t, float64(3), updated.Result)

	err = storage.DeleteTask("test-task", "testuser")
	require.NoError(t, err)

	_, err = storage.GetTaskByID("test-task", "testuser")
	assert.Error(t, err)
}

func TestAuthMiddleware(t *testing.T) {
	server, err := orchestrator.NewServer()
	require.NoError(t, err)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "Valid token",
			token:      generateTestToken(t, server.secretKey, "testuser"),
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid token",
			token:      "invalid",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Expired token",
			token:      generateTestToken(t, server.secretKey, "testuser", -time.Hour),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			w := httptest.NewRecorder()
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			server.authMiddleware(testHandler)(w, req)

			resp := w.Result()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantStatus == http.StatusOK {
				assert.True(t, handlerCalled)
			} else {
				assert.False(t, handlerCalled)
			}
		})
	}
}

func generateTestToken(t *testing.T, secret []byte, username string, expOffset ...time.Duration) string {
	exp := time.Now()
	if len(expOffset) > 0 {
		exp = exp.Add(expOffset[0])
	} else {
		exp = exp.Add(time.Hour)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": exp.Unix(),
	})

	tokenString, err := token.SignedString(secret)
	require.NoError(t, err)
	return tokenString
}
