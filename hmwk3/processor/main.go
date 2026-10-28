package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type FilterInfo struct {
	Name       string          `json:"name"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

type TaskMessage struct {
	TaskID string          `json:"task_id"`
	Filter json.RawMessage `json:"filter"`
	Image  string          `json:"image"`
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8000"
	}

	var conn *amqp.Connection
	var err error

	for i := 0; i < 15; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("error connect to rabbitmq: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("error chanel rabitmq: %v", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
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

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("error launch consumer: %v", err)
	}

	for d := range msgs {
		var task TaskMessage
		if err := json.Unmarshal(d.Body, &task); err != nil {
			log.Printf("error task parse: %v", err)
			continue
		}
		resultBase64 := processImage(task)

		commitResult(serverURL, task.TaskID, resultBase64)
	}
}

func processImage(task TaskMessage) string {
	if task.Image == "" {
		return "blank_data"
	}

	imgBytes, err := base64.StdEncoding.DecodeString(task.Image)
	if err != nil {
		return task.Image
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return task.Image
	}

	var filter FilterInfo
	_ = json.Unmarshal(task.Filter, &filter)

	var processed image.Image
	switch strings.ToLower(filter.Name) {
	case "negative":
		processed = ApplyNegative(img)
	case "flipx", "flip":
		processed = ApplyFlipX(img)
	case "blur":
		processed = ApplyBlur(img)
	case "sharpen":
		processed = ApplySharpen(img)
	default:
		processed = ApplyNegative(img)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, processed); err != nil {
		log.Printf("error png: %v", err)
		return task.Image
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func commitResult(serverURL, taskID, result string) {
	payload := map[string]string{
		"task_id": taskID,
		"result":  result,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(serverURL+"/commit", "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	defer resp.Body.Close()
}