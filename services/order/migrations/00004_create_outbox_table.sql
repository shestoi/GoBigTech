-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_outbox_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    aggregate_id TEXT NOT NULL, -- order_id
    payload JSONB NOT NULL,
    topic TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, sent, failed
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_order_outbox_events_status ON order_outbox_events(status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_order_outbox_events_aggregate_id ON order_outbox_events(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_order_outbox_events_created_at ON order_outbox_events(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_order_outbox_events_created_at;
DROP INDEX IF EXISTS idx_order_outbox_events_aggregate_id;
DROP INDEX IF EXISTS idx_order_outbox_events_status;
DROP TABLE IF EXISTS order_outbox_events;
-- +goose StatementEnd

