package orchestrator

import (
	"fmt"
	"sync"
)

type Storage struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

func NewStorage() *Storage {
	return &Storage{
		tasks: make(map[string]*Task),
	}
}

func (s *Storage) GetAllTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var allTasks []*Task
	for _, task := range s.tasks {
		// Создаем копию задачи, чтобы избежать race condition
		copyTask := *task
		allTasks = append(allTasks, &copyTask)
	}
	return allTasks
}

func (s *Storage) GetTasksByExpressionID(expressionID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasksForExpression []*Task
	for _, task := range s.tasks {
		if task.ID == expressionID {
			// Создаем копию задачи, чтобы избежать race condition
			copyTask := *task
			tasksForExpression = append(tasksForExpression, &copyTask)
		}
	}
	return tasksForExpression
}

func (s *Storage) GetPendingTask() *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.tasks {
		if task.Status == StatusNew { // Используем константу StatusNew
			// Создаем копию задачи, чтобы избежать race condition
			copyTask := *task
			return &copyTask
		}
	}
	return nil
}

// AddTask добавляет задачу в хранилище
func (s *Storage) AddTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

func (s *Storage) GetTaskByID(taskID string) *Task {
	s.mu.RLock() // Используем RLock, так как только читаем задачу
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil // Возвращаем nil, если задача не найдена
	}
	// Создаем копию задачи, чтобы избежать race condition
	copyTask := *task
	return &copyTask
}

func (s *Storage) UpdateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task with ID %s does not exist", task.ID) // Обрабатываем случай, когда задача не найдена
	}

	s.tasks[task.ID] = task
	return nil
}
