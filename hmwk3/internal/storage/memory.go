package storage

import (
	"errors"
	"sync"

	"photo-editor/internal/models"
)

var ErrNotFound = errors.New("not found")

type MemoryStorage struct {
	mu       sync.RWMutex
	tasks    map[string]*models.Task
	users    map[string]*models.User
	sessions map[string]*models.Session
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		tasks:    make(map[string]*models.Task),
		users:    make(map[string]*models.User),
		sessions: make(map[string]*models.Session),
	}
}

func (s *MemoryStorage) SaveTask(task *models.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *MemoryStorage) GetTask(id string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[id]
	if !exists {
		return nil, ErrNotFound
	}
	return task, nil
}

func (s *MemoryStorage) CompleteTask(id string, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, exists := s.tasks[id]; exists {
		task.Status = "ready"
		task.Result = result
	}
}

func (s *MemoryStorage) SaveUser(user *models.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.Username] = user
}

func (s *MemoryStorage) GetUser(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, exists := s.users[username]
	if !exists {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStorage) SaveSession(session *models.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionID] = session
}

func (s *MemoryStorage) GetSession(token string) (*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, exists := s.sessions[token]
	if !exists {
		return nil, ErrNotFound
	}
	return session, nil
}