package orchestrator

import (
	"strings"

	"github.com/google/uuid" // Import для генерации UUID
)

// Task структура задачи в оркестраторе
type Task struct {
	ID         string  `json:"id"`         // Указывать теги для JSON обязательно, чтобы marshaling/unmarshaling работал корректно
	Expression string  `json:"expression"` // Математическое выражение
	Status     string  `json:"status"`     // Статусы лучше вынести в константы
	Result     float64 `json:"result"`     // Результат
}

// Константы для статусов задач (дублирование из agent/worker.go - TODO: вынести в общий пакет)
const (
	StatusPending    = "Pending"
	StatusNew        = "New"
	StatusInProgress = "In Progress"
	StatusCompleted  = "Completed"
	StatusFailed     = "Failed"
)

// CreateTask создает задачу с математическим выражением.
func CreateTask(expression string) Task {
	// Убираем пробелы из выражения
	expression = strings.ReplaceAll(expression, " ", "")

	// Создаем новую задачу
	task := Task{
		ID:         GenerateTaskID(), // Используем UUID для более надежной генерации ID
		Expression: expression,
		Status:     StatusNew, // Лучше начинать со статуса New, а не Pending (Pending - когда задача в очереди на обработку)
		Result:     0,         //Инициализируем нулем, чтобы не было неожиданных значений
	}

	return task
}

// GenerateTaskID генерирует ID задачи (используем UUID).
func GenerateTaskID() string {
	// Генерация ID задачи (используем UUID для уникальности)
	return uuid.New().String()
}
