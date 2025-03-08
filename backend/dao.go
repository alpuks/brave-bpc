package main

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type dao struct {
	db *sql.DB
}

func newDao(db *sql.DB) *dao {
	return &dao{
		db: db,
	}
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

// annoying workaround for mysql driver.
// can't pass a slice to IN(?), must be IN(?,?..n)
type sqlParams []any

func newSqlParams() *sqlParams {
	return &sqlParams{}
}

func sqlParamsAdd(p *sqlParams, param any) string {
	*p = append(*p, param)
	return "?"
}

func sqlAddVariadic(p *sqlParams, params ...any) string {
	var paramCount int
	for _, param := range params {
		switch v := reflect.ValueOf(param); v.Kind() {
		case reflect.Slice:
			if v.Len() == 0 {
				continue
			}
			paramCount += v.Len()
			*p = slices.Grow(*p, v.Len())

			switch v.Index(0).Kind() {
			case reflect.String:
				for i := range v.Len() {
					*p = append(*p, v.Index(i).Interface().(string))
				}
			}
		default:
			paramCount++
			*p = append(*p, param)
		}
	}
	return "?" + strings.Repeat(",?", paramCount-1)
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
	params := newSqlParams()
	query := `
SELECT scope
FROM scope
WHERE
user_id = ` + sqlParamsAdd(params, userId) + ` AND
toon_id = ` + sqlParamsAdd(params, toonId) + ` AND
scope IN(` + sqlAddVariadic(params, scopes) + `)
`
	rows, err = d.db.Query(query, *params...)

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

	params = newSqlParams()
	res, err = tx.Exec(`
INSERT INTO token (toon_id, refresh_token)
VALUES (`+sqlAddVariadic(params, toonId, token.RefreshToken)+`)`, *params...)
	if err != nil {
		logger.Error("error inserting toon", zap.Error(err))
		return err
	}

	var tokenId int64
	if tokenId, err = res.LastInsertId(); err != nil {
		logger.Error("error getting toon id", zap.Error(err))
		return err
	}

	params = newSqlParams()
	scopeValues := make([]string, len(scopes))
	for i, scope := range scopes {
		scopeValues[i] = fmt.Sprintf("(%s)", sqlAddVariadic(params, userId, toonId, tokenId, scope))
	}

	_, err = tx.Exec(`
INSERT INTO scope (user_id, toon_id, token_id, scope)
VALUES`+strings.Join(scopeValues, ","), *params...)
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
