package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage() (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", "./tasks.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func createTables(db *sql.DB) error {

	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		expression TEXT NOT NULL,
		status TEXT NOT NULL,
		result REAL,
		user_login TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	return err
}

func (s *SQLiteStorage) AddTask(task *Task) error {
	_, err := s.db.Exec(`
		INSERT INTO tasks 
		(id, expression, status, result, user_login, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Expression, task.Status, task.Result,
		task.UserLogin, task.CreatedAt, task.UpdatedAt)
	return err
}

func (s *SQLiteStorage) GetUserTasks(ctx context.Context, userLogin string) ([]*Task, error) {
	query := `
        SELECT id, expression, status, result, created_at, updated_at 
        FROM tasks 
        WHERE user_login = ? 
        ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userLogin)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(
			&task.ID,
			&task.Expression,
			&task.Status,
			&task.Result,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		task.UserLogin = userLogin
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tasks, nil
}

func (s *SQLiteStorage) GetPendingTask() (*Task, error) {
	row := s.db.QueryRow(`
        SELECT id, expression, status, user_login, created_at
        FROM tasks 
        WHERE status = ? 
        ORDER BY created_at ASC 
        LIMIT 1`,
		StatusPending)

	var task Task
	err := row.Scan(
		&task.ID,
		&task.Expression,
		&task.Status,
		&task.UserLogin,
		&task.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

func (s *SQLiteStorage) GetTaskByID(id string, userLogin string) (*Task, error) {
	row := s.db.QueryRow(`
		SELECT id, expression, status, result, created_at, updated_at 
		FROM tasks 
		WHERE id = ? AND user_login = ?`,
		id, userLogin)

	var task Task
	err := row.Scan(
		&task.ID,
		&task.Expression,
		&task.Status,
		&task.Result,
		&task.CreatedAt,
		&task.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task: %v", err)
	}
	task.UserLogin = userLogin
	return &task, nil
}

func (s *SQLiteStorage) UpdateTask(task *Task) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	existing, _ := s.GetTaskByID(task.ID, task.UserLogin)
	if existing == nil {
		return fmt.Errorf("task not found")
	}

	_, err = tx.Exec(`
		UPDATE tasks 
		SET expression = ?, status = ?, result = ?, updated_at = ? 
		WHERE id = ?`,
		task.Expression,
		task.Status,
		task.Result,
		time.Now().UTC().Format(time.RFC3339),
		task.ID)
	if err != nil {
		return fmt.Errorf("failed to update task: %v", err)
	}
	fmt.Println("TASK DONE!!!!")
	return tx.Commit()
}

func (s *SQLiteStorage) DeleteTask(id string, userLogin string) error {
	_, err := s.db.Exec(`
		DELETE FROM tasks 
		WHERE id = ? AND user_login = ?`,
		id, userLogin)
	if err != nil {
		return fmt.Errorf("failed to delete task: %v", err)
	}
	return nil
}

func (s *SQLiteStorage) GetTasksByStatus(userLogin string, status string) ([]*Task, error) {
	rows, err := s.db.Query(`
		SELECT id, expression, status, result, created_at, updated_at 
		FROM tasks 
		WHERE user_login = ? AND status = ? 
		ORDER BY created_at DESC`,
		userLogin, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %v", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID,
			&task.Expression,
			&task.Status,
			&task.Result,
			&task.CreatedAt,
			&task.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %v", err)
		}
		task.UserLogin = userLogin
		tasks = append(tasks, &task)
	}
	return tasks, nil
}

func (s *SQLiteStorage) CreateUser(user *User) error {
	_, err := s.db.Exec(`
		INSERT INTO users 
		(login, password_hash, created_at) 
		VALUES (?, ?, ?)`,
		user.Login,
		user.PasswordHash,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

func (s *SQLiteStorage) GetUser(login string) (*User, error) {
	row := s.db.QueryRow(`
		SELECT login, password_hash, created_at 
		FROM users 
		WHERE login = ?`,
		login)

	var user User
	err := row.Scan(
		&user.Login,
		&user.PasswordHash,
		&user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	return &user, nil
}
