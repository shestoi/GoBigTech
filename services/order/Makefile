.PHONY: migrate-up migrate-down migrate-status migrate-create

# Database connection string
# Using 127.0.0.1 instead of localhost to avoid IPv6 resolution issues
DSN ?= postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable

# Goose binary path (will use go run if not installed)
GOOSE = go run github.com/pressly/goose/v3/cmd/goose@latest

# Migrate up (apply all pending migrations)
migrate-up:
	@echo "Running migrations up..."
	$(GOOSE) -dir migrations postgres "$(DSN)" up

# Migrate down (rollback last migration)
migrate-down:
	@echo "Rolling back last migration..."
	$(GOOSE) -dir migrations postgres "$(DSN)" down

# Migrate status (show migration status)
migrate-status:
	@echo "Migration status:"
	$(GOOSE) -dir migrations postgres "$(DSN)" status

# Create new migration (usage: make migrate-create NAME=create_something)
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=create_something"; \
		exit 1; \
	fi
	$(GOOSE) -dir migrations create $(NAME) sql

