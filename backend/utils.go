package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/hashicorp/go-envparse"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	envAppId       = "ESI_APP_ID"
	envAppSecret   = "ESI_APP_SECRET"
	envAppRedirect = "ESI_APP_REDIRECT"
	envMigrateDown = "MIGRATE_DOWN"
	envEnvironment = "ENVIRONMENT"
	envHttpPort    = "HTTP_PORT"
	envJwtSkew     = "JWT_SKEW"
	envDbUser      = "DB_USER"
	envDbPass      = "DB_PASS"
	envDbHost      = "DB_HOST"
	envDbPort      = "DB_PORT"
	envDbName      = "DB_NAME"

	cookieSession = "brave-bpc-session"
	cookieUser    = "brave-bpc"
)

func newDefaultLogger(env string) *zap.Logger {
	var (
		logger *zap.Logger
		core   zapcore.Core
		opts   []zap.Option
	)

	// this instance of Getenv does not account for vars loaded from .env file
	switch env {
	case "dev", "development":
		core = zapcore.NewCore(
			zaplogfmt.NewEncoder(zap.NewDevelopmentEncoderConfig()),
			os.Stderr,
			zapcore.DebugLevel,
		)
		opts = []zap.Option{
			zap.AddStacktrace(zapcore.ErrorLevel),
			zap.Development(),
		}

	default:
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		core = zapcore.NewCore(
			zaplogfmt.NewEncoder(cfg),
			os.Stderr,
			zapcore.InfoLevel,
		)
		opts = []zap.Option{
			zap.AddStacktrace(zapcore.ErrorLevel),
		}
	}

	logger = zap.New(core, opts...)
	zap.RedirectStdLog(logger)
	return logger
}

func newSnowflake(logger *zap.Logger) *snowflake.Node {
	snowflake.Epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	flake, err := snowflake.NewNode(1)
	if err != nil {
		logger.Fatal("failed to start snowflake", zap.Error(err))
	}

	return flake
}

func newSessionStore() sessions.Store {
	return sessions.NewFilesystemStore(
		os.TempDir(),
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	)
}

func dbConnectString() string {
	user := getEnvWithDefault(envDbUser, "local")
	pass := getEnvWithDefault(envDbPass, "local")
	host := getEnvWithDefault(envDbHost, "localhost")
	port := getEnvWithDefault(envDbPort, "3308")
	name := getEnvWithDefault(envDbName, "local")

	if host[0] == '/' {
		return fmt.Sprintf("%s:%s@unix(%s)/%s", user, pass, host, name)
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, name)
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

var errEnvFile = errors.New("failed to load .env file")

// loadEnv parses the contents of .env and sets any unset environment variables
func loadEnv() (*runtimeConfig, error) {
	fp, err := os.Open("./.env")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, errors.Join(errEnvFile, err)
	}
	defer fp.Close()

	env, err := envparse.Parse(fp)
	if err != nil {
		return nil, err
	}

	for k, v := range env {
		setUnsetEnv(k, v)
	}

	skewStr := getEnvWithDefault(envJwtSkew, "5m")
	skew, err := time.ParseDuration(skewStr)
	if err != nil {
		skew = time.Second
		return nil, fmt.Errorf("error parsing jwt skew env var: %w", err)
	}

	return &runtimeConfig{
		appId:       os.Getenv(envAppId),
		appSecret:   os.Getenv(envAppSecret),
		appRedirect: os.Getenv(envAppRedirect),
		environment: os.Getenv(envEnvironment),
		migrateDown: os.Getenv(envMigrateDown),
		httpPort:    getEnvWithDefault(envHttpPort, "2727"),
		jwtSkew:     skew,
	}, nil
}

// checks if an environment variable has been set.
// if it hasn't, set it with the value param
// returns the set value
func setUnsetEnv(key string, value string) {
	if _, set := os.LookupEnv(key); !set {
		os.Setenv(key, value)
	}
}

func getEnvWithDefault(key string, value string) string {
	val, set := os.LookupEnv(key)
	if set {
		return val
	}
	return value
}

func getLoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(ctxLogger{}).(*zap.Logger); ok {
		return logger
	}
	return nil
}

func (app *app) getUserFromSession(r *http.Request) *user {
	s, _ := app.sessionStore.Get(r, cookieSession)
	if user, ok := s.Values[sessionUserData{}].(user); ok {
		return &user
	}
	return &user{}
}

// httpError creates a json formatted error message with code.
// You should return from your handler after calling this.
func httpError(w http.ResponseWriter, message string, statusCode int) {
	h := w.Header()
	h.Del("Content-Length")
	//h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"code":%d,"msg":"%s"}`, statusCode, message)
}

// httpWrite converts data to a json object and writes it to http.ResponseWriter
func httpWrite(w http.ResponseWriter, data any) {
	buf, err := json.Marshal(data)
	if err != nil {
		httpError(w, "error marshalling json", http.StatusInternalServerError)
		return
	}
	w.Write(buf)
}
