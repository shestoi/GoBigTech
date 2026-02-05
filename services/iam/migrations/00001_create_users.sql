-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; -- расширение для генерации UUID

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- id пользователя
    login TEXT UNIQUE NOT NULL, -- логин пользователя
    password_hash TEXT NOT NULL, -- хеш пароля пользователя
    telegram_id TEXT NULL, -- id пользователя в Telegram
    created_at TIMESTAMPTZ NOT NULL DEFAULT now() -- время создания пользователя
);

CREATE INDEX IF NOT EXISTS idx_users_login ON users(login); -- индекс для логина пользователя
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at); -- индекс для времени создания пользователя
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_created_at;
DROP INDEX IF EXISTS idx_users_login;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
