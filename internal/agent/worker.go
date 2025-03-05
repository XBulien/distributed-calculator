package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Knetic/govaluate"
)

// Task структура задачи
type Task struct {
	ID         string  `json:"id"`         // Указывать теги для JSON обязательно, чтобы marshaling/unmarshaling работал корректно
	Expression string  `json:"expression"` // Математическое выражение
	Status     string  `json:"status"`     // Статусы лучше вынести в константы (см. ниже)
	Result     float64 `json:"result"`     // Результат
}

// Константы для статусов задач
const (
	StatusNew        = "New"
	StatusInProgress = "In Progress"
	StatusCompleted  = "Completed"
	StatusFailed     = "Failed" // Добавил статус Failed
)

type Agent struct {
	NumWorkers          int
	OrchestratorAddress string // Добавим адрес оркестратора в структуру агента, чтобы можно было конфигурировать
}

func NewAgent(numWorkers int, orchestratorAddress string) *Agent {
	return &Agent{
		NumWorkers:          numWorkers,
		OrchestratorAddress: orchestratorAddress,
	}
}

func (a *Agent) Start() error {
	// Запуск нескольких горутин для обработки задач
	for i := 0; i < a.NumWorkers; i++ {
		go a.ProcessTasks()
	}

	// Ожидаем завершения работы горутин (лучше использовать graceful shutdown с сигналами)
	select {}
	// TODO: Replace select {} with a mechanism to handle signals (e.g., os.Interrupt)
	//       for graceful shutdown.  This will allow the agent to properly clean up
	//       before exiting.
}

func (a *Agent) ProcessTasks() {
	for {
		// Получение задачи от оркестратора
		task, err := a.GetTask()
		if err != nil {
			log.Printf("Failed to get task: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Устанавливаем статус задачи "In Progress"
		task.Status = StatusInProgress // Используем константу
		err = a.UpdateTaskStatus(task)
		if err != nil {
			log.Printf("Failed to update task status to '%s' for %s: %v", StatusInProgress, task.ID, err)
		}

		// Выполнение вычисления
		result, err := a.Calculate(task.Expression)
		if err != nil {
			log.Printf("Failed to calculate task %s: %v", task.ID, err)
			task.Status = StatusFailed // Устанавливаем статус Failed при ошибке
			a.UpdateTaskStatus(task)   // Обновляем статус в оркестраторе
			time.Sleep(1 * time.Second)
			continue
		}

		// Отправляем результат обратно в оркестратор
		err = a.SendTaskResult(task.ID, result)
		if err != nil {
			log.Printf("Failed to send task result for %s: %v", task.ID, err)
			task.Status = StatusFailed // Устанавливаем статус Failed при ошибке
			a.UpdateTaskStatus(task)   // Обновляем статус в оркестраторе
		} else {
			// Устанавливаем статус задачи как "Completed"
			task.Status = StatusCompleted // Используем константу
			err = a.UpdateTaskStatus(task)
			if err != nil {
				log.Printf("Failed to update task status to '%s' for %s: %v", StatusCompleted, task.ID, err)
			}
		}
	}
}

func (a *Agent) UpdateTaskStatus(task *Task) error {
	// Создание payload для обновления статуса задачи
	payload := struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}{
		ID:     task.ID,
		Status: task.Status,
	}

	// Отправка запроса на обновление статуса задачи в оркестратор
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task status update: %w", err)
	}

	url := fmt.Sprintf("%s/internal/task/status", a.OrchestratorAddress) // Используем адрес оркестратора
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send task status update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body.String()) // Log the response body for debugging
	}

	return nil
}

func (a *Agent) GetTask() (*Task, error) {
	// Запрос задачи у оркестратора
	url := fmt.Sprintf("%s/internal/task", a.OrchestratorAddress) // Используем адрес оркестратора
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body.String()) // Log the response body for debugging
	}

	var result struct {
		Task *Task `json:"task"` // Указывать теги для JSON обязательно
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode task response: %w", err)
	}

	log.Printf("Received task: %+v\n", result.Task)
	return result.Task, nil
}

// Функция для вычисления математического выражения
func (a *Agent) Calculate(expression string) (float64, error) {
	// Простой парсинг и вычисление выражения
	result, err := EvaluateExpression(expression)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return result, nil
}

// Функция для вычисления выражения
func EvaluateExpression(expression string) (float64, error) {
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return 0, fmt.Errorf("failed to parse expression: %w", err)
	}

	result, err := expr.Evaluate(nil)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Convert result to float64
	floatResult, ok := result.(float64)
	if !ok {
		return 0, fmt.Errorf("expression did not evaluate to a number")
	}

	return floatResult, nil
}

func (a *Agent) SendTaskResult(taskID string, result float64) error {
	// Создание payload для отправки результата
	payload := struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}{
		ID:     taskID,
		Result: result,
	}

	// Маршализация payload в JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task result: %w", err)
	}

	// Отправка запроса с результатом в оркестратор
	url := fmt.Sprintf("%s/internal/task/result", a.OrchestratorAddress) // Используем адрес оркестратора
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send task result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body.String()) // Log the response body for debugging
	}

	return nil
}
