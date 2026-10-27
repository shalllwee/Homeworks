package models

import "time"

type Task struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Session struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}