package main

import (
	"embed"
	"fmt"
	"log"
	"os"

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
	envDbUser      = "DB_USER"
	envDbPass      = "DB_PASS"
	envDbHost      = "DB_HOST"
	envDbPort      = "DB_PORT"
	envDbName      = "DB_NAME"
	cookieName     = "brave-bpc"
)

func newDefaultLogger(level zapcore.Level) *zap.Logger {
	var (
		logger *zap.Logger
		err    error
	)

	switch os.Getenv("ENVIRONMENT") {
	default:
		if logger, err = zap.NewDevelopment(); err != nil {
			log.Fatal("failed to start development logger")
		}

	case "prod", "production":
		cfg := zap.NewProductionEncoderConfig()
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
		logger = zap.New(zapcore.NewCore(
			zaplogfmt.NewEncoder(cfg),
			os.Stdout,
			level,
		), zap.AddStacktrace(zapcore.ErrorLevel))
	}

	return logger
}

func newCookieStore() *sessions.CookieStore {
	return sessions.NewCookieStore(
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

// loads .env file and sets any unset env vars
func (app *app) loadEnv() {
	fp, err := os.Open("./.env")
	if err != nil {
		app.logger.Warn("error opening .env file", zap.Error(err))
		return
	}

	env, err := envparse.Parse(fp)
	if err != nil {
		app.logger.Fatal("error parsing env file", zap.Error(err))
	}

	for k, v := range env {
		setUnsetEnv(k, v)
	}
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
