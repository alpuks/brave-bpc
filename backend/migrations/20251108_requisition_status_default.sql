-- +goose Up
ALTER TABLE requisition_order MODIFY COLUMN requisition_status TINYINT NOT NULL DEFAULT 1;
UPDATE requisition_order SET requisition_status = requisition_status + 1;

-- +goose Down
UPDATE requisition_order SET requisition_status = requisition_status - 1;
ALTER TABLE requisition_order MODIFY COLUMN requisition_status TINYINT NOT NULL DEFAULT 0;
