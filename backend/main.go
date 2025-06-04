package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AlHeamer/brave-bpc/glue"
	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/bwmarrin/snowflake"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	esiUserAgent = "brave-bpc/0.0.0 (eve:Al Heamer)"

	headerPages       = "X-Pages"
	headerErrorRemain = "X-Esi-Error-Limit-Remain" // errors remaining this window
	headerErrorReset  = "X-Esi-Error-Limit-Reset"  // seconds until the next error window
	esiRequestTimeout = 20 * time.Second
)

var (
	errErrorsExceeded = errors.New("too many errors")
)

type appConfig struct {
	AllianceWhitelist    []int32 `json:"alliances,omitempty"`     // Alliances allowed to log into this service
	CorporationWhitelist []int32 `json:"corporations,omitempty"`  // Corporations allowed to log into this service
	AdminCorp            int32   `json:"admin_corp,omitempty"`    // Corporation that provides the service
	AdminCharacter       int32   `json:"admin_char,omitempty"`    // The character used to poll corporate data
	MaxContracts         int32   `json:"max_contracts,omitempty"` // Maximum number of contracts an account can open
}

type runtimeConfig struct {
	appId       string
	appSecret   string
	appRedirect string
	environment string
	migrateDown string
	httpPort    string
	jwtSkew     time.Duration
}

type app struct {
	config        *appConfig
	runtimeConfig *runtimeConfig
	logger        *zap.Logger
	dao           *dao
	sessionStore  sessions.Store
	esi           *goesi.APIClient
	bpos          *syncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList]
	bpcs          *syncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList]
	typeNameCache *syncMap[int32, string]
	//groupMap         map[int32]esi.GetMarketsGroupsMarketGroupIdOk
	//groupTree        map[int32][]int32
	requisitionLocks *syncMap[int64, int32]
	flake            *snowflake.Node
	jwks             *EsiJwks
	refreshToken     chan struct{}
}

func main() {
	runtimeConfig, err := loadEnv()
	logger := newDefaultLogger(runtimeConfig.environment)
	defer logger.Sync()

	if errors.Is(err, fs.ErrNotExist) {
		logger.Info("ignoring missing .env file", zap.Error(err))
	} else if errors.Is(err, errEnvFile) {
		err = err.(interface{ Unwrap() []error }).Unwrap()[1]
		logger.Error("failed to open .env file", zap.Error(err))
	} else if err != nil {
		logger.Fatal("failed to parse .env file", zap.Error(err))
	}

	app := &app{
		logger:           logger,
		sessionStore:     newSessionStore(),
		esi:              goesi.NewAPIClient(&http.Client{Timeout: 10 * time.Second}, esiUserAgent),
		flake:            newSnowflake(logger),
		bpos:             newSyncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList](),
		bpcs:             newSyncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList](),
		typeNameCache:    newSyncMap[int32, string](),
		requisitionLocks: newSyncMap[int64, int32](),
		runtimeConfig:    runtimeConfig,
		refreshToken:     make(chan struct{}, 1),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.jwks, err = NewEsiJwks(ctx,
		app.runtimeConfig.appId,
		app.runtimeConfig.jwtSkew,
		jwk.WithMinInterval(time.Hour),
		jwk.WithMaxInterval(time.Hour*24*7))
	if err != nil {
		logger.Fatal("error creating jwks cache", zap.Error(err))
	}

	app.dao = newDao(logger)
	defer app.dao.db.Close()
	app.dao.runMigrations(logger, len(app.runtimeConfig.migrateDown) > 0)

	app.config, err = app.dao.loadAppConfig()
	if err != nil {
		logger.Fatal("failed to load config from db", zap.Error(err))
	}

	gob.Register(user{})
	gob.Register(sessionAuthType{})
	gob.Register(sessionLoginScopes{})
	gob.Register(sessionLoginSrc{})
	gob.Register(sessionLoginState{})
	gob.Register(sessionUserData{})

	mux := http.NewServeMux()
	baseChain := NewMwChain(app.requestMiddleware)
	chain := baseChain.Add(app.authMiddlewareFactory(authLevel_Unauthorized))
	mux.Handle("/metrics", chain.Handle(promhttp.Handler()))
	mux.Handle("/", chain.HandleFunc(app.root))
	mux.Handle("/login", chain.HandleFunc(app.login))
	mux.Handle("/login/char", chain.HandleFunc(app.addCharToAccount))
	mux.Handle("/login/scope", chain.HandleFunc(app.addScopeToAccount))
	mux.Handle("/config", chain.HandleFunc(app.printConfig))

	app.createApiHandlers(mux, baseChain)

	done := make(chan struct{})
	go app.ticker(done)

	server := &http.Server{
		Addr:         ":" + app.runtimeConfig.httpPort,
		Handler:      mux,
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("http service listening",
			zap.String("port", app.runtimeConfig.httpPort),
			zap.String("environment", app.runtimeConfig.environment))

		if err := server.ListenAndServe(); err != nil {
			logger.Error("error serving http", zap.Error(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("received os interrupt signal, shutting down gracefully")

	done <- struct{}{}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	err = server.Shutdown(shutdownCtx)
	logger.Info("shutdown complete", zap.Error(err))
}

func (app *app) root(w http.ResponseWriter, r *http.Request) {
	logger := getLoggerFromContext(r.Context())
	logger.Debug("root")
	user := app.getUserFromSession(r)
	var bp strings.Builder
	keys := app.bpcs.Keys()

	if user.IsLoggedIn() && len(keys) > 0 {
		names, resp, err := app.esi.ESI.UniverseApi.PostUniverseNames(context.Background(), keys, nil)
		if err != nil {
			logger.Error("error fetching type names", zap.Error(err), zap.Int32s("type_ids", keys))
			http.Error(w, "error fetching type names", http.StatusInternalServerError)
			return
		}
		if resp.StatusCode != http.StatusOK {
			logger.Error("error fetching type names", zap.String("status", resp.Status))
			http.Error(w, "error fetching type names status no ok", http.StatusInternalServerError)
			return
		}

		nameMap := func() map[int32]string {
			nm := make(map[int32]string, len(names))
			for _, v := range names {
				if v.Category == string(glue.NameCategory_InventoryType) {
					nm[v.Id] = v.Name
				}
			}
			return nm
		}()

		app.bpcs.RangeFunc(func(typeId int32, list esi.GetCorporationsCorporationIdBlueprints200OkList) {
			bp.WriteString("<details><summary>" + nameMap[typeId] + "</summary><p>")
			for _, v := range list {
				bp.WriteString(fmt.Sprintf("<div>ME: %d / TE: %d / Runs: %d / Quantity: %d</div>", v.MaterialEfficiency, v.TimeEfficiency, v.Runs, v.Quantity))
			}
			bp.WriteString("</p></details>")
		})
	}

	body := `
<html>
<body>
` + fmt.Sprintf("%+v", user) + `
<ul>
<li><a href="/login">login</a>
<li><a href="/login/char">add character</a>
<li><a href="/login/scope">add scopes</a>
<li><a href="/config">config</a>
<li><a href="/metrics">metrics</a>
</ul>
` + bp.String() + `
</body>
</html>
`
	w.Write([]byte(body))
}

// standard login
func (app *app) login(w http.ResponseWriter, r *http.Request) {
	logger := getLoggerFromContext(r.Context())
	logger.Debug("login")
	app.doLogin(w, r, nil, authTypeLogin)
}

func (app *app) addCharToAccount(w http.ResponseWriter, r *http.Request) {
	// check if already logged in
	logger := getLoggerFromContext(r.Context())
	logger.Debug("add char to account")
	s, _ := app.sessionStore.Get(r, cookieSession)
	if s.IsNew {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	// add character to account
	app.doLogin(w, r, nil, authTypeAddCharacter)
}

// director login
func (app *app) addScopeToAccount(w http.ResponseWriter, r *http.Request) {
	// check if already logged in
	logger := getLoggerFromContext(r.Context())
	logger.Debug("add scope to account")
	s, _ := app.sessionStore.Get(r, cookieSession)
	if s.IsNew {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// create rows with refresh token
	app.doLogin(w, r, []string{
		string(glue.EsiScope_AssetsReadCorporationAssets_v1),
		string(glue.EsiScope_CorporationsReadBlueprints_v1),
		string(glue.EsiScope_IndustryReadCorporationJobs_v1),
	}, authTypeAddScopes)
}

// print out the app config data
func (app *app) printConfig(w http.ResponseWriter, r *http.Request) {
	logger := getLoggerFromContext(r.Context())
	logger.Debug("printConfig")
	body := `
<html>
<body>
` + fmt.Sprintf("%+v<br/>%+v", *app.config, *app.runtimeConfig) + `
<ul>
<li><a href="/login">login</a>
<li><a href="/login/char">add character</a>
<li><a href="/login/scope">add scopes</a>
<li><a href="/config">config</a>
<li><a href="/metrics">metrics</a>
</ul>
</body>
</html>
`
	w.Write([]byte(body))
}
