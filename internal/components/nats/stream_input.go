package nats

import (
	"context"
	"github.com/Jeffail/shutdown"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"strconv"
	"sync"
)

func natsJetStreamInputConfig() *service.ConfigSpec {
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
		Field(service.NewStringField("name").
			Description("an optional name for the consumer. If not set, one is generated automatically. Cannot contain whitespace, ., *, >, path separators (forward or backwards slash), and non-printable characters.").
			Default("").
			Optional()).
		Field(service.NewStringField("durable").
			Description("an optional durable name for the consumer. If both Durable and Name are set, they have to be equal. Unless InactiveThreshold is set, a durable consumer will not be cleaned up automatically. Cannot contain whitespace, ., *, >, path separators (forward or backwards slash), and non-printable characters.").
			Optional()).
		Field(service.NewStringListField("subjects").
			Description("allows filtering messages from a stream by subject").
			Optional().
			Example("foo.bar.baz").Example("foo.*.baz").Example("foo.bar.*").Example("foo.>")).
		Field(service.NewBoolField("bind").
			Description("Indicates that the subscription should use an existing consumer.").
			Optional()).

		Field(service.NewStringField("description").
			Description("an optional description of the consumer.").
			Optional().
			Advanced()).
		Field(service.NewStringAnnotatedEnumField("deliver", map[string]string{
			"all":              "Deliver all available messages.",
			"last":             "Deliver starting with the last published messages.",
			"new":              "Deliver starting from now, not taking into account any previous messages.",
			"last_per_subject": "Deliver starting with the last published message per subject.",
		}).
			Description("defines from which point to start delivering messages").
			Default("all").
			Advanced()).
		Field(service.NewDurationField("ack_wait").
			Description("defines how long the server will wait for an acknowledgement before resending a message.").
			Default("30s").
			Advanced()).
		Field(service.NewIntField("max_deliver").
			Description("defines the maximum number of delivery attempts for a message. Applies to any message that is re-sent due to ack policy.").
			Default(-1).
			Advanced()).
		Field(service.NewStringAnnotatedEnumField("replay", map[string]string{
			"instant":  "messages are sent as fast as possible.",
			"original": "messages are sent in the same intervals in which they were stored on stream.",
		}).
			Description("defines the rate at which messages are sent to the consumer").
			Default("instant").
			Advanced()).
		Field(service.NewIntField("max_ack_pending").
			Description("is a maximum number of outstanding unacknowledged messages. Once this limit is reached, the server will suspend sending messages to the consumer. Set to -1 for unlimited.").
			Advanced().
			Default(1024)).
		Fields(connectionTailFields()...).
		Field(inputTracingDocs())
}

func init() {
	err := service.RegisterInput(
		"jetstream_stream", natsJetStreamInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			input, err := newJetStreamReaderFromConfig(conf, mgr)
			if err != nil {
				return nil, err
			}
			return conf.WrapInputExtractTracingSpanMapping("jetstream_stream", input)
		})
	if err != nil {
		panic(err)
	}
}

//------------------------------------------------------------------------------

type jetStreamReader struct {
	connDetails connectionDetails
	stream      string
	bind        bool

	cc *jetstream.ConsumerConfig

	log *service.Logger

	connMut  sync.Mutex
	natsConn *nats.Conn

	consumer jetstream.Consumer
	messages jetstream.MessagesContext

	shutSig *shutdown.Signaller

	// The pool caller id. This is a unique identifier we will provide when calling methods on the pool. This is used by
	// the pool to do reference counting and ensure that connections are only closed when they are no longer in use.
	pcid string
}

func newJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*jetStreamReader, error) {
	j := jetStreamReader{
		log:     mgr.Logger(),
		shutSig: shutdown.NewSignaller(),
	}

	var err error
	if j.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
		return nil, err
	}

	j.cc = &jetstream.ConsumerConfig{}

	if j.stream, err = conf.FieldString("stream"); err != nil {
		return nil, err
	}

	if conf.Contains("name") {
		if j.cc.Name, err = conf.FieldString("name"); err != nil {
			return nil, err
		}
	}

	if conf.Contains("durable") {
		if j.cc.Durable, err = conf.FieldString("durable"); err != nil {
			return nil, err
		}
	}

	if conf.Contains("subjects") {
		if j.cc.FilterSubjects, err = conf.FieldStringList("subjects"); err != nil {
			return nil, err
		}
	}

	if conf.Contains("bind") {
		if j.bind, err = conf.FieldBool("bind"); err != nil {
			return nil, err
		}
	}

	if conf.Contains("description") {
		if j.cc.Description, err = conf.FieldString("description"); err != nil {
			return nil, err
		}
	}

	if j.cc.DeliverPolicy, err = parseDeliverPolicy(conf, "deliver"); err != nil {
		return nil, err
	}

	if j.cc.AckWait, err = conf.FieldDuration("ack_wait"); err != nil {
		return nil, err
	}

	if j.cc.MaxDeliver, err = conf.FieldInt("max_deliver"); err != nil {
		return nil, err
	}

	if j.cc.ReplayPolicy, err = parseReplayPolicy(conf, "replay"); err != nil {
		return nil, err
	}

	if j.cc.MaxAckPending, err = conf.FieldInt("max_ack_pending"); err != nil {
		return nil, err
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
				_ = pool.Release(j.pcid, j.connDetails)
			}
		}
	}()

	if natsConn, err = pool.Get(ctx, j.pcid, j.connDetails); err != nil {
		return err
	}

	js, err := jetstream.New(natsConn)
	if err != nil {
		return err
	}

	// if the consumer is bound, we don't need to create it again, just bind to it
	if j.bind && j.stream != "" && j.cc.Durable != "" {
		if j.consumer, err = js.Consumer(ctx, j.stream, j.cc.Durable); err != nil {
			return err
		}
	}

	if j.consumer, err = js.CreateOrUpdateConsumer(ctx, j.stream, *j.cc); err != nil {
		return err
	}

	if j.messages, err = j.consumer.Messages(); err != nil {
		return err
	}

	j.natsConn = natsConn
	return nil
}

func (j *jetStreamReader) disconnect() {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.messages != nil {
		j.messages.Stop()
	}
	if j.natsConn != nil {
		if err := pool.Release(j.pcid, j.connDetails); err != nil {
			j.log.Errorf("Failed to release NATS connection: %v", err)
		}

		j.natsConn = nil
	}
}

func (j *jetStreamReader) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	j.connMut.Lock()
	c := j.consumer
	j.connMut.Unlock()
	if c == nil {
		return nil, nil, service.ErrNotConnected
	}

	msgChan := make(chan *msgCall)
	go func() {
		msg, err := j.messages.Next()
		msgChan <- &msgCall{msg: msg, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, func(ctx context.Context, err error) error {
			return nil
		}, ctx.Err()
	case msg := <-msgChan:
		if msg.err != nil {
			return nil, nil, msg.err
		}
		return convertMessage(msg.msg)
		// why the hell did I ever did that??
		//case <-time.After(10 * time.Second):
		//  return nil, nil, service.ErrEndOfInput
	}
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

func convertMessage(m jetstream.Msg) (*service.Message, service.AckFunc, error) {
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

	return msg, func(ctx context.Context, res error) error {
		if res == nil {
			return m.Ack()
		}
		return m.Nak()
	}, nil
}

type msgCall struct {
	msg jetstream.Msg
	err error
}
