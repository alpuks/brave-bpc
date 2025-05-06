package main

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/antihax/goesi"
	"github.com/gorilla/sessions"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	authLevel_Unauthorized = iota
	authLevel_Authorized   = iota
	authLevel_Worker       = iota
	authLevel_Admin        = iota
)

// keys used in session.Values map
type (
	sessionLoginState  struct{} // oauth2 state
	sessionLoginSrc    struct{} // source url that we're logging in from
	sessionAuthType    struct{} // auth type new, add (character), scope
	sessionLoginScopes struct{} // scopes to grant
	sessionUserData    struct{} // user data after login
)

type user struct {
	UserId        int64
	CharacterId   int32
	Level         int
	CharacterName string
}

func (u *user) IsLoggedIn() bool {
	return u.UserId > 0
}

type authType string

const (
	authTypeLogin        authType = "login" // create a user if none exists (whitelist ok)
	authTypeAddCharacter authType = "add"   // link character to user
	authTypeAddScopes    authType = "scope" // add scopes to character
)

func (app *app) doLogin(w http.ResponseWriter, r *http.Request, esiScopes []string, authType authType) {
	logger := getLoggerFromContext(r.Context()).With(
		zap.String("auth_type", string(authType)),
		zap.Strings("scopes", esiScopes))

	s, _ := app.session.Get(r, cookieName)

	if code := r.URL.Query().Get("code"); code != "" {
		// if code is set, this is the callback state.
		app.callback(logger, w, r, s, code)
		return
	}

	b := make([]byte, 16)
	_, _ = crand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	s.Values[sessionLoginState{}] = state
	s.Values[sessionLoginSrc{}] = r.URL.Query().Get("src")
	s.Values[sessionAuthType{}] = string(authType)
	s.Values[sessionLoginScopes{}] = esiScopes

	if err := s.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ssoAuth := goesi.NewSSOAuthenticatorV2(
		&http.Client{Timeout: 10 * time.Second},
		app.runtimeConfig.appId,
		app.runtimeConfig.appSecret,
		app.runtimeConfig.appRedirect,
		esiScopes)

	url := ssoAuth.AuthorizeURL(state, true, esiScopes)
	http.Redirect(w, r, url, http.StatusFound)
}

type scopeSourcePair struct {
	scope string
	token oauth2.TokenSource
}

func (app *app) createTokens(tsps []scopeRefreshPair) []scopeSourcePair {
	var esiScopes []string
	for _, tsp := range tsps {
		esiScopes = append(esiScopes, tsp.scope)
	}

	ssoAuth := goesi.NewSSOAuthenticatorV2(
		&http.Client{Timeout: 10 * time.Second},
		app.runtimeConfig.appId,
		app.runtimeConfig.appSecret,
		app.runtimeConfig.appRedirect,
		esiScopes)

	var tokens []scopeSourcePair
	for _, tsp := range tsps {
		tok := ssoAuth.TokenSource(&oauth2.Token{
			RefreshToken: tsp.token,
		})
		tokens = append(tokens, scopeSourcePair{
			scope: tsp.scope,
			token: tok,
		})
	}

	return tokens
}

func (app *app) callback(logger *zap.Logger, w http.ResponseWriter, r *http.Request, s *sessions.Session, code string) {
	sessionStateValue := s.Values[sessionLoginState{}].(string)
	queryState := r.URL.Query().Get("state")
	delete(s.Values, sessionLoginState{})

	if sessionStateValue != queryState {
		// TODO: on a mismatch state there's a panic here?
		logger.Error("mismatch state",
			zap.String("sessionState", sessionStateValue),
			zap.String("queryState", queryState))
		delete(s.Values, sessionLoginState{})
		delete(s.Values, sessionLoginSrc{})
		http.Error(w, "mismatch state", http.StatusBadRequest)
		return
	}

	esiScopes := s.Values[sessionLoginScopes{}].([]string)
	delete(s.Values, sessionLoginScopes{})

	ssoAuth := goesi.NewSSOAuthenticatorV2(
		&http.Client{Timeout: 10 * time.Second},
		app.runtimeConfig.appId,
		app.runtimeConfig.appSecret,
		app.runtimeConfig.appRedirect,
		esiScopes)

	token, err := ssoAuth.TokenExchange(code)
	if err != nil {
		http.Error(w, "token exchange error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tokSrc := ssoAuth.TokenSource(token)
	v, err := ssoAuth.Verify(tokSrc)
	if err != nil {
		http.Error(w, "token verification failed", http.StatusInternalServerError)
		return
	}
	logger = logger.With(
		zap.Int32("character_id", v.CharacterID),
		zap.Strings("scopes", strings.Split(v.Scopes, " ")),
		zap.String("character_name", v.CharacterName))
	logger.Info("verified user")

	// esi get character corp and alliance
	esiCtx := context.WithValue(context.Background(), goesi.ContextOAuth2, tokSrc)
	charData, resp, err := app.esi.ESI.CharacterApi.GetCharactersCharacterId(esiCtx, v.CharacterID, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		logger.Error("error getting character data")
		http.Error(w, "error getting character data", http.StatusInternalServerError)
		return
	}

	if len(app.config.AllianceWhitelist) > 0 || len(app.config.CorporationWhitelist) > 0 {
		if !slices.Contains(app.config.AllianceWhitelist, charData.AllianceId) {
			if !slices.Contains(app.config.CorporationWhitelist, charData.CorporationId) {
				logger.Warn("character not in corp or alliance whitelist")
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}
		}
		// no alliance or corp whitelist has been set, let anyone login
	}

	var userId int64
	var toonId int64
	switch authType := authType(s.Values[sessionAuthType{}].(string)); authType {
	case authTypeLogin:
		userId = app.dao.getUserForCharacter(logger, v.CharacterID, v.CharacterOwnerHash)
		if userId == 0 {
			userId, toonId, err = app.dao.createUserWithCharacter(logger, v.CharacterID, v.CharacterOwnerHash)
			if err != nil {
				http.Error(w, "error creating user", http.StatusInternalServerError)
				return
			}
			logger.Debug("created user", zap.Int64("user_id", userId), zap.Int64("toon_id", toonId))
		}

	case authTypeAddCharacter, authTypeAddScopes:
		// we must already be logged in to add more characters
		user := app.getUserFromSession(r)
		if !user.IsLoggedIn() {
			logger.Warn("attempting to add character or scope without login", zap.String("auth_type", string(authType)))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userId = user.UserId

		toonId, _, _ = app.dao.findOrCreateToon(logger, userId, v.CharacterID, v.CharacterOwnerHash)
		if toonId == 0 {
			logger.Debug("error creating toon", zap.Int64("user_id", userId), zap.Int64("toon_id", toonId))
			http.Error(w, "error creating toon", http.StatusInternalServerError)
			return
		}

		if err = app.dao.addScopes(logger, userId, toonId, esiScopes, token, s); err != nil {
			http.Error(w, "error adding scope", http.StatusInternalServerError)
			return
		}
	}

	delete(s.Values, sessionAuthType{})

	// TODO: check user corp and set appropriately
	authLevel := authLevel_Authorized

	authData := user{
		UserId:        userId,
		Level:         authLevel,
		CharacterId:   v.CharacterID,
		CharacterName: v.CharacterName,
	}
	//s.Values[sessionUserId] = userId
	//s.Values[sessionLevel] = authLevel_Authorized
	if charData.CorporationId == app.config.AdminCorp {
		authData.Level = authLevel_Admin
	}
	s.Values[sessionUserData{}] = authData

	sourcePage := s.Values[sessionLoginSrc{}].(string)
	delete(s.Values, sessionLoginSrc{})

	if err = s.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: redirect using value stored in state
	http.Redirect(w, r, sourcePage, http.StatusFound)
}
