package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"time"

	"github.com/antihax/goesi"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	esiUserAgent = "brave-bpc - Al Heamer"
)

var (
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

type app struct {
	logger  *zap.Logger
	db      *sql.DB
	session *sessions.CookieStore
	esi     *goesi.APIClient
	esiCtx  context.Context
}

func main() {
	var logLevel zapcore.Level
	switch os.Getenv("ENVIRONMENT") {
	default:
	case "dev":
		logLevel = zapcore.DebugLevel
	case "prod", "production":
		logLevel = zapcore.WarnLevel
	}

	logger := newDefaultLogger(logLevel)
	app := &app{
		logger:  logger,
		session: newCookieStore(),
		esi:     goesi.NewAPIClient(httpClient, esiUserAgent),
	}
	app.loadEnv()

	var err error
	if app.db, err = sql.Open("mysql", dbConnectString()); err != nil {
		logger.Fatal("error opening db connection", zap.Error(err))
	}
	defer app.db.Close()
	if err = app.db.Ping(); err != nil {
		logger.Fatal("error establishing db connetion", zap.Error(err))
	}
	app.runMigrations()

	http.HandleFunc("/", app.root)
	http.HandleFunc("/login", app.login)
	http.HandleFunc("/auth", app.auth)

	logger.Fatal("error serving http", zap.Error(http.ListenAndServe("localhost:2727", nil)))
}

func (app *app) root(w http.ResponseWriter, r *http.Request) {

}

// standard login
func (app *app) login(w http.ResponseWriter, r *http.Request) {
	app.doLogin(w, r, nil)
}

// director login
func (app *app) auth(w http.ResponseWriter, r *http.Request) {
	app.doLogin(w, r, []string{
		"esi-corporations.read_blueprints.v1",
		"esi-industry.read_corporation_jobs.v1",
	})
}
