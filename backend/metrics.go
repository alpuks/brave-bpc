package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func timerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(w, r)
		httpRequestDuration.WithLabelValues(r.Pattern).Observe(time.Since(t).Seconds())
	})
}

var (
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_http_duration_seconds",
		Help: "Duration of HTTP requests in seconds.",
	}, []string{"path"})
)
