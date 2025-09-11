package nats

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/nats-io/nats.go"

	"github.com/Jeffail/shutdown"

	"github.com/redpanda-data/benthos/v4/public/service"
)

func natsJetStreamInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Reads messages from NATS JetStream subjects.").
		Description(`
## Consume mirrored streams

In the case where a stream being consumed is mirrored from a different JetStream domain the stream cannot be resolved 
from the subject name alone, and so the stream name as well as the subject (if applicable) must both be specified.

## Metadata

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

` + connectionNameDescription() + authDescription()).
		Fields(connectionHeadFields()...).
		Field(service.NewStringField("name").Description("The name of the consumer")).
		Field(service.NewBoolField("bind").
			Description("Indicates whether the input should use an existing consumer").
			Default(false).
			Optional()).
		Field(service.NewStringField("stream").
			Description("The stream to consume from")).
		Field(service.NewStringListField("filter_subjects").
			Description("The list of subjects to filter on").
			Optional().
			Example("foo.bar.baz").Example("foo.*.baz").Example("foo.bar.*").Example("foo.>")).
		Field(service.NewStringAnnotatedEnumField("deliver", map[string]string{
			"all":               "Deliver all available messages.",
			"last":              "Deliver starting with the last published messages.",
			"last_per_subject":  "Deliver starting with the last published message per subject.",
			"new":               "Deliver starting from now, not taking into account any previous messages.",
			"by_start_time":     "Deliver starting with messages published after a specific time.",
			"by_start_sequence": "Deliver starting with messages published after a specific sequence number.",
		}).
			Description("Determines which messages to deliver when consuming without a durable subscriber.").
			Default("all")).
		Field(service.NewIntField("start_sequence").Description("The start sequence when using the `by_start_seqyuence` deliver policy").Optional()).
		Field(service.NewStringField("start_time").Description("The start time when using the `by_start_time` deliver policy. Should be in RFC3339 format").Optional()).
		Field(service.NewStringField("ack_wait").
			Description("The maximum amount of time NATS server should wait for an ack from consumer.").
			Advanced().
			Default("30s").
			Example("100ms").
			Example("5m")).
		Field(service.NewIntField("max_ack_pending").
			Description("The maximum number of outstanding acks to be allowed before consuming is halted.").
			Advanced().
			Default(1024)).
		Field(service.NewIntField("batch_size").
			Description("The maximum number of messages to consume in a single batch").
			Default(1)).
		Field(service.NewDurationField("flush_wait").
			Description("the amount of time to wait for a new message before closing an incomplete batch").
			Default("100ms")).
		Field(service.NewDurationField("nak_delay").
			Description("The amount of time to wait before reattempting to deliver a message that was NAK'd").
			Optional().
			Advanced()).
		Fields(connectionTailFields()...).
		Field(inputTracingDocs())
}

func init() {
	err := service.RegisterBatchInput(
		"nats_jetstream_batched", natsJetStreamInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
			input, err := newJetStreamReaderFromConfig(conf, mgr)
			if err != nil {
				return nil, err
			}
			return conf.WrapBatchInputExtractTracingSpanMapping("nats_jetstream_batched", input)
		})
	if err != nil {
		panic(err)
	}
}

//------------------------------------------------------------------------------

type jetStreamReader struct {
	connDetails connectionDetails

	bind      bool
	stream    string
	cfg       jetstream.ConsumerConfig
	batchSize int
	flushWait time.Duration
	nakDelay  *time.Duration

	log *service.Logger

	connMut  sync.Mutex
	natsConn *nats.Conn
	consumer jetstream.Consumer

	shutSig *shutdown.Signaller
}

func newJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*jetStreamReader, error) {
	j := jetStreamReader{
		log:     mgr.Logger(),
		shutSig: shutdown.NewSignaller(),
		cfg:     jetstream.ConsumerConfig{},
	}

	var err error
	if j.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
		return nil, err
	}

	j.cfg.Name, err = conf.FieldString("name")
	if err != nil {
		return nil, err
	}

	j.bind, err = conf.FieldBool("bind")
	if err != nil {
		return nil, err
	}

	j.stream, err = conf.FieldString("stream")
	if err != nil {
		return nil, err
	}

	deliver, err := conf.FieldString("deliver")
	if err != nil {
		return nil, err
	}
	switch deliver {
	case "all":
		j.cfg.DeliverPolicy = jetstream.DeliverAllPolicy
	case "last":
		j.cfg.DeliverPolicy = jetstream.DeliverLastPolicy
	case "last_per_subject":
		j.cfg.DeliverPolicy = jetstream.DeliverLastPerSubjectPolicy
	case "new":
		j.cfg.DeliverPolicy = jetstream.DeliverNewPolicy
	case "by_start_time":
		j.cfg.DeliverPolicy = jetstream.DeliverByStartTimePolicy
		st, err := conf.FieldString("start_time")
		if err != nil {
			return nil, err
		}

		startTime, err := time.Parse(time.RFC3339, st)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start time: %v", err)
		}

		j.cfg.OptStartTime = &startTime

	case "by_start_sequence":
		j.cfg.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		// get the start sequence as well
		ss, err := conf.FieldInt("start_sequence")
		if err != nil {
			return nil, err
		}
		j.cfg.OptStartSeq = uint64(ss)
	default:
		return nil, fmt.Errorf("deliver option %v was not recognised", deliver)
	}

	subs, err := conf.FieldStringList("filter_subjects")
	if err != nil {
		return nil, err
	}
	if len(subs) > 1 {
		j.cfg.FilterSubjects = subs
	} else if len(subs) == 1 {
		j.cfg.FilterSubject = subs[0]
	}

	ackWaitStr, err := conf.FieldString("ack_wait")
	if err != nil {
		return nil, err
	}
	if ackWaitStr != "" {
		j.cfg.AckWait, err = time.ParseDuration(ackWaitStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ack wait duration: %v", err)
		}
	}

	if j.cfg.MaxAckPending, err = conf.FieldInt("max_ack_pending"); err != nil {
		return nil, err
	}

	if j.batchSize, err = conf.FieldInt("batch_size"); err != nil {
		return nil, err
	}

	if j.flushWait, err = conf.FieldDuration("flush_wait"); err != nil {
		return nil, err
	}

	if conf.Contains("nak_delay") {
		delay, err := conf.FieldDuration("nak_delay")
		if err != nil {
			return nil, err
		}
		j.nakDelay = &delay
	}

	return &j, nil
}

//------------------------------------------------------------------------------

func (j *jetStreamReader) Connect(ctx context.Context) (err error) {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		return nil
	}

	var natsConn *nats.Conn

	defer func() {
		if err != nil {
			if natsConn != nil {
				natsConn.Close()
			}
		}
	}()

	if natsConn, err = j.connDetails.get(ctx); err != nil {
		return err
	}

	jCtx, err := jetstream.New(natsConn)
	if err != nil {
		return err
	}

	if j.bind {
		j.consumer, err = jCtx.Consumer(ctx, j.stream, j.cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to bind consumer %q: %w", j.cfg.Name, err)
		}
	} else {
		j.consumer, err = jCtx.CreateOrUpdateConsumer(ctx, j.stream, j.cfg)
		if err != nil {
			return fmt.Errorf("failed to create consumer %q: %w", j.cfg.Name, err)
		}
	}

	j.natsConn = natsConn
	return nil
}

func (j *jetStreamReader) disconnect() {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	j.consumer = nil
	if j.natsConn != nil {
		j.natsConn.Close()
		j.natsConn = nil
	}
}

func (j *jetStreamReader) ReadBatch(ctx context.Context) (service.MessageBatch, service.AckFunc, error) {
	if j.consumer == nil {
		return nil, nil, service.ErrNotConnected
	}

	msgs, err := j.consumer.Fetch(j.batchSize, jetstream.FetchMaxWait(j.flushWait))
	if err != nil {
		//if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
		//	break
		//}
		return nil, nil, err
	}
	if msgs.Error() != nil {
		return nil, nil, msgs.Error()
	}

	var ackers []service.AckFunc
	var batch service.MessageBatch
	for msg := range msgs.Messages() {
		ackers = append(ackers, func(ctx context.Context, res error) error {
			if res == nil {
				return msg.Ack()
			}

			if j.nakDelay != nil {
				return msg.NakWithDelay(*j.nakDelay)
			} else {
				return msg.Nak()
			}
		})
		batch = append(batch, convertMessage(msg))
	}

	return batch, func(ctx context.Context, res error) error {
		for _, msg := range ackers {
			if err := msg(ctx, res); err != nil {
				return fmt.Errorf("failed to ack message: %v", err)
			}
		}

		return nil
	}, nil
}

func (j *jetStreamReader) Close(ctx context.Context) error {
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

func convertMessage(m jetstream.Msg) *service.Message {
	msg := service.NewMessage(m.Data())
	msg.MetaSet("nats_subject", m.Subject())

	metadata, err := m.Metadata()
	if err == nil {
		msg.MetaSet("nats_sequence_stream", strconv.Itoa(int(metadata.Sequence.Stream)))
		msg.MetaSet("nats_sequence_consumer", strconv.Itoa(int(metadata.Sequence.Consumer)))
		msg.MetaSet("nats_num_delivered", strconv.Itoa(int(metadata.NumDelivered)))
		msg.MetaSet("nats_num_pending", strconv.Itoa(int(metadata.NumPending)))
		msg.MetaSet("nats_domain", metadata.Domain)
		msg.MetaSet("nats_timestamp_unix_nano", strconv.Itoa(int(metadata.Timestamp.UnixNano())))
	}

	for k := range m.Headers() {
		v := m.Headers().Get(k)
		if v != "" {
			msg.MetaSet(k, v)
		}
	}

	return msg
}
