package orchestrator

import (
	"fmt"
	"sync"
	"time"
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
			copyTask := *task
			tasksForExpression = append(tasksForExpression, &copyTask)
		}
	}
	return tasksForExpression
}
func (s *Storage) GetPendingTask() *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldestTask *Task
	var oldestTime time.Time

	for _, task := range s.tasks {
		taskTime, err := time.Parse(time.RFC3339, task.CreatedAt)
		if err != nil {
			continue
		}

		if task.Status == StatusPending &&
			(oldestTask == nil || taskTime.Before(oldestTime)) {
			oldestTask = task
			oldestTime = taskTime
		}
	}

	if oldestTask != nil {
		copyTask := *oldestTask

		oldestTask.Status = StatusInProgress
		oldestTask.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		return &copyTask
	}

	return nil
}

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil
	}

	copyTask := *task
	return &copyTask
}

func (s *Storage) UpdateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return fmt.Errorf("task with ID %s does not exist", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}
