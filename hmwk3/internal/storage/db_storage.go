package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"photo-editor/internal/models"
)

type DBStorage struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewDBStorage(db *sql.DB, rdb *redis.Client) *DBStorage {
	return &DBStorage{
		db:  db,
		rdb: rdb,
	}
}

func (s *DBStorage) SaveTask(task *models.Task) error {
	query := `INSERT INTO tasks (id, user_id, status, result, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := s.db.Exec(query, task.ID, task.UserID, task.Status, task.Result, task.CreatedAt)
	return err
}

func (s *DBStorage) GetTask(id string) (*models.Task, error) {
	query := `SELECT id, user_id, status, COALESCE(result, ''), created_at FROM tasks WHERE id = $1`
	row := s.db.QueryRow(query, id)

	var task models.Task
	err := row.Scan(&task.ID, &task.UserID, &task.Status, &task.Result, &task.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (s *DBStorage) CompleteTask(id string, result string) error {
	query := `UPDATE tasks SET status = 'ready', result = $1 WHERE id = $2 AND status = 'in_progress'`
	res, err := s.db.Exec(query, result, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}


func (s *DBStorage) SaveUser(user *models.User) error {
	query := `INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, user.ID, user.Username, user.PasswordHash)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrAlreadyExist
		}
		return err
	}
	return nil
}

func (s *DBStorage) GetUser(username string) (*models.User, error) {
	query := `SELECT id, username, password_hash FROM users WHERE username = $1`
	row := s.db.QueryRow(query, username)

	var user models.User
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}


func (s *DBStorage) SaveSession(session *models.Session) error {
	ctx := context.Background()
	return s.rdb.Set(ctx, "session:"+session.SessionID, session.UserID, 24*time.Hour).Err()
}

func (s *DBStorage) GetSession(token string) (*models.Session, error) {
	ctx := context.Background()
	userID, err := s.rdb.Get(ctx, "session:"+token).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &models.Session{
		UserID:    userID,
		SessionID: token,
	}, nil
}