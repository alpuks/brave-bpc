package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/AlHeamer/brave-bpc/sqlparams"
	"github.com/gorilla/sessions"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type scopeRefreshPair struct {
	token string
	scope string
}

type dao struct {
	db *sql.DB
}

func newDao(db *sql.DB) *dao {
	return &dao{
		db: db,
	}
}

func (d *dao) loadAppConfig(logger *zap.Logger, config *appConfig) *appConfig {
	var strConfig []byte
	if err := d.db.QueryRow("SELECT config FROM config LIMIT 1").Scan(&strConfig); err != nil {
		logger.Error("failed to read config from db", zap.Error(err))
		return nil
	}
	conf := &appConfig{
		AllianceWhitelist:    slices.Clone(config.AllianceWhitelist),
		CorporationWhitelist: slices.Clone(config.CorporationWhitelist),
		AdminCorp:            config.AdminCorp,
		AdminCharacter:       config.AdminCharacter,
		MaxContracts:         config.MaxContracts,
	}
	json.Unmarshal(strConfig, conf)
	return conf
}

func (d *dao) getTokenForCharacter(logger *zap.Logger, characterId int32, roles []string) []scopeRefreshPair {
	logger = logger.With(zap.Int32("character_id", characterId), zap.Strings("roles", roles))

	params := sqlparams.New()
	rows, err := d.db.Query(`
SELECT s.scope, t.refresh_token
FROM scope s
	LEFT JOIN token t
		ON t.toon_id = s.toon_id
	LEFT JOIN toon o
		ON o.id = s.toon_id
WHERE o.character_id = `+params.AddParam(characterId)+`
AND s.scope IN(`+params.AddParams(roles)+`)
`, params...)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("could not fetch refresh token", zap.Error(err))
		return nil
	}

	var tsps []scopeRefreshPair
	for rows.Next() {
		var tsp scopeRefreshPair
		err := rows.Scan(&tsp.scope, &tsp.token)
		if err != nil {
			logger.Error("error scanning row", zap.Error(err))
		}
		tsps = append(tsps, tsp)
	}

	if rows.Err() != nil {
		logger.Error("error parsing rows", zap.Error(err))
		return nil
	}

	return tsps
}

// userid, found, created
func (d *dao) getUserForCharacter(logger *zap.Logger, characterId int32, ownerHash string) int64 {
	logger = logger.With(zap.Int32("character_id", characterId), zap.String("owner_hash", ownerHash))

	var userId int64
	err := d.db.QueryRow(`
SELECT user_id
FROM toon
WHERE
character_id = ? AND
owner_hash = ?
`, characterId, ownerHash).Scan(&userId)

	if err != nil && err != sql.ErrNoRows {
		logger.Error("error finding user", zap.Error(err))
	}

	return userId
}

func (d *dao) createUserWithCharacter(logger *zap.Logger, characterId int32, ownerHash string) (int64, int64, error) {
	row, err := d.db.Exec(`
INSERT INTO user
(primary_toon_hash, date_created, date_modified)
VALUES(?, NOW(), NOW())
`, ownerHash)

	if err != nil {
		logger.Error("error creating new user", zap.Error(err))
		return 0, 0, err
	}

	var userId int64
	if userId, err = row.LastInsertId(); err != nil {
		logger.Error("error getting id when creating new user", zap.Error(err))
		return 0, 0, err
	}

	toonId, _, _ := d.findOrCreateToon(logger, userId, characterId, ownerHash)

	return userId, toonId, nil
}

func (d *dao) findOrCreateToon(logger *zap.Logger, userId int64, characterId int32, ownerHash string) (int64, bool, bool) {
	if userId <= 0 {
		return 0, false, false
	}

	logger = logger.With(zap.Int64("user_id", userId), zap.Int32("character_id", characterId), zap.String("owner_hash", ownerHash))
	var toonId int64
	err := d.db.QueryRow(`
SELECT id
FROM toon
WHERE character_id=? AND
owner_hash=? AND
user_id=?
`, characterId, ownerHash, userId).Scan(&toonId)

	if err == nil {
		return toonId, true, false
	} else if err != sql.ErrNoRows {
		logger.Error("error getting toon", zap.Error(err))
		return 0, false, false
	}

	res, err := d.db.Exec(`
INSERT INTO toon
(user_id, character_id, owner_hash)
VALUES(?, ?, ?)
`, userId, characterId, ownerHash)
	if err != nil {
		logger.Error("error creating toon", zap.Error(err))
		return 0, false, false
	}

	if toonId, err = res.LastInsertId(); err != nil {
		logger.Error("couldn't get toon_id", zap.Error(err))
		return 0, false, false
	}

	return toonId, false, true
}

func (d *dao) addScopes(logger *zap.Logger, userId int64, toonId int64, scopes []string, token *oauth2.Token, session *sessions.Session) error {
	if len(scopes) == 0 {
		// no need to store a token if there's no scopes
		return nil
	}
	slices.Sort(scopes)

	// TODO: check if this toon already has the scope, and remove it or replace it.
	var tx *sql.Tx
	var err error
	var res sql.Result
	var rows *sql.Rows
	params := sqlparams.New()
	query := `
SELECT scope
FROM scope
WHERE
user_id = ` + params.AddParam(userId) + ` AND
toon_id = ` + params.AddParam(toonId) + ` AND
scope IN(` + params.AddParams(scopes) + `)
`
	rows, err = d.db.Query(query, params...)

	if err != nil && err != sql.ErrNoRows {
		logger.Error("error fetching existing roles", zap.Error(err))
		return err
	}

	foundScopes := make([]string, 0, len(scopes))
	for rows.Next() {
		var scope string
		if err = rows.Scan(&scope); err != nil {
			logger.Error("error scanning row", zap.Error(err))
			continue
		}
		foundScopes = append(foundScopes, scope)
	}

	if err = rows.Err(); err != nil {
		logger.Error("error iterating rows", zap.Error(err))
	}

	slices.Sort(foundScopes)
	if len(foundScopes) == len(scopes) {
		// no need to save, all scopes already exist
		session.Values[sessionScopes] = foundScopes
		return nil
	}

	tx, err = d.db.Begin()
	if err != nil {
		logger.Error("error starting transaction", zap.Error(err))
		return err
	}

	params = sqlparams.New()
	res, err = tx.Exec(`
INSERT INTO token (toon_id, refresh_token)
VALUES (`+params.AddParams(toonId, token.RefreshToken)+`)
`, params...)
	if err != nil {
		logger.Error("error inserting toon", zap.Error(err))
		return err
	}

	var tokenId int64
	if tokenId, err = res.LastInsertId(); err != nil {
		logger.Error("error getting toon id", zap.Error(err))
		return err
	}

	params = sqlparams.New()
	scopeValues := make([]string, len(scopes))
	for i, scope := range scopes {
		scopeValues[i] = fmt.Sprintf("(%s)", params.AddParams(userId, toonId, tokenId, scope))
	}

	_, err = tx.Exec(`
INSERT INTO scope (user_id, toon_id, token_id, scope)
VALUES`+strings.Join(scopeValues, ","), params...)
	if err != nil {
		logger.Error("error inserting scopes", zap.Error(err))
		return err
	}

	if err = tx.Commit(); err != nil {
		logger.Error("error committing transaction", zap.Error(err))
		return err
	}

	sessionScopes, found := session.Values[sessionScopes]
	if !found {
		sessionScopes = make([]string, len(scopes))
	}
	sessionScopes = append(sessionScopes.([]string), scopes...)
	session.Values[sessionScopes] = sessionScopes

	return nil
}

func (d *dao) runMigrations(logger *zap.Logger) {
	var err error
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(zap.NewStdLog(logger))

	if err = goose.SetDialect("mysql"); err != nil {
		logger.Fatal("unable to setup db for migrations", zap.Error(err))
	}
	if os.Getenv("MIGRATE_DOWN") != "" {
		logger.Debug("migrate db down")
		if err = goose.Down(d.db, "migrations"); err != nil {
			logger.Warn("unable to down migrations", zap.Error(err))
		}
	}
	logger.Debug("migrate db up")
	if err = goose.Up(d.db, "migrations"); err != nil {
		logger.Fatal("unable to up migrations", zap.Error(err))
	}
}

func (dao *dao) createRequisition(characterId int32, blueprints []byte) error {
	var err error
	params := sqlparams.New()
	_, err = dao.db.Exec(`
INSERT INTO requisition_order
(character_id, blueprints)
VALUES (`+params.AddParams(characterId, blueprints)+`)
`, params...)
	return err
}

func (dao *dao) updateRequisition() error {
	var err error
	return err
}
