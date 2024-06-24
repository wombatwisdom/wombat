package nats

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
)

func subjectsByStreamAndFilters(ctx context.Context, js jetstream.JetStream, stream string, filters []string) ([]string, error) {
	str, err := js.Stream(ctx, stream)
	if err != nil {
		return nil, err
	}

	sm := map[string]uint64{}
	for _, filter := range filters {
		i, err := str.Info(ctx, jetstream.WithSubjectFilter(filter))
		if err != nil {
			return nil, fmt.Errorf("unable to get stream info for %s.%s", stream, filter)
		}

		for sub, cnt := range i.State.Subjects {
			sm[sub] = cnt
		}
	}

	subjects := make([]string, 0, len(sm))
	cnt := uint64(0)
	idx := 0
	for sub, smc := range sm {
		cnt += smc
		subjects[idx] = sub
		idx++
	}

	return subjects, nil
}
