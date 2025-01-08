package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/antihax/goesi"
	"github.com/gorilla/sessions"
	"go.uber.org/zap"
)

var loginAlliances = []int32{
	99003214, // Brave Collective
	99010079, // Brave United
}

var adminCorps = []int32{
	98445423, // Brave Industries
	98363855, // Nothing Industries
}

func (app *app) doLogin(w http.ResponseWriter, r *http.Request, esiScopes []string) {
	s, _ := app.session.Get(r, cookieName)

	if code := r.URL.Query().Get("code"); code != "" {
		// if code is set, this is the callback state.
		app.callback(w, r, s, code)
		return
	}

	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	s.Values["state"] = state
	s.Values["src"] = r.URL.Query().Get("src")

	if err := s.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ssoAuth := goesi.NewSSOAuthenticatorV2(
		&http.Client{Timeout: 10 * time.Second},
		os.Getenv(envAppId),
		os.Getenv(envAppSecret),
		os.Getenv(envAppRedirect),
		esiScopes)

	url := ssoAuth.AuthorizeURL(state, true, esiScopes)
	http.Redirect(w, r, url, http.StatusFound)
}

func (app *app) callback(w http.ResponseWriter, r *http.Request, s *sessions.Session, code string) {
	sessionState := s.Values["state"].(string)
	queryState := r.URL.Query().Get("state")
	if sessionState != queryState {
		// TODO: on a mismatch state there's a panic here?
		app.logger.Error("mismatch state",
			zap.String("sessionState", sessionState),
			zap.String("queryState", queryState))
		delete(s.Values, "state")
		delete(s.Values, "src")
		http.Error(w, "mismatch state", http.StatusBadRequest)
		return
	}

	ssoAuth := goesi.NewSSOAuthenticatorV2(
		&http.Client{Timeout: 10 * time.Second},
		os.Getenv(envAppId),
		os.Getenv(envAppSecret),
		os.Getenv(envAppRedirect),
		strings.Split(s.Values["roles"].(string), " "))

	token, err := ssoAuth.TokenExchange(code)
	if err != nil {
		http.Error(w, "token exchange error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tokSrc := ssoAuth.TokenSource(token)
	app.esiCtx = context.WithValue(context.Background(), goesi.ContextOAuth2, tokSrc)

	v, err := ssoAuth.Verify(tokSrc)
	if err != nil {
		http.Error(w, "token verification failed", http.StatusInternalServerError)
		return
	}
	app.logger.Info("verified", zap.String("character", v.CharacterName))

	// esi get character corp and alliance
	val, resp, err := app.esi.ESI.CharacterApi.GetCharactersCharacterId(app.esiCtx, v.CharacterID, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		app.logger.Error("error getting character data")
		http.Error(w, "error getting character data", http.StatusInternalServerError)
		return
	}

	if !slices.Contains(loginAlliances, val.AllianceId) {
		app.logger.Warn("character not in alliance whitelist",
			zap.Int32("character_id", v.CharacterID),
			zap.String("character_name", v.CharacterName))
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	if slices.Contains(adminCorps, val.CorporationId) {
		// TODO: assign auth level
	}

	if err = s.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}
