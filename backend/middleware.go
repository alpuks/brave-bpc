package main

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	headerContentType = "Content-Type"
	headerContentJson = "application/json; charset=utf-8"
)

type (
	ctxLogger    struct{}
	ctxRequestId struct{}
)

// creates a requestId, logger, retrieves session data, and stores them in the request context.
func (app *app) requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()

		requestId := app.flake.Generate()
		logger := app.logger.With(zap.Int64("request_id", requestId.Int64()))

		ctx := context.WithValue(r.Context(), ctxRequestId{}, requestId)
		ctx = context.WithValue(ctx, ctxLogger{}, logger)

		next.ServeHTTP(w, r.WithContext(ctx))

		// write prometheus timing metrics
		httpRequestDuration.WithLabelValues(r.Pattern).Observe(time.Since(t).Seconds())
	})
}

func (app *app) authMiddlewareFactory(requiredLevel int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := getLoggerFromContext(r.Context())

			user := app.getUserFromSession(r)
			if user.IsLoggedIn() {
				logger = logger.With(zap.Any("user", user))
				r = r.WithContext(context.WithValue(r.Context(), ctxLogger{}, logger))
			} else if requiredLevel > authLevel_Unauthorized {
				logger.Info("user not logged in", zap.Any("request", r), zap.String("pattern", r.Pattern))
				httpError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if user.Level < requiredLevel {
				logger.Info("user not authorized", zap.Int("auth_level", user.Level), zap.Int("required_level", requiredLevel), zap.String("pattern", r.Pattern))
				httpError(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(headerContentType, headerContentJson)
		next.ServeHTTP(w, r)
	})
}
