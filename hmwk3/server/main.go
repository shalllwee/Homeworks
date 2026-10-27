package main

import (
	"log"
	"net/http"
	"os"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "photo-editor/docs"
	"photo-editor/internal/handler"
	"photo-editor/internal/storage"
)

// @title                      Photo Editor
// @version                    1.0
// @description                Сервис редактирования изображений
// @host                       localhost:8000
// @BasePath                   /
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
func main() {
	store := storage.NewMemoryStorage()
	h := handler.NewHandler(store)

	mux := http.NewServeMux()

	mux.Handle("/swagger/", httpSwagger.Handler())

	h.RegisterRoutes(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Started in : http://127.0.0.1:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("error: %v", err)
	}
}