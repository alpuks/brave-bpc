-- +goose Up
CREATE TABLE config(
	updated_at DATETIME NOT NULL DEFAULT NOW(),
	updated_by VARCHAR(64) NOT NULL DEFAULT 'unknown',
	config JSON NOT NULL, -- We'll only have one row ever
	PRIMARY KEY(updated_at)
);

INSERT INTO config VALUES (NOW(), 'initial', '{
"alliances": [99003214],
"corporations": [],
"admin_corp": 98544197,
"admin_char": 95154016,
"max_contracts": 2
}');

-- +goose Down
DROP TABLE config;
