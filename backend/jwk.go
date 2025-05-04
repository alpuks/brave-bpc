package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	esiOauthMetadata = "https://login.eveonline.com/.well-known/oauth-authorization-server"
	jwtClaimScp      = "scp"   // scopes
	jwtClaimName     = "name"  // character name
	jwtClaimSubject  = "sub"   // subject (EVE:CHARACTER:<id>)
	jwtClaimOwner    = "owner" // owner hash
	oauthIssuer      = "issuer"
)

type EsiJwks struct {
	appId     string
	wellKnown *EveOnlineWellKnownOauthAuthServer
	cache     *jwk.Cache
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
	// azp    string // authorized parties
	// tenant string // tranquility
	// tier   string // live
	// region string // world (or china?)
	Name      string
	OwnerHash string

	// extracted fields
	Scopes      []string // set from scp
	CharacterId int32    // extracted from subject
}

type EveOnlineWellKnownOauthAuthServer struct {
	Issuer                                     string   `json:"issuer,omitempty"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint,omitempty"`
	ResponseTypesSupported                     []string `json:"response_types_supported,omitempty"`
	JwksUri                                    string   `json:"jwks_uri,omitempty"`
	RevocationEndpoint                         string   `json:"revocation_endpoint,omitempty"`
	SubjectTypesSupported                      []string `json:"subject_types_supported,omitempty"`
	RevocationEndpointAuthMethodsSupported     []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	IdTokenSigningAlgValuesSupported           []string `json:"id_token_signing_alg_values_supported,omitempty"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported,omitempty"`
}

// ctx should have a cancelfunc that the host app controls.
func NewEsiJwks(ctx context.Context, appId string, regOpts ...jwk.RegisterOption) (*EsiJwks, error) {
	wellKnown, err := FetchEsiWellKnown()
	if err != nil {
		return nil, err
	}

	cache, err := NewJwksCache(ctx, wellKnown.JwksUri, regOpts...)
	if err != nil {
		return nil, err
	}

	return &EsiJwks{
		appId:     appId,
		wellKnown: wellKnown,
		cache:     cache,
	}, nil
}

func FetchEsiWellKnown() (*EveOnlineWellKnownOauthAuthServer, error) {
	var (
		err           error
		js            []byte
		resp          *http.Response
		client        = &http.Client{Timeout: 10 * time.Second}
		oauthMetadata = &EveOnlineWellKnownOauthAuthServer{}
	)

	if resp, err = client.Get(esiOauthMetadata); err != nil {
		return nil, fmt.Errorf("http get error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get not ok: %s", resp.Status)
	}

	if js, err = io.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}

	if err = json.Unmarshal(js, &oauthMetadata); err != nil {
		return nil, fmt.Errorf("error unmarshalling body: %w", err)
	}

	return oauthMetadata, nil
}

// ctx should have a cancelfunc for the user to hold on to
func NewJwksCache(ctx context.Context, jwksUri string, regOpts ...jwk.RegisterOption) (*jwk.Cache, error) {
	jwksCache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("error creating new jwk.Cache: %w", err)
	}

	if err = jwksCache.Register(ctx, jwksUri, regOpts...); err != nil {
		return nil, fmt.Errorf("error registering jwskUri in cache: %w", err)
	}

	return jwksCache, nil
}

func (j *EsiJwks) VerifyTokenClaims(
	ctx context.Context,
	accessToken []byte,
	parseOpts ...jwt.ParseOption,
) (*TokenClaims, error) {
	keySet, err := j.cache.Lookup(ctx, j.wellKnown.JwksUri)
	if err != nil {
		return nil, err
	}

	parseOpts = append(parseOpts,
		jwt.WithKeySet(keySet),
		jwt.WithIssuer(j.wellKnown.Issuer),
		jwt.WithAudience(j.appId),
		jwt.WithAudience("EVE Online"))
	tok, err := jwt.Parse(accessToken, parseOpts...)
	if err != nil {
		return nil, err
	}

	var (
		scopeClaims   []interface{}
		subject       string // "EVE:CHARACTER:<id>"
		characterName string
		ownerHash     string
	)

	if err = tok.Get(jwtClaimSubject, &subject); err != nil {
		return nil, err
	}

	if err = tok.Get(jwtClaimName, &characterName); err != nil {
		return nil, err
	}

	if err = tok.Get(jwtClaimOwner, &ownerHash); err != nil {
		return nil, err
	}

	_ = tok.Get(jwtClaimScp, &scopeClaims) // scopes can be missing
	scopes := make([]string, len(scopeClaims))
	for i, scope := range scopeClaims {
		scopes[i] = scope.(string)
	}

	return &TokenClaims{
		Scopes:      scopes,
		CharacterId: extractCharacterIdFromSubject(subject),
		Name:        characterName,
		OwnerHash:   ownerHash,
	}, nil
}

func extractCharacterIdFromSubject(subject string) int32 {
	subId, _ := strconv.ParseInt(subject[strings.LastIndex(subject, ":")+1:], 10, 32)
	return int32(subId)
}
