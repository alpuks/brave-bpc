-- +goose Up
CREATE TABLE user (
	id BIGINT AUTO_INCREMENT,
	primary_toon_hash CHAR(32),
	auth_level TINYINT,
	date_created DATETIME NOT NULL DEFAULT NOW(),
	date_modified DATETIME NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	UNIQUE (primary_toon_hash)
);

CREATE TABLE toon (
	id BIGINT AUTO_INCREMENT,
	user_id BIGINT NOT NULL,       -- our user id
	character_id INTEGER NOT NULL, -- eve character id
	owner_hash CHAR(32) NOT NULL,  -- esi owner hash
	date_created DATETIME NOT NULL DEFAULT NOW(),
	date_modified DATETIME NOT NULL DEFAULT NOW(),
	PRIMARY KEY (character_id, owner_hash),
	UNIQUE (id),
	FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE -- deleted when user.id deleted
);

CREATE TABLE token (
	id BIGINT AUTO_INCREMENT,
	toon_id BIGINT,
	refresh_token TEXT,
	date_created DATETIME NOT NULL DEFAULT NOW(),
	date_modified DATETIME NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	FOREIGN KEY (toon_id) REFERENCES toon(id) ON DELETE CASCADE -- deleted when toon.id deleted
);

CREATE TABLE scope (
	id BIGINT AUTO_INCREMENT,
	user_id BIGINT,
	toon_id INTEGER,
	token_id BIGINT,
	scope CHAR(64),
	date_created DATETIME NOT NULL DEFAULT NOW(),
	date_modified DATETIME NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	UNIQUE (toon_id, scope),
	FOREIGN KEY (token_id) REFERENCES token(id) ON DELETE CASCADE -- deleted when token.id deleted
);

-- +goose Down
DROP TABLE scope;
DROP TABLE token;
DROP TABLE toon;
DROP TABLE user;
