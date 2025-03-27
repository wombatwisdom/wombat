package core

import (
    "context"
    "fmt"
    "github.com/nats-io/nats.go"
    "github.com/redpanda-data/benthos/v4/public/service"
    "github.com/wombatwisdom/wombat/docs"
    "github.com/wombatwisdom/wombat/metadata"
    "strings"
)

const (
    OutputComponentName = "nats_core"

    OutputSubjectField     = "subject"
    OutputBatchPolicyField = "batching"
    OutputAdvancedField    = "advanced"
)

func OutputConfig() *service.ConfigSpec {
    commonConnectionFields, advancedConnectionFields := ConnectionConfigFields()

    var advancedFields []*service.ConfigField
    advancedFields = append(advancedFields, advancedConnectionFields...)
    advancedFields = append(advancedFields,
        service.NewOutputMaxInFlightField(),
        metadata.MetadataFilterConfig(),
        service.NewBatchPolicyField(OutputBatchPolicyField))

    return service.NewConfigSpec().
        Description(docs.NewDescription("Send messages to a NATS subject.").WithBody(`
The NATS core output allows you to send messages to a subject. It does so by creating a NATS publisher and 
converting each message into a NATS message and send each individually to the NATS server. The subject is an expression
that is constructed based on the message being processed.`).Markdown()).
        Fields(
            service.NewInterpolatedStringField(OutputSubjectField).
                Examples("my.subject", "accounts.${!this.account}.invoices.${!this.invoice_id}", "logs.${!this.year}").
                Description(strings.TrimSpace(`
The expression used to construct the subject to send the message to.
The expression cannot contain any wildcards. Instead, it can contain variables that are extracted from the message being processed.`)),
        ).
        Fields(commonConnectionFields...).
        Field(service.NewObjectField(OutputAdvancedField, advancedFields...))
}

func OutputFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
    connection, err := ConnectionFromConfig(conf)
    if err != nil {
        return nil, service.BatchPolicy{}, 0, err
    }

    subject, err := conf.FieldInterpolatedString(OutputSubjectField)
    if err != nil {
        return nil, service.BatchPolicy{}, 0, err
    }

    metadataFilter, err := metadata.FilterFromConfig(conf)
    if err != nil {
        return nil, service.BatchPolicy{}, 0, err
    }

    batchPolicy, err := conf.FieldBatchPolicy(OutputBatchPolicyField)
    if err != nil {
        return nil, service.BatchPolicy{}, 0, err
    }

    maxInFlight, err := conf.FieldMaxInFlight()
    if err != nil {
        return nil, service.BatchPolicy{}, 0, err
    }

    return &Output{
        Connection:     *connection,
        Subject:        subject,
        MetadataFilter: metadataFilter,
    }, batchPolicy, maxInFlight, nil
}

type Output struct {
    Connection

    Subject        *service.InterpolatedString
    MetadataFilter metadata.Filter
}

func (o *Output) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
    for idx, message := range batch {
        if err := o.Write(ctx, message); err != nil {
            return fmt.Errorf("batch #%d: %w", idx, err)
        }
    }

    return nil
}

func (o *Output) Write(ctx context.Context, message *service.Message) error {
    subject, err := o.Subject.TryString(message)
    if err != nil {
        return fmt.Errorf("subject: %w", err)
    }

    payload, err := message.AsBytes()
    if err != nil {
        return fmt.Errorf("payload: %w", err)
    }

    header := make(map[string][]string)
    _ = message.MetaWalk(func(key string, value string) error {
        // -- skip the metadata if the filter is set and the key is not included
        if o.MetadataFilter != nil && !o.MetadataFilter.Include(key) {
            return nil
        }

        header[key] = append(header[key], value)
        return nil
    })

    msg := nats.NewMsg(subject)
    msg.Data = payload
    msg.Header = header

    if err := o.Client().PublishMsg(msg); err != nil {
        return fmt.Errorf("publish: %w", err)
    }

    return nil
}
