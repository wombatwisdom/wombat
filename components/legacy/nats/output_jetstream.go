package nats

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/Jeffail/shutdown"

	"github.com/redpanda-data/benthos/v4/public/service"
)

func natsJetStreamOutputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Write messages to a NATS JetStream subject.").
		Description(connectionNameDescription() + authDescription()).
		Fields(connectionHeadFields()...).
		Field(service.NewInterpolatedStringField("subject").
			Description("A subject to write to.").
			Example("foo.bar.baz").
			Example(`${! meta("kafka_topic") }`).
			Example(`foo.${! json("meta.type") }`)).
		Field(service.NewInterpolatedStringMapField("headers").
			Description("Explicit message headers to add to messages.").
			Default(map[string]any{}).
			Example(map[string]any{
				"Content-Type": "application/json",
				"Timestamp":    `${!meta("Timestamp")}`,
			}).Version("4.1.0")).
		Field(service.NewMetadataFilterField("metadata").
			Description("Determine which (if any) metadata values should be added to messages as headers.").
			Optional()).
		Field(service.NewOutputMaxInFlightField().
			Default(1024)).
		Fields(service.NewBatchPolicyField("batching")).
		Fields(connectionTailFields()...).
		Field(outputTracingDocs())
}

func init() {
	err := service.RegisterBatchOutput(
		"nats_jetstream_batched", natsJetStreamOutputConfig(),

		func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
			bPol, err := conf.FieldBatchPolicy("batching")
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}

			maxInFlight, err := conf.FieldInt("max_in_flight")
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}
			w, err := newJetStreamWriterFromConfig(conf, mgr)
			if err != nil {
				return nil, service.BatchPolicy{}, 0, err
			}
			spanOutput, err := conf.WrapBatchOutputExtractTracingSpanMapping("nats_jetstream_batched", w)
			return spanOutput, bPol, maxInFlight, err
		})
	if err != nil {
		panic(err)
	}
}

//------------------------------------------------------------------------------

type jetStreamOutput struct {
	connDetails   connectionDetails
	subjectStrRaw string
	subjectStr    *service.InterpolatedString
	headers       map[string]*service.InterpolatedString
	metaFilter    *service.MetadataFilter

	log *service.Logger

	connMut  sync.Mutex
	natsConn *nats.Conn
	js       jetstream.JetStream

	shutSig *shutdown.Signaller
}

func newJetStreamWriterFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*jetStreamOutput, error) {
	j := jetStreamOutput{
		log:     mgr.Logger(),
		shutSig: shutdown.NewSignaller(),
	}

	var err error
	if j.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
		return nil, err
	}

	if j.subjectStrRaw, err = conf.FieldString("subject"); err != nil {
		return nil, err
	}

	if j.subjectStr, err = conf.FieldInterpolatedString("subject"); err != nil {
		return nil, err
	}

	if j.headers, err = conf.FieldInterpolatedStringMap("headers"); err != nil {
		return nil, err
	}

	if conf.Contains("metadata") {
		if j.metaFilter, err = conf.FieldMetadataFilter("metadata"); err != nil {
			return nil, err
		}
	}
	return &j, nil
}

//------------------------------------------------------------------------------

func (j *jetStreamOutput) Connect(ctx context.Context) (err error) {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		return nil
	}

	defer func() {
		if err != nil && j.natsConn != nil {
			j.natsConn.Close()
		}
	}()

	if j.natsConn, err = j.connDetails.get(ctx); err != nil {
		return err
	}

	if j.js, err = jetstream.New(j.natsConn); err != nil {
		return err
	}

	return nil
}

func (j *jetStreamOutput) disconnect() {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		j.natsConn.Close()
		j.natsConn = nil
	}
	j.js = nil
}

//------------------------------------------------------------------------------

func (j *jetStreamOutput) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
	if j.js == nil {
		return service.ErrNotConnected
	}

	var pafs []jetstream.PubAckFuture

	for _, msg := range batch {
		nmsg, err := j.convertMsg(msg)
		if err != nil {
			return err
		}

		paf, err := j.js.PublishMsgAsync(nmsg)
		if err != nil {
			return err
		}
		pafs = append(pafs, paf)
	}

	// -- wait for the acks to come back
	<-j.js.PublishAsyncComplete()

	// -- run through the list of acks and check for errors. If an error occured, return it.
	for _, paf := range pafs {
		select {
		case <-paf.Ok():
		case err := <-paf.Err():
			return err
		}
	}

	return nil
}

func (j *jetStreamOutput) Close(ctx context.Context) error {
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

func (j *jetStreamOutput) convertMsg(msg *service.Message) (*nats.Msg, error) {
	subject, err := j.subjectStr.TryString(msg)
	if err != nil {
		return nil, fmt.Errorf(`failed string interpolation on field "subject": %w`, err)
	}

	jsmsg := nats.NewMsg(subject)
	msgBytes, err := msg.AsBytes()
	if err != nil {
		return nil, err
	}

	jsmsg.Data = msgBytes
	for k, v := range j.headers {
		value, err := v.TryString(msg)
		if err != nil {
			return nil, fmt.Errorf(`failed string interpolation on header %q: %w`, k, err)
		}

		jsmsg.Header.Add(k, value)
	}
	_ = j.metaFilter.Walk(msg, func(key, value string) error {
		jsmsg.Header.Add(key, value)
		return nil
	})

	return jsmsg, nil
}
