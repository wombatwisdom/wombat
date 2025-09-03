package mqtt3

import (
	"fmt"
	"github.com/redpanda-data/benthos/v4/public/service"
)

func init() {
	// Register MQTT input with ww_ prefix for seamless integration
	err := service.RegisterBatchInput(
		"ww_mqtt_3",
		inputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
			return newInput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_mqtt_3 input: %w", err))
	}

	// Register MQTT output with ww_ prefix for seamless integration
	err = service.RegisterBatchOutput(
		"ww_mqtt_3",
		outputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (out service.BatchOutput, batchPolicy service.BatchPolicy, maxInFlight int, err error) {
			return newOutput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_mqtt_3 output: %w", err))
	}
}
