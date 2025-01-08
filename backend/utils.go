package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/hashicorp/go-envparse"
	"github.com/pressly/goose/v3"
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
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return zap.New(zapcore.NewCore(
		zaplogfmt.NewEncoder(cfg),
		os.Stdout,
		level,
	), zap.AddStacktrace(zapcore.ErrorLevel))
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
	port := getEnvWithDefault(envDbPort, "3307")
	name := getEnvWithDefault(envDbName, "local")

	if host[0] == '/' {
		return fmt.Sprintf("%s:%s@unix(%s)/%s", user, pass, host, name)
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, name)
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

func (app *app) runMigrations() {
	var err error
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(zap.NewStdLog(app.logger))

	if err = goose.SetDialect("mysql"); err != nil {
		app.logger.Fatal("unable to setup db for migrations", zap.Error(err))
	}
	if os.Getenv("MIGRATE_DOWN") != "" {
		app.logger.Info("migrate db down")
		if err = goose.Down(app.db, "migrations"); err != nil {
			app.logger.Warn("unable to down migrations", zap.Error(err))
		}
	}
	app.logger.Info("migrate db up")
	if err = goose.Up(app.db, "migrations"); err != nil {
		app.logger.Fatal("unable to up migrations", zap.Error(err))
	}
}

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
