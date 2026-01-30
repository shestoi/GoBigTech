module github.com/shestoi/GoBigTech/cmd/kafka-playground

go 1.24.2

require (
	github.com/segmentio/kafka-go v0.4.47
	github.com/shestoi/GoBigTech/platform v0.0.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/caarlos0/env/v10 v10.0.0 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)

replace github.com/shestoi/GoBigTech/platform => ../../platform
