package nats

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redpanda-data/benthos/v4/public/service"
	"sync"

	"github.com/Jeffail/shutdown"
)

func natsJetStreamOutputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Version("3.46.0").
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
		Field(service.NewOutputMaxInFlightField().Default(1024)).
		Fields(connectionTailFields()...).
		Fields(outputTracingDocs())
}

func init() {
	err := service.RegisterOutput(
		"jetstream_stream", natsJetStreamOutputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
			maxInFlight, err := conf.FieldInt("max_in_flight")
			if err != nil {
				return nil, 0, err
			}
			w, err := newJetStreamWriterFromConfig(conf, mgr)
			if err != nil {
				return nil, 0, err
			}
			spanOutput, err := conf.WrapOutputExtractTracingSpanMapping("jetstream_stream", w)
			if err != nil {
				return nil, 0, err
			}
			return spanOutput, maxInFlight, err
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

	// The pool caller id. This is a unique identifier we will provide when calling methods on the pool. This is used by
	// the pool to do reference counting and ensure that connections are only closed when they are no longer in use.
	pcid string
}

func newJetStreamWriterFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*jetStreamOutput, error) {
	j := jetStreamOutput{
		log:     mgr.Logger(),
		shutSig: shutdown.NewSignaller(),
		pcid:    uuid.New().String(),
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

	var natsConn *nats.Conn
	var js jetstream.JetStream

	defer func() {
		if err != nil && natsConn != nil {
			_ = pool.Release(j.pcid, j.connDetails)
		}
	}()

	if natsConn, err = pool.Get(ctx, j.pcid, j.connDetails); err != nil {
		return err
	}

	if js, err = jetstream.New(natsConn); err != nil {
		return err
	}

	j.natsConn = natsConn
	j.js = js
	return nil
}

func (j *jetStreamOutput) disconnect() {
	j.connMut.Lock()
	defer j.connMut.Unlock()

	if j.natsConn != nil {
		_ = pool.Release(j.pcid, j.connDetails)
		j.natsConn = nil
	}
	j.js = nil
}

//------------------------------------------------------------------------------

func (j *jetStreamOutput) Write(ctx context.Context, msg *service.Message) error {
	j.connMut.Lock()
	js := j.js
	j.connMut.Unlock()
	if js == nil {
		return service.ErrNotConnected
	}

	subject, err := j.subjectStr.TryString(msg)
	if err != nil {
		return fmt.Errorf(`failed string interpolation on field "subject": %w`, err)
	}

	jsmsg := nats.NewMsg(subject)
	msgBytes, err := msg.AsBytes()
	if err != nil {
		return err
	}

	jsmsg.Data = msgBytes
	for k, v := range j.headers {
		value, err := v.TryString(msg)
		if err != nil {
			return fmt.Errorf(`failed string interpolation on header %q: %w`, k, err)
		}

		jsmsg.Header.Add(k, value)
	}
	_ = j.metaFilter.Walk(msg, func(key, value string) error {
		jsmsg.Header.Add(key, value)
		return nil
	})

	_, err = js.PublishMsg(ctx, jsmsg)
	return err
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
