package main

import (
	"log"
	"net/http"
	"os"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	_ "photo-editor/docs"
	"photo-editor/internal/handler"
	"photo-editor/internal/storage"
)

func connectRabbitMQ(url string) (*amqp.Connection, *amqp.Channel) {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 15; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("error connect rabbitqm: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("error channel rabbitmq: %v", err)
	}

	_, err = ch.QueueDeclare(
		"image_tasks",
		true, 
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("error queue: %v", err)
	}

	return conn, ch
}


// @title                      Photo Editor
// @version                    1.0
// @description                Сервис редактирования изображений
// @host                       localhost:8000
// @BasePath                   /
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	var ch *amqp.Channel
	if rabbitURL != "" {
		_, ch = connectRabbitMQ(rabbitURL)
	}
	store := storage.NewMemoryStorage()
	h := handler.NewHandler(store, ch, "image_tasks")

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