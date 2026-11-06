package main

import (
	"bytes"
	"encoding/base64"
	"database/sql"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

type FilterInfo struct {
	Name       string          `json:"name"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

type TaskMessage struct {
	TaskID string          `json:"task_id"`
	Filter FilterInfo      `json:"filter"`
	Image  string          `json:"image"`
}

func main() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("metrics start - :9100/metrics")
		if err := http.ListenAndServe(":9100", nil); err != nil {
			log.Fatalf("Error starting metrics server: %v", err)
		}
	}()


	pgDSN := os.Getenv("POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://postgres:password@postgres:5432/photo_db?sslmode=disable"
	}
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@rabbitmq:5672/"
	}

	var conn *amqp.Connection
	for i := 0; i < 15; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			log.Println("Success connecting to RabbitQM")
			break
		}
		time.Sleep(2 * time.Second)
	}
	if conn == nil || err != nil {
		log.Fatalf("fail connecting to RabbitMQ, adress %s: %v", rabbitURL, err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error creating channel rmq %v", err )
	}
	defer ch.Close()

	q, _ := ch.QueueDeclare("image_tasks", true, false, false, false, nil)
	msgs, _ := ch.Consume(q.Name, "", true, false, false, false, nil)

	for d := range msgs {
		var task TaskMessage
		if err := json.Unmarshal(d.Body, &task); err != nil {
			continue
		}

		resultBase64 := processImage(task)
		query := `UPDATE tasks SET status = 'ready', result = $1 WHERE id = $2`
		_, err = db.Exec(query, resultBase64, task.TaskID)
		if err != nil {
			log.Printf("error updating task in DB: %v", err)
		} else {
			log.Printf("Task completed: %s ", task.TaskID)
		}
	}
}

func processImage(task TaskMessage) string {
	filterName := strings.ToLower(task.Filter.Name)
	if filterName == "" {
		filterName = "negative"
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		processDuration.WithLabelValues(filterName).Observe(duration)
		filterCounter.WithLabelValues(filterName).Inc()
	}()

	if task.Image == "" {
		return "blank"
	}

	imgBytes, err := base64.StdEncoding.DecodeString(task.Image)
	if err != nil {
		return task.Image
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return task.Image
	}

	var processed image.Image
	switch filterName {
	case "negative":
		processed = ApplyNegative(img)
	case "flipx", "flip":
		processed = ApplyFlipX(img)
	case "blur":
		processed = ApplyBlur(img, task.Filter.Parameters)
	case "sharpen":
		processed = ApplySharpen(img, task.Filter.Parameters)
	default:
		processed = ApplyNegative(img)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, processed)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}