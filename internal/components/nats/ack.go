package nats

import (
	"context"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"time"
)

func Acker(msg jetstream.Msg, timer *service.MetricTimer) service.AckFunc {
	start := time.Now().UnixMilli()
	return func(ctx context.Context, err error) error {
		defer func() {
			timer.Timing(time.Now().UnixMilli() - start)
		}()

		return err
		//if err != nil {
		//	return msg.Nak()
		//}
		//
		//return msg.Ack()

	}
}

type Ackers struct {
	ackers []service.AckFunc
}

func (a *Ackers) Add(acker service.AckFunc) {
	a.ackers = append(a.ackers, acker)
}

func (a *Ackers) Ack(ctx context.Context, err error) error {
	for _, acker := range a.ackers {
		_ = acker(ctx, err)
	}
	return nil
}
