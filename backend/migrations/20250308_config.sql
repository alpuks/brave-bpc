-- +goose Up
CREATE TABLE config(
	config JSON -- We'll only have one row ever
);

INSERT INTO config VALUES ('{}');

-- +goose Down
DROP TABLE config;
