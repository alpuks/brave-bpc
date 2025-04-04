-- +goose Up
CREATE TABLE requisition_order(
	id                 BIGINT AUTO_INCREMENT NOT NULL,
	character_id       INTEGER NOT NULL,
	requisition_status TINYINT NOT NULL DEFAULT 0,
	created_at         DATETIME NOT NULL DEFAULT NOW(),
	updated_at         DATETIME NOT NULL DEFAULT NOW(),
	updated_by         VARCHAR(64) NOT NULL,
	blueprints         JSON NOT NULL,
	notes              TEXT,
	PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE requisition_order;
