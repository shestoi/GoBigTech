-- +goose Up
-- +goose StatementBegin
-- Добавляем колонки для статуса обработки (идемпотентность с retry)
ALTER TABLE notification_inbox_events
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'sent',
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Для новых записей default при INSERT — pending (существующие уже получили sent выше)
ALTER TABLE notification_inbox_events ALTER COLUMN status SET DEFAULT 'pending';

CREATE INDEX IF NOT EXISTS idx_notification_inbox_events_status ON notification_inbox_events(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notification_inbox_events_status;
ALTER TABLE notification_inbox_events
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS updated_at;
-- +goose StatementEnd
