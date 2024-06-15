package nats

import (
  "context"
  "github.com/nats-io/nats.go/jetstream"
  "github.com/redpanda-data/benthos/v4/public/service"
)

func orderedJetStreamInputConfig() *service.ConfigSpec {
  return service.NewConfigSpec().
    Stable().
    Categories("Services").
    Version("3.46.0").
    Summary("Reads messages from a NATS JetStream Stream in the order they were published.").
    Description(`
### Ordered Consumer
This input is based on the Ordered Consumer feature of NATS JetStream. It allows you to consume messages from a stream 
in the order they were published.

Ordered is strictly processing messages in the order that they were stored on the stream, providing a consistent and 
deterministic message ordering. It is also resilient to consumer deletion.

` + connectionNameDescription() + jetstreamInputDescription()).
    Fields(connectionHeadFields()...).
    Field(service.NewStringField("stream").
      Description("A stream to consume from.")).
    Field(service.NewStringListField("filter_subjects").
      Description("Filter messages from a stream by subject. These subjects may contain wildcards.").
      Optional().
      Examples("foo.bar.baz", "foo.>", "foo.*.baz")).
    Field(service.NewStringAnnotatedEnumField("deliver_policy", map[string]string{
      "all":              "Deliver all available messages.",
      "last":             "Deliver starting with the last published messages.",
      "new":              "Deliver starting from now, not taking into account any previous messages.",
      "by_start_time":    "Deliver starting from a specific time.",
      "last_per_subject": "Deliver starting with the last published message per subject.",
    }).
      Description("Defines from which point to start delivering messages from the stream.").
      Default("all")).
    Field(service.NewStringField("start_time").
      Description("An optional time from which to start message delivery. Only applicable when delivery_policy is set to 'by_start_time'").
      Optional().
      Advanced()).
    Field(service.NewStringAnnotatedEnumField("replay_policy", map[string]string{
      "original": "messages are sent in the same intervals in which they were stored on stream.",
      "instant":  "messages are sent as fast as possible.",
    }).
      Description("the rate at which messages are sent to the consumer.").
      Default("original").
      Advanced()).
    Field(service.NewIntField("max_ack_pending").
      Description("The maximum number of outstanding acks to be allowed before consuming is halted.").
      Advanced().
      Default(1024)).
    Fields(connectionTailFields()...).
    Fields(inputTracingDocs())
}

func init() {
  err := service.RegisterBatchInput(
    "jetstream_stream", orderedJetStreamInputConfig(),
    func(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
      return newOrderedJetStreamReaderFromConfig(conf, mgr)
    })
  if err != nil {
    panic(err)
  }
}

//------------------------------------------------------------------------------

func newOrderedJetStreamReaderFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*jetStreamReader, error) {
  return newJetStreamReaderFromConfig(conf, mgr, func(ctx context.Context, js jetstream.JetStream) (jetstream.Consumer, error) {
    stream, err := conf.FieldString("stream")
    if err != nil {
      return nil, err
    }

    cfg, err := parseOrderedJetStreamConfig(conf)
    if err != nil {
      return nil, err
    }

    return js.OrderedConsumer(ctx, stream, *cfg)
  })
}

//------------------------------------------------------------------------------

func parseOrderedJetStreamConfig(conf *service.ParsedConfig) (*jetstream.OrderedConsumerConfig, error) {
  cfg := jetstream.OrderedConsumerConfig{}

  var err error
  if cfg.FilterSubjects, err = conf.FieldStringList("filter_subjects"); err != nil {
    return nil, err
  }

  if cfg.DeliverPolicy, err = parseDeliverPolicy(conf, "deliver_policy"); err != nil {
    return nil, err
  }

  if cfg.OptStartTime, err = parseTime(conf, "start_time"); err != nil {
    return nil, err
  }

  if cfg.ReplayPolicy, err = parseReplayPolicy(conf, "replay_policy"); err != nil {
    return nil, err
  }

  return &cfg, nil
}
