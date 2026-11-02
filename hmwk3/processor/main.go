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

	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("metrics start - :9100/metrics")
		if err := http.ListenAndServe(":9100", nil); err != nil {
			log.Fatalf("Error starting metrics server: %v", err)
		}
	}()

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8000"
	}

	var conn *amqp.Connection
	for i := 0; i < 15; i++ {
		var err error
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	defer conn.Close()

	ch, _ := conn.Channel()
	defer ch.Close()

	q, _ := ch.QueueDeclare("image_tasks", true, false, false, false, nil)
	msgs, _ := ch.Consume(q.Name, "", true, false, false, false, nil)

	for d := range msgs {
		var task TaskMessage
		if err := json.Unmarshal(d.Body, &task); err != nil {
			continue
		}

		resultBase64 := processImage(task)
		commitResult(serverURL, task.TaskID, resultBase64)
	}
}

func processImage(task TaskMessage) string {
	var filter FilterInfo
	_ = json.Unmarshal(task.Filter, &filter)
	filterName := strings.ToLower(filter.Name)
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
		processed = ApplyBlur(img)
	case "sharpen":
		processed = ApplySharpen(img)
	default:
		processed = ApplyNegative(img)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, processed)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func commitResult(serverURL, taskID, result string) {
	payload, _ := json.Marshal(map[string]string{
		"task_id": taskID,
		"result":  result,
	})
	resp, err := http.Post(serverURL+"/commit", "application/json", bytes.NewReader(payload))
	if err == nil {
		_ = resp.Body.Close()
	}
}