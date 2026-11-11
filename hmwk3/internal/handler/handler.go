package handler

import (
	"context"
	"image"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"encoding/base64"
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
		session, err := h.storage.GetSession(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		ctx := context.WithValue(r.Context(), "user_id", session.UserID)
		next(w, r.WithContext(ctx))
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
	password := req.Password
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
	password := req.Password
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

type FilterParamsRequest struct {
	Sigma *float64 `json:"sigma,omitempty"`
}

type FilterRequest struct {
	Name       string          `json:"name" example:"negative"`
	Parameters json.RawMessage `json:"parameters,omitempty" swaggertype:"primitive,string" example:"{\"sigma\": 2.0}"`
}

type CreateTaskRequest struct {
	Filter FilterRequest `json:"filter"`
	Image  string        `json:"image"`
}

type RabbitTaskMessage struct {
	TaskID string          `json:"task_id"`
	Filter FilterRequest   `json:"filter"`
	Image  string          `json:"image"`
}

// CreateTask godoc
// @Summary      Создать задачу
// @Description  Создает задачу на обработку изображения
// @Tags         tasks
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request body handler.CreateTaskRequest true "параметры задачи"
// @Success      201  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /task [post]
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok || userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 15<<20)

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json or payload too large"})
		return
	}

	filterName := strings.ToLower(req.Filter.Name)
	if filterName != "negative" && filterName != "flipx" && filterName != "blur" && filterName != "sharpen" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid filter"})
		return
	}

	if (filterName == "blur" || filterName == "sharpen") && len(req.Filter.Parameters) > 0 {
		var p FilterParamsRequest
		if err := json.Unmarshal(req.Filter.Parameters, &p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid filter parameters"})
			return
		}
		if p.Sigma != nil && *p.Sigma <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sigma should be more than 0"})
			return
		}
	}
	if strings.TrimSpace(req.Image) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image payload can't be empty"})
		return
	}
	imgBytes, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 image"})
		return
	}
	_, _, err = image.DecodeConfig(bytes.NewReader(imgBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "imvalid image format"})
		return
	}

	if h.rabbitChan == nil || h.rabbitChan.IsClosed() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message broker is unavalable"})
		return
	}
	taskID := uuid.New().String()
	task := &models.Task{
		ID:        taskID,
		UserID:    userID,
		Status:    "in_progress",
		CreatedAt: time.Now(),
	}
	if err := h.storage.SaveTask(task); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	msgBytes, _ := json.Marshal(RabbitTaskMessage{
		TaskID: taskID,
		Filter: req.Filter,
		Image:  req.Image,
	})

	err = h.rabbitChan.PublishWithContext(
		r.Context(),
		"",
		h.queueName,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         msgBytes,
		},
	)
	if err != nil {
		_ = h.storage.CompleteTask(taskID, "")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "failed to queue task"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"task_id": taskID,
	})
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
	currentUserID := r.Context().Value("user_id").(string)
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

	if task.UserID != currentUserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
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
// @Produce      image/png
// @Param        task_id path string true "ID задачи"
// @Success      200  {string}  binary "PNG"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /result/{task_id} [get]
func (h *Handler) GetResult(w http.ResponseWriter, r *http.Request) {
	currentUserID := r.Context().Value("user_id").(string)
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

	if task.UserID != currentUserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if task.Status != "ready" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task not ready yet"})
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") || strings.Contains(accept, "image/") {
		imgBytes, err := base64.StdEncoding.DecodeString(task.Result)
		if err == nil {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(imgBytes)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"result": task.Result,
	})
}