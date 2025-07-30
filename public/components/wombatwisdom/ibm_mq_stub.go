//go:build !mqclient

// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
)

func init() {
	// Register stub IBM MQ input that explains the build requirement
	err := service.RegisterInput(
		"ww_ibm_mq",
		wwIBMMQStubConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return nil, fmt.Errorf("IBM MQ component requires CGO and IBM MQ client libraries. Build with: go build -tags mqclient")
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq stub input: %w", err))
	}

	// Register stub IBM MQ output that explains the build requirement  
	err = service.RegisterOutput(
		"ww_ibm_mq",
		wwIBMMQStubConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
			return nil, 0, fmt.Errorf("IBM MQ component requires CGO and IBM MQ client libraries. Build with: go build -tags mqclient")
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq stub output: %w", err))
	}
}

func wwIBMMQStubConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Integration").
		Summary("IBM MQ integration (requires CGO and MQ client libraries)").
		Description(`
IBM MQ integration using wombatwisdom components.

## Build Requirements

This component requires CGO and the IBM MQ client libraries to be installed:

1. Install IBM MQ client libraries from IBM
2. Build with the mqclient tag: go build -tags mqclient

## Without MQ Client Libraries

When IBM MQ client libraries are not available, this component will fail to initialize
with an error message explaining the build requirements.

## Features (when properly built)

- Transactional message processing with SYNCPOINT
- Batch processing for improved throughput  
- Multiple parallel connections for high-volume queues
- Full IBM MQ metadata extraction and formatting
- Support for CCSID, encoding, and format configuration
`).
		Field(service.NewStringField("queue_name").
			Description("IBM MQ queue name (requires mqclient build tag)")).
		Field(service.NewStringField("system_name").
			Description("IBM MQ system resource name").
			Default("default"))
}