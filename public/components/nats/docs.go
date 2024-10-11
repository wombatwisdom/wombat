package nats

import (
	"github.com/redpanda-data/benthos/v4/public/service"
)

func connectionNameDescription() string {
	return `## Connection name

When monitoring and managing a production NATS system, it is often useful to
know which connection a message was send/received from. This can be achieved by
setting the connection name option when creating a NATS connection.

Wombat will automatically set the connection name based off the label of the given
NATS component, so that monitoring tools between NATS and Wombat can stay in sync.
`
}

func inputTracingDocs() *service.ConfigField {
	return service.NewExtractTracingSpanMappingField()
}
func outputTracingDocs() *service.ConfigField {
	return service.NewInjectTracingSpanMappingField()
}
