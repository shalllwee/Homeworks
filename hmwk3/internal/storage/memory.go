package storage

import (
	"sync"
	"photo-editor/internal/models"
)


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

func (s *MemoryStorage) SaveTask(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskCopy := *task
	s.tasks[task.ID] = &taskCopy
	return nil
}

func (s *MemoryStorage) GetTask(id string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[id]
	if !exists {
		return nil, ErrNotFound
	}
	taskCopy := *task
	return &taskCopy, nil
}

func (s *MemoryStorage) CompleteTask(id string, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[id]
	if !exists {
		return ErrNotFound
	}
	task.Status = "ready"
	task.Result = result
	return nil
}

func (s *MemoryStorage) SaveUser(user *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.Username]; exists {
		return ErrAlreadyExist
	}

	userCopy := *user
	s.users[user.Username] = &userCopy
	return nil
}

func (s *MemoryStorage) GetUser(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, exists := s.users[username]
	if !exists {
		return nil, ErrNotFound
	}
	userCopy := *user
	return &userCopy, nil
}

func (s *MemoryStorage) SaveSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessCopy := *session
	s.sessions[session.SessionID] = &sessCopy
	return nil
}

func (s *MemoryStorage) GetSession(token string) (*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return nil, ErrNotFound
	}

	sessCopy := *session
	return &sessCopy, nil
}