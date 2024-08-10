package nats

import (
	"context"
	"fmt"
	"github.com/Jeffail/shutdown"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"sync"
	"time"
)

func natsClaimingJetStreamInputConfig() *service.ConfigSpec {
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
		Field(service.NewDurationField("redeliver_delay").
			Description("defines how long to ask the server to wait to redeliver the message in case a message is already being processed for the subject.").
			Default("100ms").
			Advanced()).
		Field(service.NewDurationField("msg_wait").
			Description("defines how long to wait for a message to be received.").
			Default("0s").
			Advanced()).
		Field(service.NewDurationField("ack_wait").
			Description("defines how long the server will wait for an acknowledgement before resending a message.").
			Default("30s").
			Advanced()).
		Field(service.NewIntField("max_deliver").
			Description("defines the maximum number of delivery attempts for a message. Applies to any message that is re-sent due to ack policy.").
			Default(-1).
			Advanced()).
		Field(service.NewStringField("claim_header").
			Description("defines the header to be used as the claim key. The claim key is used to make sure no other messages with the same key can be processed concurrently").
			Optional().
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
		"jetstream_stream_claim", natsClaimingJetStreamInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			input, err := newClaimingJetStreamReaderFromConfig(conf, mgr)
			if err != nil {
				return nil, err
			}
			return conf.WrapInputExtractTracingSpanMapping("jetstream_stream_claim", input)
		})
	if err != nil {
		panic(err)
	}
}

//------------------------------------------------------------------------------

type claimingJetStreamReader struct {
	connDetails    connectionDetails
	stream         string
	bind           bool
	msgWait        time.Duration
	redeliverDelay time.Duration

	cc  *jetstream.ConsumerConfig
	log *service.Logger

	claimHeader string
	claims      map[string]struct{}
	claimsMut   sync.Mutex

	messages jetstream.MessagesContext

	connMut  sync.Mutex
	natsConn *nats.Conn

	shutSig *shutdown.Signaller

	// The pool caller id. This is a unique identifier we will provide when calling methods on the pool. This is used by
	// the pool to do reference counting and ensure that connections are only closed when they are no longer in use.
	pcid string
}

func newClaimingJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*claimingJetStreamReader, error) {
	j := claimingJetStreamReader{
		log:     mgr.Logger(),
		shutSig: shutdown.NewSignaller(),
		claims:  map[string]struct{}{},
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

	if j.msgWait, err = conf.FieldDuration("msg_wait"); err != nil {
		return nil, err
	}

	if j.redeliverDelay, err = conf.FieldDuration("redeliver_delay"); err != nil {
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

	if conf.Contains("claim_header") {
		if j.claimHeader, err = conf.FieldString("claim_header"); err != nil {
			return nil, err
		}
	}

	return &j, nil
}

//------------------------------------------------------------------------------

func (j *claimingJetStreamReader) Connect(ctx context.Context) (err error) {
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
	// if the consumer is bound, we don't need to create it again, just bind to it
	if j.bind && j.stream != "" && j.cc.Durable != "" {
		if _, err = js.Consumer(ctx, j.stream, j.cc.Durable); err != nil {
			return err
		}
	}

	if consumer, err = js.CreateOrUpdateConsumer(ctx, j.stream, *j.cc); err != nil {
		return err
	}

	if j.messages, err = consumer.Messages(); err != nil {
		return err
	}

	return nil
}

func (j *claimingJetStreamReader) disconnect() {
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

func (j *claimingJetStreamReader) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	if j.messages == nil {
		return nil, func(ctx context.Context, err error) error { return nil }, service.ErrNotConnected
	}

	for {
		jsMsg, err := j.nextMsg(ctx, j.msgWait)
		if err != nil {
			return nil, func(ctx context.Context, err error) error { return nil }, err
		}

		ack, err := j.claim(jsMsg)
		if err != nil {
			return nil, ack, err
		}

		if ack == nil {
			// -- if ack is nil, the subject is already claimed, so we should not process the message
			// -- instead we will instruct the server to send the message again at a later point
			_ = jsMsg.NakWithDelay(j.redeliverDelay)
			j.log.Trace("Message already claimed, NAKing")
			continue
		}

		msg, err := convertMessage(jsMsg)
		if err != nil {
			return nil, func(ctx context.Context, err error) error { return nil }, err
		}

		return msg, ack, nil
	}
}

func (j *claimingJetStreamReader) Close(ctx context.Context) error {
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

func (j *claimingJetStreamReader) nextMsg(ctx context.Context, timeout time.Duration) (jetstream.Msg, error) {
	result := make(chan msgErr, 1)
	go func() {
		msg, err := j.messages.Next()
		result <- msgErr{msg: msg, err: err}
	}()

	if timeout == 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-result:
			return r.msg, r.err
		}
	} else {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			j.log.Trace("Message receive timeout")
			return nil, fmt.Errorf("timed out")
		case r := <-result:
			return r.msg, r.err
		}
	}
}

// claim claims the claim_key for processing.
//
// claim will not return an ack function if the claim_key has already been claimed. This should be an indication
// for the caller not to process the message, but instead NAK it to be processed at a later point in time.
//
// The returned ack function will remove the claim from the claim_key, resulting in new messages being received for the
// subject to be able to claim the claim_key again.
//
// If the claim_key is nil, the ack function will ACK the message if err is nil, and NAK it if err is not nil.
func (j *claimingJetStreamReader) claim(jsMsg jetstream.Msg) (service.AckFunc, error) {
	if j.claimHeader == "" {
		return func(ctx context.Context, err error) error {
			if err == nil {
				return jsMsg.Ack()
			} else {
				return jsMsg.Nak()
			}
		}, nil
	}

	// -- get the header with specified as the claim key
	ck := jsMsg.Headers().Get(j.claimHeader)
	j.claimsMut.Lock()
	defer j.claimsMut.Unlock()

	// -- check if the subject is already claimed
	if _, exists := j.claims[ck]; exists {
		return nil, nil
	}

	j.log.Tracef("Claiming message with key: %v; currently holding %d claims", ck, len(j.claims))
	// -- claim the message
	j.claims[ck] = struct{}{}
	return func(ctx context.Context, err error) error {
		// -- unclaim the message
		func() {
			j.claimsMut.Lock()
			defer j.claimsMut.Unlock()
			delete(j.claims, ck)
			j.log.Tracef("released claim for key: %v; currently holding %d claims", ck, len(j.claims))
		}()

		if err == nil {
			return jsMsg.Ack()
		} else {
			if err2 := jsMsg.Nak(); err2 != nil {
				return err2
			}

			return err
		}
	}, nil
}
