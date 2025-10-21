package ibmmq

import (
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
)

func init() {
	// Register IBM MQ input with ww_ prefix for seamless integration
	err := service.RegisterBatchInput(
		"ww_ibm_mq",
		inputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
			input, err := newInput(conf, mgr)
			return input, err
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq input: %w", err))
	}

	// Register IBM MQ output with ww_ prefix for seamless integration
	err = service.RegisterBatchOutput(
		"ww_ibm_mq",
		outputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (out service.BatchOutput, batchPolicy service.BatchPolicy, maxInFlight int, err error) {
			return newOutput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq output: %w", err))
	}
}
