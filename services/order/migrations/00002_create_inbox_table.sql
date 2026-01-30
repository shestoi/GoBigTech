-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_inbox_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ,
    order_id TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_inbox_events_order_id ON order_inbox_events(order_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_inbox_events_order_id;
DROP TABLE IF EXISTS order_inbox_events;
-- +goose StatementEnd

