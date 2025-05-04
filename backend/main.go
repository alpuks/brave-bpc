package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlHeamer/brave-bpc/glue"
	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/antihax/goesi/optional"
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
	eveWellKnown *EveOnlineWellKnownOauthAuthServer
	appId        string
	appSecret    string
	appRedirect  string
	environment  string
	migrateDown  string
	httpPort     string
}

type app struct {
	config        *appConfig
	runtimeConfig *runtimeConfig
	logger        *zap.Logger
	dao           *dao
	session       *sessions.CookieStore
	esi           *goesi.APIClient
	bpos          *syncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList]
	bpcs          *syncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList]
	//groupMap         map[int32]esi.GetMarketsGroupsMarketGroupIdOk
	//groupTree        map[int32][]int32
	requisitionLocks *syncMap[int64, int32]
	flake            *snowflake.Node
	jwks             *EsiJwks
}

func main() {
	var err error
	logger := newDefaultLogger()
	defer logger.Sync()

	app := &app{
		logger:           logger,
		session:          newCookieStore(),
		esi:              goesi.NewAPIClient(&http.Client{Timeout: 10 * time.Second}, esiUserAgent),
		flake:            newSnowflake(logger),
		bpos:             newSyncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList](),
		bpcs:             newSyncMap[int32, esi.GetCorporationsCorporationIdBlueprints200OkList](),
		requisitionLocks: newSyncMap[int64, int32](),
	}

	app.loadEnv() // load .env file into os env
	app.runtimeConfig = &runtimeConfig{
		appId:       os.Getenv(envAppId),
		appSecret:   os.Getenv(envAppSecret),
		appRedirect: os.Getenv(envAppRedirect),
		environment: os.Getenv(envEnvironment),
		migrateDown: os.Getenv(envMigrateDown),
		httpPort:    getEnvWithDefault(envHttpPort, "2727"),
	}
	app.runtimeConfig.eveWellKnown, err = FetchEsiWellKnown()
	if err != nil {
		logger.Fatal("fetch esi well-known", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.jwks, err = NewEsiJwks(ctx,
		app.runtimeConfig.eveWellKnown.JwksUri,
		jwk.WithMinInterval(time.Hour),
		jwk.WithMaxInterval(time.Hour*24*7))
	if err != nil {
		logger.Fatal("error creating jwks cache", zap.Error(err))
	}

	app.dao = newDao(logger)
	defer app.dao.db.Close()
	app.dao.runMigrations(logger)

	// TODO: Remove appConfig param
	app.config, err = app.dao.loadAppConfig(&appConfig{
		AllianceWhitelist: []int32{
			99003214, // Brave Collective
			99010079, // Brave United
		},
		CorporationWhitelist: []int32{
			98445423, // Brave Industries
			98363855, // Nothing Industries
			98544197, // Valor Shipyards
		},
		AdminCorp:      98544197,
		AdminCharacter: 95154016,
		MaxContracts:   2,
	})
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

	logger.Info("http service listening", zap.String("port", app.runtimeConfig.httpPort))
	server := &http.Server{
		Addr:         ":" + app.runtimeConfig.httpPort,
		Handler:      mux,
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			logger.Error("error serving http", zap.Error(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan
	logger.Info("received os interrupt signal, shutting down gracefully")

	done <- struct{}{}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	err = server.Shutdown(shutdownCtx)
	logger.Info("shutdown complete", zap.Error(err))
}

func (app *app) ticker(done <-chan struct{}) {
	logger := app.logger.Named("ticker")
	time.Sleep(3 * time.Second)
	//app.buildMarketTree()
	bpos, bpcs, err := app.updateBlueprintInventory(logger)
	ticker := time.NewTicker(60 * time.Minute)
	if err != nil {
		logger.Error("error updating blueprint inventory", zap.Error(err))
	} else {
		app.bpos.Overwrite(bpos)
		app.bpcs.Overwrite(bpcs)
	}
	for {
		select {
		case <-done:
			logger.Info("exiting ticker loop")
			return

		case <-ticker.C:
			// 1 minute jitter window
			switch app.runtimeConfig.environment {
			case "prod", "production":
				time.Sleep(time.Duration(rand.IntN(60)) * time.Second)
			}
			bpos, bpcs, err = app.updateBlueprintInventory(logger)
			if err != nil {
				if errors.Is(err, errErrorsExceeded) {
					time.Sleep(5 * time.Minute)
					ticker.Reset(60 * time.Minute)
				}
			} else {
				app.bpos.Overwrite(bpos)
				app.bpcs.Overwrite(bpcs)
			}
		}
	}
}

func sameBlueprintQuality(a esi.GetCorporationsCorporationIdBlueprints200Ok, b esi.GetCorporationsCorporationIdBlueprints200Ok) bool {
	return a.TypeId == b.TypeId &&
		a.MaterialEfficiency == b.MaterialEfficiency &&
		a.TimeEfficiency == b.TimeEfficiency &&
		a.Runs == b.Runs
}

// filter location before running through this func
func coalesceBlueprints(blueprints esi.GetCorporationsCorporationIdBlueprints200OkList) (
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
) {
	bpos := make(map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList)
	bpcs := make(map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList)

	for _, bp := range blueprints {
		var (
			m   map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList
			qty int32 = 1
		)
		switch bp.Quantity {
		case -2: // BPC
			m = bpcs
		case -1: // researched BPO
			m = bpos
		default: // BPO stack > 0
			m = bpos
			qty = bp.Quantity
		}

		idx := -1
		if _, ok := m[bp.TypeId]; !ok {
			m[bp.TypeId] = esi.GetCorporationsCorporationIdBlueprints200OkList{}
		} else {
			idx = slices.IndexFunc(m[bp.TypeId], func(e esi.GetCorporationsCorporationIdBlueprints200Ok) bool {
				return sameBlueprintQuality(e, bp)
			})
		}

		if idx == -1 { // equivalent blueprint not found
			b := bp
			b.Quantity = qty
			m[bp.TypeId] = append(m[bp.TypeId], b)
		} else {
			m[bp.TypeId][idx].Quantity += qty
		}
	}

	return bpos, bpcs
}

func (app *app) updateBlueprintInventory(logger *zap.Logger) (
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	error,
) {
	tsps := app.dao.getTokenForCharacter(logger, app.config.AdminCharacter, []string{string(glue.EsiScope_CorporationsReadBlueprints_v1)})
	toks := app.createTokens(tsps)
	if len(toks) == 0 {
		return nil, nil, errors.New("error creating esi token")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	esiCtx := context.WithValue(ctx, goesi.ContextOAuth2, toks[0].token)

	//tok, _ := toks[0].token.Token()
	blueprints, resp, err := app.esi.ESI.CorporationApi.GetCorporationsCorporationIdBlueprints(esiCtx, app.config.AdminCorp,
		&esi.GetCorporationsCorporationIdBlueprintsOpts{
			// IfNoneMatch: etag,
			Page: optional.NewInt32(1),
		})
	if err != nil {
		logger.Error("error fetching blueprints", zap.Error(err))
		return nil, nil, fmt.Errorf("could not fetch first page of blueprints: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK && resp.StatusCode >= http.StatusBadRequest {
		// some other error happened. print to logs
		var buf []byte
		resp.Body.Read(buf)
		logger.Debug("status not 200", zap.String("status_code", resp.Status), zap.String("body", string(buf)))
	}

	pages, _ := strconv.ParseInt(resp.Header.Get(headerPages), 10, 64)
	for page := int32(2); page <= int32(pages); page++ {
		var bp esi.GetCorporationsCorporationIdBlueprints200OkList
		bp, resp, err = app.esi.ESI.CorporationApi.GetCorporationsCorporationIdBlueprints(esiCtx, app.config.AdminCorp,
			&esi.GetCorporationsCorporationIdBlueprintsOpts{
				Page: optional.NewInt32(page),
			})

		if err != nil {
			logger.Warn("error fetching blueprints", zap.Error(err))
			//return nil, nil, fmt.Errorf("could not fetch first page of blueprints: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < http.StatusOK && resp.StatusCode >= http.StatusBadRequest {
			// some other error happened. print to logs
			var buf []byte
			resp.Body.Read(buf)
			logger.Warn("status not 200", zap.String("status_code", resp.Status), zap.String("body", string(buf)))
		}

		blueprints = append(blueprints, bp...)
	}

	// TODO: filter blueprint locations
	bpos, bpcs := coalesceBlueprints(blueprints)
	locations := buildItemLocationMap(blueprints)
	_ = locations

	logger.Debug("successfully fetched blueprints")
	return bpos, bpcs, nil
}

func buildItemLocationMap(blueprints esi.GetCorporationsCorporationIdBlueprints200OkList) map[int64]esi.GetCorporationsCorporationIdBlueprints200OkList {
	locations := make(map[int64]esi.GetCorporationsCorporationIdBlueprints200OkList)
	for _, bp := range blueprints {
		if _, ok := locations[bp.LocationId]; !ok {
			locations[bp.LocationId] = esi.GetCorporationsCorporationIdBlueprints200OkList{bp}
			continue
		}
		locations[bp.LocationId] = append(locations[bp.LocationId], bp)
	}
	return locations
}

func (app *app) root(w http.ResponseWriter, r *http.Request) {
	logger := getLoggerFromContext(r.Context())
	logger.Info("root")
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
` + fmt.Sprintf("%v", user) + `
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
	app.doLogin(w, r, nil, authTypeLogin)
}

func (app *app) addCharToAccount(w http.ResponseWriter, r *http.Request) {
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
func (app *app) addScopeToAccount(w http.ResponseWriter, r *http.Request) {
	// check if already logged in
	s, _ := app.session.Get(r, cookieName)
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
	logger.Info("printConfig")
	body := `
<html>
<body>
` + fmt.Sprintf("%+v", *app.config) + `
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
