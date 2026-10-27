package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"github.com/google/uuid"
	"photo-editor/internal/models"
	"photo-editor/internal/storage"
)

type Handler struct {
	storage *storage.MemoryStorage
}

func NewHandler(s *storage.MemoryStorage) *Handler {
	return &Handler{storage: s}
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
		authHeader := r.Header.Get("Autorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unautorized"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
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

	user := &models.User{
		ID:       uuid.New().String(),
		Username: req.Username,
		Password: req.Password,
	}
	h.storage.SaveUser(user)

	writeJSON(w, http.StatusCreated, map[string]string{"message": "register done"})
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

	user, err := h.storage.GetUser(req.Username)
	if err != nil || user.Password != req.Password {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid "})
		return
	}

	token := uuid.New().String()
	h.storage.SaveSession(&models.Session{
		UserID:    user.ID,
		SessionID: token,
	})
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
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

	task := &models.Task{
		ID:        taskID,
		Status:    "in_progress",
		CreatedAt: time.Now(),
	}
	h.storage.SaveTask(task)

	go func(id string) {
		time.Sleep(2 * time.Second)
		h.storage.CompleteTask(id, "filtered_image")
	}(taskID)

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