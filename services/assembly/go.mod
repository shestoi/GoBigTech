module github.com/shestoi/GoBigTech/services/assembly

go 1.24.2

require (
	github.com/google/uuid v1.6.0
	github.com/segmentio/kafka-go v0.4.50
	github.com/shestoi/GoBigTech/platform v0.0.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/shestoi/GoBigTech/platform => ../../platform
