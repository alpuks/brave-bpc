-- +goose Up
CREATE TABLE config(
	updated_at DATETIME NOT NULL DEFAULT NOW(),
	updated_by VARCHAR(64) NOT NULL DEFAULT 'unknown',
	config JSON NOT NULL, -- We'll only have one row ever
	PRIMARY KEY(updated_at)
);

INSERT INTO config VALUES (NOW(), 'initial', '{}');

-- +goose Down
DROP TABLE config;
