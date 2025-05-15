package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_http_duration_seconds",
		Help: "Duration of HTTP requests in seconds.",
	}, []string{"path"})
	fetchBlueprintDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "blueprint_fetch_http_duration_seconds",
		Help: "Time taken to collect all blueprint pages",
	})
	logCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bpc_log_count",
		Help: "The number of logs (by type) that have been fired",
	}, []string{"level"})
)
