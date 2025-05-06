package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	esiOauthMetadata = "https://login.eveonline.com/.well-known/oauth-authorization-server"
	jwtClaimScp      = "scp"
	jwtClaimName     = "name"
	jwtClaimSub      = "sub"
	jwtClaimOwner    = "owner"
)

func fetchEsiWellKnown(logger *zap.Logger) (string, string) {
	var (
		err           error
		js            []byte
		resp          *http.Response
		client        = &http.Client{Timeout: 10 * time.Second}
		oauthMetadata = make(map[string]interface{})
	)

	if resp, err = client.Get(esiOauthMetadata); err != nil {
		logger.Fatal("error fetching oauth well-known uri", zap.Error(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Fatal("oauth metadata status not ok", zap.String("status", resp.Status))
	}

	if js, err = io.ReadAll(resp.Body); err != nil {
		logger.Fatal("error reading response body", zap.Error(err))
	}

	if err = json.Unmarshal(js, &oauthMetadata); err != nil {
		logger.Fatal("error unmarshalling body", zap.Error(err))
	}

	jwksUri, ok := oauthMetadata["jwks_uri"].(string)
	if !ok {
		logger.Fatal("jwks_uri is not a string")
	}

	issuer, ok := oauthMetadata["issuer"].(string)
	if !ok {
		logger.Fatal("issuer is not a string")
	}

	return issuer, jwksUri
}

func newJwksCache(logger *zap.Logger, jwksUri string) (*jwk.Cache, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	jwksCache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		cancel()
		logger.Fatal("error creating new jwk.Cache", zap.Error(err))
	}

	if err = jwksCache.Register(ctx, jwksUri,
		jwk.WithMinInterval(time.Hour),
		jwk.WithMaxInterval(time.Hour*24*7)); err != nil {
		cancel()
		logger.Fatal("error registering jwksUri in cache", zap.Error(err), zap.String("jwks_uri", jwksUri))
	}

	return jwksCache, cancel
}

func (app *app) verifyTokenAndClaims(logger *zap.Logger, accessToken *oauth2.Token) *TokenClaims {
	jwks, err := app.jwksCache.Lookup(context.Background(), app.runtimeConfig.jwksUri)
	if err != nil {
		logger.Error("", zap.Error(err))
	}

	tok, err := jwt.Parse([]byte(accessToken.AccessToken),
		jwt.WithKeySet(jwks),
		jwt.WithIssuer(app.runtimeConfig.oauthIssuer),
		jwt.WithAudience(app.runtimeConfig.appId),
		jwt.WithAudience("EVE Online"),
	)
	if err != nil {
		logger.Error("", zap.Error(err))
		//httpError(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	logger.Debug("token", zap.Any("token", tok))

	var (
		scopeClaims []interface{}
		sub         string
		name        string
		ownerHash   string
	)

	if err = tok.Get(jwtClaimSub, &sub); err != nil {
		logger.Error("error getting claim", zap.Error(err))
	}

	if err = tok.Get(jwtClaimName, &name); err != nil {
		logger.Error("error getting claim", zap.Error(err))
	}

	if err = tok.Get(jwtClaimOwner, &ownerHash); err != nil {
		logger.Error("error getting claim", zap.Error(err))
	}

	_ = tok.Get(jwtClaimScp, &scopeClaims) // scopes can be missing
	scopes := make([]string, len(scopeClaims))
	for i, scope := range scopeClaims {
		scopes[i] = scope.(string)
	}

	subSplit := strings.Split(sub, ":")
	subId, _ := strconv.ParseInt(subSplit[len(subSplit)-1], 10, 32)

	return &TokenClaims{
		Scopes:      scopes,
		CharacterId: int32(subId),
		Name:        name,
		OwnerHash:   ownerHash,
	}
}

type TokenClaims struct {
	// audience   []string
	// expiration time.Time
	// issuedAt   time.Time
	// issuer     string
	// jwtId      string
	// notBefore  time.Time
	// subject    string // CHARACTER:EVE:<charid>

	// private claims
	// kid    string // jwks key id
	// azp    string // idk, it's appid though.
	// tenant string // tranquility
	// tier   string // live
	// region string // world (or china?)
	Name      string
	OwnerHash string

	// extracted fields
	Scopes      []string // set from scp
	CharacterId int32    // extracted from subject
}
