package ibmmq

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
		// MQTT is a persistent connection
		return nil // nil -> continue processing. service.ErrNoData -> stop processing
	case errors.Is(err, spec.ErrAlreadyConnected):
		// Already connected is not an error for Benthos
		return nil
	}

	return err
}

func translateError(err error) error {
	if err == nil {
		return nil
	}

	if translated := errorAdapter(err); !errors.Is(translated, err) {
		return translated
	}

	return err
}
