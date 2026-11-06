package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	processDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "image_process_duration_seconds",
			Help:    "Время выполнения обработки изображения",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"filter"},
	)

	filterCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "image_filters_total",
			Help: "Общее количество применений фильтров",
		},
		[]string{"filter"},
	)
)