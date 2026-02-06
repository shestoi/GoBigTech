package observability

import (
	"google.golang.org/grpc/metadata"
)

// metadataCarrier адаптирует metadata.MD к propagation.TextMapCarrier
type metadataCarrier struct {
	md metadata.MD
}

// NewMetadataCarrier создаёт carrier для gRPC metadata (incoming или outgoing)
func NewMetadataCarrier(md metadata.MD) *metadataCarrier {
	if md == nil {
		md = metadata.MD{}
	}
	return &metadataCarrier{md: md}
}

// Get возвращает значение по ключу (keys в gRPC metadata lowercase)
func (c *metadataCarrier) Get(key string) string {
	vals := c.md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Set устанавливает пару key-value
func (c *metadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

// Keys возвращает все ключи (для Inject — обычно не нужны, для Extract — все ключи metadata)
func (c *metadataCarrier) Keys() []string {
	out := make([]string, 0, len(c.md))
	for k := range c.md {
		out = append(out, k)
	}
	return out
}
