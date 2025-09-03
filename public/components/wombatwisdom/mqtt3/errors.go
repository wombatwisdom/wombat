package mqtt3

import (
	"errors"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
)

func errorAdapter(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, spec.ErrNotConnected):
		return service.ErrNotConnected
	case errors.Is(err, spec.ErrNoData):
		// MQTT is a persistent connection, no data is temporary
		// This shouldn't normally happen, but if it does, treat as temporary
		return nil
	case errors.Is(err, spec.ErrAlreadyConnected):
		// Already connected is not an error for Benthos
		return nil
	}

	return err
}

func translateConnectError(err error) error {
	if err == nil {
		return nil
	}

	// Try specific translation first
	if translated := errorAdapter(err); translated != err {
		return translated
	}

	// For any connection failure during init, return ErrNotConnected
	// to trigger Benthos retry behavior
	return service.ErrNotConnected
}

func translateReadError(err error) error {
	if err == nil {
		return nil
	}

	if translated := errorAdapter(err); translated != err {
		return translated
	}

	// MQTT should never return ErrEndOfInput as it's a persistent connection
	// Any other error during read is treated as a connection issue
	return service.ErrNotConnected
}

func translateWriteError(err error) error {
	if err == nil {
		return nil
	}

	// Try specific translation first
	if translated := errorAdapter(err); translated != err {
		return translated
	}

	// Other write errors should be returned as-is for proper retry handling
	// This allows Benthos to handle retries based on the specific error type
	return err
}
