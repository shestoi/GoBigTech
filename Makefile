.PHONY: help test test-unit test-integration test-e2e build
.PHONY: kafka-up kafka-down kafka-reset kafka-topics kafka-topics-list kafka-topics-create
.PHONY: kafka-producer kafka-consumer kafka-consume-payment kafka-consume-assembly kafka-consume-dlq
.PHONY: obs-up obs-down jaeger
.PHONY: app-up app-down stack-up stack-down all-up all-down

# ---- Help ----
help:
	@echo ""
	@echo "Available commands:"
	@echo "  make test             Run all unit tests"
	@echo "  make test-unit        Run unit tests only"
	@echo "  make test-integration Run integration tests (Postgres)"
	@echo "  make test-e2e         Run e2e tests (Mongo + gRPC)"
	@echo "  make build            Build all services"
	@echo ""
	@echo "Kafka commands:"
	@echo "  make kafka-up              Start Kafka (docker compose up -d)"
	@echo "  make kafka-down            Stop Kafka (docker compose down)"
	@echo "  make kafka-reset           Stop Kafka and remove volumes, then start fresh"
	@echo "  make kafka-topics-list     List all Kafka topics"
	@echo "  make kafka-topics-create   Create domain topics (order.payment.completed, order.assembly.completed, notification.dlq)"
	@echo "  make kafka-producer        Open console producer for test-topic"
	@echo "  make kafka-consumer        Open console consumer for test-topic (from beginning)"
	@echo "  make kafka-consume-payment  Open console consumer for order.payment.completed (from beginning)"
	@echo "  make kafka-consume-assembly Open console consumer for order.assembly.completed (from beginning)"
	@echo "  make kafka-consume-dlq      Open console consumer for order.payment.completed.dlq (from beginning)"
	@echo ""
	@echo "Service commands:"
	@echo "  make notification-run       Run Notification service (APP_ENV=local)"
	@echo ""
	@echo "Observability commands:"
	@echo "  make obs-up                 Start observability stack (otel-collector, jaeger, prometheus, alertmanager, grafana, elasticsearch, kibana, filebeat)"
	@echo "  make obs-down               Stop observability stack"
	@echo "  make jaeger                 Print Jaeger UI URL (see docs/OBSERVABILITY.md)"
	@echo ""
	@echo "Docker mode commands:"
	@echo "  make app-up                 Start app services in docker (iam, inventory, payment, order, assembly, notification)"
	@echo "  make app-down               Stop app services"
	@echo "  make stack-up               Start infrastructure + observability (kafka + obs stack)"
	@echo "  make stack-down             Stop infrastructure + observability"
	@echo "  make all-up                 Start everything (stack + app services)"
	@echo "  make all-down               Stop everything"
	@echo ""

# ---- Tests ----
test: test-unit

test-unit:
	go test ./...

test-integration:
	go test -tags=integration ./... -v -timeout 5m

test-e2e:
	go test -tags=e2e ./... -v -timeout 5m

# ---- Build ----
build:
	go build ./services/order/cmd/order
	go build ./services/inventory/cmd/inventory
	go build ./services/payment/cmd/payment
	go build ./services/iam/cmd/iam

# ---- Kafka ----
kafka-up:
	docker compose -f docker-compose.kafka.yml up -d

kafka-down:
	docker compose -f docker-compose.kafka.yml down

kafka-reset:
	docker compose -f docker-compose.kafka.yml down -v
	docker compose -f docker-compose.kafka.yml up -d

kafka-topics:
	@echo "Use 'make kafka-topics-list' instead"
	@make kafka-topics-list

kafka-create-topics:
	@echo "Creating Kafka topics..."
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic order.payment.completed --partitions 1 --replication-factor 1 --if-not-exists || true
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic order.assembly.completed --partitions 1 --replication-factor 1 --if-not-exists || true
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic notification.dlq --partitions 1 --replication-factor 1 --if-not-exists || true
	@echo "Topics created successfully"

kafka-topics-create:
	@echo "Creating Kafka topics..."
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic order.payment.completed --partitions 1 --replication-factor 1 --if-not-exists || true
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic order.assembly.completed --partitions 1 --replication-factor 1 --if-not-exists || true
	@docker exec gobigtech-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --create --topic notification.dlq --partitions 1 --replication-factor 1 --if-not-exists || true
	@echo "Topics created successfully"

kafka-producer:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic test-topic

kafka-consumer:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic test-topic --from-beginning

kafka-consume-payment:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic order.payment.completed --from-beginning

kafka-consume-assembly:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic order.assembly.completed --from-beginning

kafka-consume-dlq:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic notification.dlq --from-beginning

# ---- Kafka topics management ----
# kafka-topics-list уже определён выше, не дублируем

kafka-tail-payment:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic order.payment.completed --from-beginning

kafka-tail-assembly:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic order.assembly.completed --from-beginning

kafka-tail-notification-dlq:
	docker exec -it gobigtech-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic notification.dlq --from-beginning

# ---- Migrations ----
migrate-up-order:
	cd services/order && goose -dir migrations postgres "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable" up

migrate-up-notification:
	cd services/notification && go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "postgres://order_user:order_password@127.0.0.1:15432/orders?sslmode=disable" up

migrate-up-iam:
	cd services/iam && goose -dir migrations postgres "postgres://iam_user:iam_password@127.0.0.1:15433/iam?sslmode=disable" up

# ---- Services ----
order-run:
	cd services/order && APP_ENV=local go run ./cmd/order

assembly-run:
	cd services/assembly && APP_ENV=local go run ./cmd/assembly

notification-run:
	cd services/notification && APP_ENV=local go run ./cmd/notification

iam-run:
	cd services/iam && APP_ENV=local go run ./cmd/iam

inventory-run:
	cd services/inventory && APP_ENV=local go run ./cmd/inventory

# ---- Observability stack ----
OBS_SERVICES = otel-collector jaeger prometheus alertmanager grafana elasticsearch kibana filebeat

obs-up:
	docker compose up -d $(OBS_SERVICES)

obs-down:
	docker compose stop $(OBS_SERVICES)

jaeger:
	@echo "Jaeger UI: http://127.0.0.1:16686 (see docs/OBSERVABILITY.md)"

# ---- Docker mode: app services ----
app-up:
	docker compose up -d iam inventory payment order assembly notification

app-down:
	docker compose stop iam inventory payment order assembly notification

# ---- Docker mode: infrastructure + observability ----
stack-up:
	make kafka-up
	make obs-up

stack-down:
	make obs-down
	make kafka-down

# ---- Docker mode: everything ----
all-up:
	make stack-up
	make app-up

all-down:
	make app-down
	make stack-down

# Генерация заказов для проверки алерта >10 заказов/мин (нужны SESSION_ID и USER_ID)
gen-orders:
	@if [ -z "$$SESSION_ID" ] || [ -z "$$USER_ID" ]; then echo "Usage: SESSION_ID=xxx USER_ID=yyy make gen-orders"; exit 1; fi; \
	for i in 1 2 3 4 5 6 7 8 9 10 11; do \
	  curl -s -X POST http://127.0.0.1:8080/orders -H "Content-Type: application/json" -H "x-session-id: $$SESSION_ID" -d "{\"user_id\":\"$$USER_ID\",\"items\":[{\"product_id\":\"p$$i\",\"quantity\":1}]}" && echo " order $$i" || echo " order $$i failed"; \
	  sleep 5; \
	done