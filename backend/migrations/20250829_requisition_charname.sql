-- +goose Up
ALTER TABLE requisition_order ADD character_name VARCHAR(64) NOT NULL;

-- +goose Down
ALTER TABLE requisition_order DROP COLUMN character_name;
