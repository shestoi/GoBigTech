# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy entire repository (needed for go.work to see all modules)
COPY . .

WORKDIR /app/services/payment

# Download dependencies
RUN go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /payment ./cmd/payment

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=builder /payment .

EXPOSE 50052

ENV APP_ENV=docker

ENTRYPOINT ["./payment"]
