package nats

import (
	"context"
	"github.com/Jeffail/shutdown"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"sync"
	"time"
)

func natsOrderedJetStreamInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Version("3.46.0").
		Summary("Reads messages from NATS JetStream subjects.").
		Description(`
### Consuming Mirrored Streams

In the case where a stream being consumed is mirrored from a different JetStream domain the stream cannot be resolved from the subject name alone, and so the stream name as well as the subject (if applicable) must both be specified.

### Metadata

This input adds the following metadata fields to each message:

` + "```text" + `
- nats_subject
- nats_sequence_stream
- nats_sequence_consumer
- nats_num_delivered
- nats_num_pending
- nats_domain
- nats_timestamp_unix_nano
` + "```" + `

You can access these metadata fields using
[function interpolation](/docs/configuration/interpolation#bloblang-queries).

` + connectionNameDescription() + authDescription()).
		Fields(connectionHeadFields()...).
		Field(service.NewStringField("stream").
			Description("A stream to consume from")).
		Field(service.NewStringListField("subjects").
			Description("allows filtering messages from a stream by subject").
			Optional().
			Example("foo.bar.baz").Example("foo.*.baz").Example("foo.bar.*").Example("foo.>")).
		Field(service.NewStringAnnotatedEnumField("deliver", map[string]string{
			"all":              "Deliver all available messages.",
			"last":             "Deliver starting with the last published messages.",
			"new":              "Deliver starting from now, not taking into account any previous messages.",
			"last_per_subject": "Deliver starting with the last published message per subject.",
		}).
			Description("defines from which point to start delivering messages").
			Default("all").
			Advanced()).
		Field(service.NewDurationField("redeliver_delay").
			Description("defines how long to ask the server to wait to redeliver the message in case a message is already being processed for the subject.").
			Default("100ms").
			Advanced()).
		Field(service.NewDurationField("msg_wait").
			Description("defines how long to wait for a message to be received.").
			Default("0s").
			Advanced()).
		Field(service.NewStringAnnotatedEnumField("replay", map[string]string{
			"instant":  "messages are sent as fast as possible.",
			"original": "messages are sent in the same intervals in which they were stored on stream.",
		}).
			Description("defines the rate at which messages are sent to the consumer").
			Default("instant").
			Advanced()).
		Fields(connectionTailFields()...).
		Field(inputTracingDocs())
}

func init() {
	err := service.RegisterInput(
		"jetstream_ordered_stream", natsOrderedJetStreamInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			input, err := newOrderedJetStreamReaderFromConfig(conf, mgr)
			if err != nil {
				return nil, err
			}
			return conf.WrapInputExtractTracingSpanMapping("jetstream_ordered_stream", service.AutoRetryNacks(input))
		})
	if err != nil {
		panic(err)
	}
}

//------------------------------------------------------------------------------

type orderedJetStreamReader struct {
	connDetails    connectionDetails
	stream         string
	bind           bool
	msgWait        time.Duration
	redeliverDelay time.Duration

	cc  *jetstream.OrderedConsumerConfig
	log *service.Logger

	consumer jetstream.Consumer

	connMut  sync.Mutex
	natsConn *nats.Conn

	shutSig *shutdown.Signaller

	// The pool caller id. This is a unique identifier we will provide when calling methods on the pool. This is used by
	// the pool to do reference counting and ensure that connections are only closed when they are no longer in use.
	pcid string

	msgProcDuration *service.MetricTimer
}

func newOrderedJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*orderedJetStreamReader, error) {
	j := orderedJetStreamReader{
		log:             mgr.Logger(),
		shutSig:         shutdown.NewSignaller(),
		msgProcDuration: mgr.Metrics().NewTimer("msg_processing_duration_ms"),
	}

	var err error
	if j.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
		return nil, err
	}

	j.cc = &jetstream.OrderedConsumerConfig{}

	if j.stream, err = conf.FieldString("stream"); err != nil {
		return nil, err
	}

	if conf.Contains("subjects") {
		if j.cc.FilterSubjects, err = conf.FieldStringList("subjects"); err != nil {
			return nil, err
		}
	}

	if j.cc.DeliverPolicy, err = parseDeliverPolicy(conf, "deliver"); err != nil {
		return nil, err
	}

	if j.msgWait, err = conf.FieldDuration("msg_wait"); err != nil {
		return nil, err
	}

	if j.redeliverDelay, err = conf.FieldDuration("redeliver_delay"); err != nil {
		return nil, err
	}

	if j.cc.ReplayPolicy, err = parseReplayPolicy(conf, "replay"); err != nil {
		return nil, err
	}

	return &j, nil
}

//------------------------------------------------------------------------------

func (j *orderedJetStreamReader) Connect(ctx context.Context) (err error) {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		return nil
	}

	if j.shutSig.IsSoftStopSignalled() {
		j.shutSig.TriggerHasStopped()
		return service.ErrEndOfInput
	}

	// -- get the nats connection from the pool
	var nc *nats.Conn
	defer func() {
		if err != nil {
			if nc != nil {
				_ = pool.Release(j.pcid, j.connDetails)
			}
		}
	}()
	if nc, err = pool.Get(ctx, j.pcid, j.connDetails); err != nil {
		return err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}

	var consumer jetstream.Consumer

	if consumer, err = js.OrderedConsumer(ctx, j.stream, *j.cc); err != nil {
		return err
	}
	j.consumer = consumer

	return nil
}

func (j *orderedJetStreamReader) disconnect() {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		if err := pool.Release(j.pcid, j.connDetails); err != nil {
			j.log.Errorf("Failed to release NATS connection: %v", err)
		}

		j.natsConn = nil
	}
}

func (j *orderedJetStreamReader) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	errAck := func(ctx context.Context, err error) error { return err }
	if j.consumer == nil {
		return nil, errAck, service.ErrNotConnected
	}

	var err error
	var msg jetstream.Msg

	res, err := j.consumer.Fetch(1)
	if err != nil {
		return nil, errAck, err
	}
	select {
	case msg = <-res.Messages():
		if msg == nil {
			if res.Error() != nil {
				return nil, errAck, res.Error()
			}
			return nil, errAck, nats.ErrTimeout
		}
	case <-ctx.Done():
		return nil, errAck, ctx.Err()
	}

	acker := Acker(msg, j.msgProcDuration)
	m, err := convertMessage(msg)
	if err != nil {
		return nil, acker, err
	}

	return m, acker, nil
}

func (j *orderedJetStreamReader) Close(ctx context.Context) error {
	go func() {
		j.disconnect()
		j.shutSig.TriggerHasStopped()
	}()
	select {
	case <-j.shutSig.HasStoppedChan():
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
