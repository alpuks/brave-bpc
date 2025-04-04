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
)
