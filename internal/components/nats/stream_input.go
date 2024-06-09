package nats

import (
  "context"
  "errors"
  "github.com/Jeffail/shutdown"
  "github.com/nats-io/nats.go"
  "github.com/nats-io/nats.go/jetstream"
  "github.com/redpanda-data/benthos/v4/public/service"
  "strconv"
  "sync"
)

func jetstreamInputDescription() string {
  return `### Metadata

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

` + connectionNameDescription() + authDescription()
}

func natsJetStreamInputConfig() *service.ConfigSpec {
  return service.NewConfigSpec().
    Stable().
    Categories("Services").
    Version("3.46.0").
    Summary("Reads messages from NATS JetStream subjects.").
    Fields(connectionHeadFields()...).
    Field(service.NewStringField("queue").
      Description("An optional queue group to consume as.").
      Optional()).
    Field(service.NewStringField("subject").
      Description("A subject to consume from. Supports wildcards for consuming multiple subjects. Either a subject or stream must be specified.").
      Optional().
      Example("foo.bar.baz").Example("foo.*.baz").Example("foo.bar.*").Example("foo.>")).
    Field(service.NewStringField("durable").
      Description("Preserve the state of your consumer under a durable name.").
      Optional()).
    Field(service.NewStringField("stream").
      Description("A stream to consume from. Either a subject or stream must be specified.").
      Optional()).
    Field(service.NewBoolField("bind").
      Description("Indicates that the subscription should use an existing consumer.").
      Optional()).
    Field(service.NewStringAnnotatedEnumField("deliver", map[string]string{
      "all":              "Deliver all available messages.",
      "last":             "Deliver starting with the last published messages.",
      "last_per_subject": "Deliver starting with the last published message per subject.",
      "new":              "Deliver starting from now, not taking into account any previous messages.",
    }).
      Description("Determines which messages to deliver when consuming without a durable subscriber.").
      Default("all")).
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
    Fields(inputTracingDocs()).
    Fields(connectionTailFields()...)
}

//------------------------------------------------------------------------------

type consumerCreationCallback func(ctx context.Context, js jetstream.JetStream) (jetstream.Consumer, error)

type jetStreamReader struct {
  connDetails connectionDetails
  ccc         consumerCreationCallback
  pullOpts    []jetstream.PullMessagesOpt

  log *service.Logger

  connMut  sync.Mutex
  natsConn *nats.Conn
  messages jetstream.MessagesContext

  shutSig *shutdown.Signaller
}

func newJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources, ccc consumerCreationCallback) (*jetStreamReader, error) {
  j := jetStreamReader{
    log:     mgr.Logger(),
    shutSig: shutdown.NewSignaller(),
    ccc:     ccc,
  }

  var err error
  if j.connDetails, err = connectionDetailsFromParsed(conf, mgr); err != nil {
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

  var nc *nats.Conn
  var js jetstream.JetStream
  var consumer jetstream.Consumer
  var messages jetstream.MessagesContext

  defer func() {
    if err != nil {
      if messages != nil {
        messages.Drain()
      }
      if nc != nil {
        nc.Close()
      }
    }
  }()

  if nc, err = j.connDetails.get(ctx); err != nil {
    return err
  }

  if js, err = jetstream.New(nc); err != nil {
    return err
  }

  if consumer, err = j.ccc(ctx, js); err != nil {
    return err
  }

  if messages, err = consumer.Messages(j.pullOpts...); err != nil {
    return err
  }

  j.natsConn = nc
  j.messages = messages

  return nil
}

func (j *jetStreamReader) disconnect() {
  j.connMut.Lock()
  defer j.connMut.Unlock()

  if j.messages != nil {
    j.messages.Drain()
  }

  if j.natsConn != nil {
    j.natsConn.Close()
    j.natsConn = nil
  }
}

func (j *jetStreamReader) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
  msg, err := j.messages.Next()
  if err != nil {
    if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
      return nil, nil, service.ErrEndOfInput
    }
    return nil, nil, err
  }

  return convertMessage(msg)
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
