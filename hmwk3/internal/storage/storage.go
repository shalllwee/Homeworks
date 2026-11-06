package storage

import (
	"errors"
	"photo-editor/internal/models"
)

var ErrNotFound = errors.New("not found")
var	ErrAlreadyExist = errors.New("user already exist")

type Storage interface {
	SaveTask(task *models.Task) error
	GetTask(id string) (*models.Task, error)
	CompleteTask(id string, result string) error

	SaveUser(user *models.User) error
	GetUser(username string) (*models.User, error)

	SaveSession(session *models.Session) error
	GetSession(token string) (*models.Session, error)
}