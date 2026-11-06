package main

import (
	"log"
	"net/http"
	"context"
	"os"
	"time"
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	_ "photo-editor/docs"
	"photo-editor/internal/handler"
	"photo-editor/internal/storage"
)

func connectRabbitMQ(url string) *amqp.Channel {
	for i := 0; i < 15; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, _ := conn.Channel()
			_, _ = ch.QueueDeclare("image_tasks", true, false, false, false, nil)
			log.Println("connected to RabbitQM")
			return ch
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatal("Error connecting to RabbitMQ")
	return nil
}

func connectPostgres(dsn string) *sql.DB {
	for i := 0; i < 15; i++ {
		db, err := sql.Open("postgres", dsn)
		if err == nil && db.Ping() == nil {
			log.Println("connected to PostgreSQL")
			return db
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatal("Error connectiong to PostgreSQL")
	return nil
}

func connectRedis(addr string) *redis.Client {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	for i := 0; i <= 15; i++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			log.Println("connected to redis")
			return rdb
		}
		time.Sleep(2 * time.Second)
	}
	log.Fatal("error connecting to Redis")
	return nil
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
	pgDSN := os.Getenv("POSTGRES_DSN")
	
	if pgDSN == "" {
		pgDSN = "postgres://postgres:password@localhost:5432/photo_db?sslmode=disable"
	}
	db := connectPostgres(pgDSN)
	defer db.Close()

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := connectRedis(redisAddr)
	defer rdb.Close()
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}
	ch := connectRabbitMQ(rabbitURL)
	defer ch.Close()
	store := storage.NewDBStorage(db, rdb)
	h := handler.NewHandler(store, ch, "image_tasks")

	mux := http.NewServeMux()

	mux.Handle("/swagger/", httpSwagger.Handler())
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

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