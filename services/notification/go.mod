module github.com/shestoi/GoBigTech/services/notification

go 1.24.2

require (
	github.com/jackc/pgx/v5 v5.8.0
	github.com/segmentio/kafka-go v0.4.50
	github.com/shestoi/GoBigTech/platform v0.0.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace github.com/shestoi/GoBigTech/platform => ../../platform
