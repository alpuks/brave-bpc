package main

import (
	"database/sql"
	"fmt"
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
	dao     *dao
	session *sessions.CookieStore
	esi     *goesi.APIClient
}

func main() {
	logLevel := zapcore.DebugLevel
	switch os.Getenv("ENVIRONMENT") {
	case "prod", "production":
		logLevel = zapcore.WarnLevel
	}

	logger := newDefaultLogger(logLevel)
	defer logger.Sync()

	app := &app{
		logger:  logger,
		session: newCookieStore(),
		esi:     goesi.NewAPIClient(httpClient, esiUserAgent),
	}
	app.loadEnv()

	if db, err := sql.Open("mysql", dbConnectString()); err != nil {
		logger.Fatal("error opening db connection", zap.Error(err))
	} else {
		defer db.Close()
		if err = db.Ping(); err != nil {
			logger.Fatal("error establishing db connetion", zap.Error(err))
		}
		app.dao = newDao(db)
		app.dao.runMigrations(logger)
	}

	http.HandleFunc("/", app.root)
	http.HandleFunc("/login", app.login)
	http.HandleFunc("/add", app.loginAdd)
	http.HandleFunc("/auth", app.auth)

	logger.Debug("listening on port 2727")
	logger.Fatal("error serving http", zap.Error(http.ListenAndServe("localhost:2727", nil)))
}

func (app *app) root(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	var values string
	for k, v := range s.Values {
		values = values + fmt.Sprintf("%s: %v<br/>", k, v)
	}

	body := `
<html>
<body>
` + values + `
<ul>
<li><a href="/login">login</a>
<li><a href="/add">add character</a>
<li><a href="/auth">add scopes</a>
</ul>
</body>
</html>
`
	w.Write([]byte(body))
}

// standard login
func (app *app) login(w http.ResponseWriter, r *http.Request) {
	app.doLogin(w, r, nil, authTypeLogin)
}

func (app *app) loginAdd(w http.ResponseWriter, r *http.Request) {
	// check if already logged in
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	// add character to account
	app.doLogin(w, r, nil, authTypeAddCharacter)
}

// director login
func (app *app) auth(w http.ResponseWriter, r *http.Request) {
	// check if already logged in
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// create rows with refresh token
	app.doLogin(w, r, []string{
		"esi-corporations.read_blueprints.v1",
		"esi-industry.read_corporation_jobs.v1",
	}, authTypeAddScopes)
}
