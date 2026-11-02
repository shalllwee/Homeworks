package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"photo-editor/internal/models"
	"photo-editor/internal/storage"
)

type Handler struct {
	storage storage.Storage
	rabbitChan *amqp.Channel
	queueName  string
}

func NewHandler(s storage.Storage, ch *amqp.Channel, queueName string) *Handler {
	return &Handler {
		storage: 	s,
		rabbitChan: ch,
		queueName:  queueName,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /commit", h.CommitTask)
	mux.HandleFunc("POST /task", h.authMiddleware(h.CreateTask))
	mux.HandleFunc("GET /status/{task_id}", h.authMiddleware(h.GetStatus))
	mux.HandleFunc("GET /result/{task_id}", h.authMiddleware(h.GetResult))
}

func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			authHeader = r.Header.Get("authorization")
		}
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		token := strings.TrimSpace(authHeader[7:])
		_, err := h.storage.GetSession(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		next(w, r)
	}
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Register godoc
// @Summary      Регистрация пользователя
// @Description  Регистрирует нового пользователя
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "username и password"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Router       /register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password cant be empty"})
		return
	}

	hashPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "fail hash password"})
		return
	}

	user := &models.User{
		ID:       uuid.New().String(),
		Username: username,
		PasswordHash: string(hashPassword),
	}
	
	if err := h.storage.SaveUser(user); err != nil {
		if errors.Is(err, storage.ErrAlreadyExist) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "user already exist"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "registered successfully"})
}


// Login godoc
// @Summary      Вход в систему
// @Description  Логин пользователя и присвоение токена
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body map[string]string true "username и password"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password cant be empty"})
		return
	}

	user, err := h.storage.GetUser(username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid login or password"})
		return
	}
	token := uuid.New().String()
	if err := h.storage.SaveSession(&models.Session{
		UserID:    user.ID,
		SessionID: token,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}
type RabbitTaskMessage struct {
	TaskID string          `json:"task_id"`
	Filter json.RawMessage `json:"filter"`
	Image  string          `json:"image"`
}

// CreateTask godoc
// @Summary      Создать задачу
// @Description  Создает задачу на обработку изображения
// @Tags         tasks
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /task [post]
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	taskID := uuid.New().String()
	var body struct {
		Filter json.RawMessage `json:"filter"`
		Image  string          `json:"image"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	task := &models.Task{
		ID:        taskID,
		Status:    "in_progress",
		CreatedAt: time.Now(),
	}
	if err := h.storage.SaveTask(task); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if h.rabbitChan != nil {
		msgData, _ := json.Marshal(RabbitTaskMessage{
			TaskID: taskID,
			Filter: body.Filter,
			Image:  body.Image,
		})

		err := h.rabbitChan.PublishWithContext(
			context.Background(),
			"",
			h.queueName,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        msgData,
			},
		)
		if err != nil {
			log.Printf("Error sending to RMQ: %v", err)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"task_id": taskID,
	})
}

func (h *Handler) CommitTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.storage.CompleteTask(req.TaskID, req.Result); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "committed"})
}

// GetStatus godoc
// @Summary      Получить статус задачи
// @Description  Выдаёт текущий статус
// @Tags         tasks
// @Security     BearerAuth
// @Produce      json
// @Param        task_id path string true "ID задачи"
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /status/{task_id} [get]
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := h.storage.GetTask(taskID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": task.Status,
	})
}

// GetResult godoc
// @Summary      Получить результат задачи
// @Description  Выдаёт результат завершенной задачи
// @Tags         tasks
// @Security     BearerAuth
// @Produce      json
// @Param        task_id path string true "ID задачи"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /result/{task_id} [get]
func (h *Handler) GetResult(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := h.storage.GetTask(taskID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if task.Status != "ready" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task not ready yet"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"result": task.Result,
	})
}