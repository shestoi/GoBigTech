-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notification_inbox_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ,
    order_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    partition INT NOT NULL,
    message_offset BIGINT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_inbox_events_order_id ON notification_inbox_events(order_id);
CREATE INDEX IF NOT EXISTS idx_notification_inbox_events_topic ON notification_inbox_events(topic);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notification_inbox_events_topic;
DROP INDEX IF EXISTS idx_notification_inbox_events_order_id;
DROP TABLE IF EXISTS notification_inbox_events;
-- +goose StatementEnd

