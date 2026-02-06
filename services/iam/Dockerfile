# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy entire repository (needed for go.work to see all modules)
COPY . .

WORKDIR /app/services/iam

# Download dependencies
RUN go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /iam ./cmd/iam

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=builder /iam .
COPY --from=builder /app/services/iam/migrations ./migrations

EXPOSE 50053

ENV APP_ENV=docker

ENTRYPOINT ["./iam"]
