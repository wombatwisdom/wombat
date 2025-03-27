package core

import (
    "context"
    "github.com/nats-io/nats.go"
    "github.com/redpanda-data/benthos/v4/public/service"
    "github.com/wombatwisdom/wombat/docs"
    "github.com/wombatwisdom/wombat/utils"
    "strings"
)

const (
    InputComponentName = "nats_core"

    InputSubjectField    = "subject"
    InputQueueField      = "queue"
    InputBatchCountField = "batch_count"
    InputAdvancedField   = "advanced"
)

func InputConfig() *service.ConfigSpec {
    commonConnectionFields, advancedConnectionFields := ConnectionConfigFields()

    return service.NewConfigSpec().
        Description(docs.NewDescription("Receive messages from a NATS subject.").
            WithBody(`
The NATS core input allows you to read messages from a subject. It does so by creating a NATS subscriber and fetch
batches of messages at a time. By default, the batch count is set to 1, which is quite conservative since it means
a new message is only fetched once the current one has been processed.`).
            WithSubSections(docs.NewSection("Queue Groups").WithBody(`
Each input with the same queue name will be load balancing messages across all members of the group. This is useful 
when you want to scale the processing of messages across multiple instances.`)).
            WithSubSections(ConnectionDescription()...).
            Markdown()).
        Fields(
            service.NewStringField(InputSubjectField).
                Examples("my.subject", "accounts.*.invoices.*", "logs.>").
                Description(strings.TrimSpace(`
The subject to subscribe to.
The subject may contain wildcards, which will be matched against any subject that matches the pattern.`)),
            service.NewStringField(InputQueueField).
                Optional().
                Description(strings.TrimSpace(`
The queue group to join.
If set, the subscription will be load balancing messages across all members of the group.`)),
            service.NewIntField(InputBatchCountField).
                Default(1).
                Description(strings.TrimSpace(`
The maximum number of messages to fetch at a time.
This field is used to control the number of messages that are fetched from the NATS server in a single batch. 
Processing guarantees apply to the batch, not individual messages. This means that when processing a batch of messages,
a failure would cause the entire batch to be reprocessed.`)),
        ).
        Fields(commonConnectionFields...).
        Field(service.NewObjectField(InputAdvancedField, advancedConnectionFields...))

}

func InputFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
    connection, err := ConnectionFromConfig(conf)
    if err != nil {
        return nil, err
    }

    input := &Input{
        Connection: *connection,
    }

    input.Subject, err = conf.FieldString(InputSubjectField)
    if err != nil {
        return nil, err
    }

    input.Queue, err = conf.FieldString(InputQueueField)
    if err != nil {
        return nil, err
    }

    input.BatchCount, err = conf.FieldInt(InputBatchCountField)
    if err != nil {
        return nil, err
    }

    return input, nil
}

type Input struct {
    Connection

    Subject string
    Queue   string

    BatchCount int

    sub *nats.Subscription
}

func (i *Input) Connect(ctx context.Context) error {
    if err := i.Connection.Connect(ctx); err != nil {
        return err
    }

    // create the subscription
    var err error
    if i.Queue == "" {
        i.sub, err = i.Client().SubscribeSync(i.Subject)
    } else {
        i.sub, err = i.Client().QueueSubscribeSync(i.Subject, i.Queue)
    }
    return err
}

func (i *Input) Close(ctx context.Context) error {
    if i.sub != nil {
        if err := i.sub.Unsubscribe(); err != nil {
            return err
        }
    }

    return i.Connection.Close(ctx)
}

func (i *Input) ReadBatch(ctx context.Context) (service.MessageBatch, service.AckFunc, error) {
    msgs, err := i.sub.Fetch(i.BatchCount)
    if err != nil {
        return nil, nil, err
    }

    result := make([]*service.Message, len(msgs))
    for i, msg := range msgs {
        m := service.NewMessage(msg.Data)

        for k, v := range msg.Header {
            if len(v) == 1 {
                m.MetaSet(k, v[0])
            } else {
                m.MetaSet(k, strings.Join(v, ","))
            }
        }

        // add message metadata as headers
        m.MetaSet("nats_subject", msg.Subject)
        m.MetaSet("nats_reply", msg.Reply)

        result[i] = m
    }

    return result, utils.AckPassthrough, nil
}
