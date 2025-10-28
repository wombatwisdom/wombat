package ibmmq

import (
	"context"
	"errors"
	"fmt"
	"github.com/wombatwisdom/components/framework/spec"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	ibmmq "github.com/wombatwisdom/components/bundles/ibm-mq"

	"github.com/wombatwisdom/wombat/public/components/wombatwisdom"
)

const (
	fldQueueManagerName = "queue_manager_name"
	fldQueueName        = "queue_name"
	fldChannelName      = "channel_name"
	fldConnectionName   = "connection_name"
	fldUserId           = "user_id"
	fldPassword         = "password"
	fldApplicationName  = "application_name"
	fldBatchSize        = "batch_size"
	fldBatchWaitTime    = "batch_wait_time"
	fldTLS              = "tls"
	fldTLSEnabled       = "enabled"
	fldTLSCipherSpec    = "cipher_spec"
	fldTLSKeyRepository = "key_repository"
	fldTLSKeyRepoPass   = "key_repository_password"
	fldTLSCertLabel     = "certificate_label"
	fldTLSPeerName      = "ssl_peer_name"
	fldTLSFipsRequired  = "fips_required"
)

func inputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Connects to IBM MQ queue managers and consumes messages from specified queues.").
		Description(`
Connects to IBM MQ queue managers and consumes messages from specified queues.

Uses IBM MQ input component found in [wombatwisdom/components](https://github.com/wombatwisdom/components).

## Build Requirements

This component requires IBM MQ client libraries. Build with the ` + "`mqclient`" + ` build tag:

` + "```bash" + `
go build -tags mqclient
` + "```" + `
`).
		Field(service.NewStringField(fldQueueManagerName).
			Description("The queue manager to connect to").
			Example("QM1")).
		Field(service.NewStringField(fldQueueName).
			Description("The queue name to read messages from").
			Example("DEV.QUEUE.1")).
		Field(service.NewStringField(fldChannelName).
			Description("The channel name for client connections").
			Example("DEV.APP.SVRCONN")).
		Field(service.NewStringField(fldConnectionName).
			Description("The connection name in the format hostname(port)").
			Example("localhost(1414)")).
		Field(service.NewStringField(fldUserId).
			Description("Optional: The user ID for authentication").
			Default("").
			Optional()).
		Field(service.NewStringField(fldPassword).
			Description("Optional: The user password for authentication").
			Default("").
			Optional().
			Secret()).
		Field(service.NewStringField(fldApplicationName).
			Description("Optional: Application name for MQ connection identification").
			Default("wombat").
			Optional().
			Advanced()).
		Field(service.NewIntField(fldBatchSize).
			Description("Maximum number of messages to batch together").
			Default(1).
			Optional().
			Advanced()).
		Field(service.NewDurationField(fldBatchWaitTime).
			Description("Maximum time to wait for a complete batch before returning partial batch").
			Default("100ms").
			Optional().
			Advanced()).
		Field(service.NewObjectField(fldTLS,
			service.NewBoolField(fldTLSEnabled).
				Description("Enable TLS encryption for the connection").
				Default(false),
			service.NewStringField(fldTLSCipherSpec).
				Description("The cipher specification to use for TLS").
				Example("TLS_RSA_WITH_AES_128_CBC_SHA256").
				Default("").
				Optional(),
			service.NewStringField(fldTLSKeyRepository).
				Description("Path to the key repository containing certificates (without file extension)").
				Example("/opt/mqm/ssl/key").
				Default("").
				Optional(),
			service.NewStringField(fldTLSKeyRepoPass).
				Description("Password for the key repository").
				Default("").
				Optional().
				Secret(),
			service.NewStringField(fldTLSCertLabel).
				Description("Certificate label to use from the key repository").
				Default("").
				Optional(),
			service.NewStringField(fldTLSPeerName).
				Description("Peer name for SSL/TLS validation").
				Default("").
				Optional(),
			service.NewBoolField(fldTLSFipsRequired).
				Description("Require FIPS 140-2 compliant algorithms").
				Default(false).
				Optional(),
		).Description("TLS/SSL configuration for secure connections").
			Optional().Advanced())
}

type input struct {
	inputConfig *ibmmq.InputConfig
	input       *ibmmq.Input
	logger      *service.Logger
	mgr         *service.Resources
	compCtx     *wombatwisdom.ComponentContext
	compCancel  context.CancelFunc
}

func newInput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
	bp := service.BatchPolicy{Count: 1}

	// Extract configuration
	queueManagerName, err := conf.FieldString(fldQueueManagerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue_manager_name: %w", err)
	}

	queueName, err := conf.FieldString(fldQueueName)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue_name: %w", err)
	}

	channelName, err := conf.FieldString(fldChannelName)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel_name: %w", err)
	}

	connectionName, err := conf.FieldString(fldConnectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection_name: %w", err)
	}

	userId, _ := conf.FieldString(fldUserId)
	password, _ := conf.FieldString(fldPassword)
	applicationName, _ := conf.FieldString(fldApplicationName)
	if applicationName == "" {
		applicationName = "wombat"
	}

	batchSize, _ := conf.FieldInt(fldBatchSize)
	if batchSize <= 0 {
		batchSize = 1
	}

	batchWaitTime, _ := conf.FieldDuration(fldBatchWaitTime)
	if batchWaitTime <= 0 {
		batchWaitTime = 100 * time.Millisecond
	}

	// Extract TLS configuration if present
	var tlsConfig *ibmmq.TLSConfig
	if conf.Contains(fldTLS) {
		if tlsRaw, err := conf.FieldAny(fldTLS); err == nil {
			if tlsMap, ok := tlsRaw.(map[string]interface{}); ok {
				if enabled, ok := tlsMap[fldTLSEnabled].(bool); ok && enabled {
					tlsConfig = &ibmmq.TLSConfig{
						Enabled: true,
					}
					if cs, ok := tlsMap[fldTLSCipherSpec].(string); ok {
						tlsConfig.CipherSpec = cs
					}
					if kr, ok := tlsMap[fldTLSKeyRepository].(string); ok {
						tlsConfig.KeyRepository = kr
					}
					if krp, ok := tlsMap[fldTLSKeyRepoPass].(string); ok {
						tlsConfig.KeyRepositoryPassword = krp
					}
					if cl, ok := tlsMap[fldTLSCertLabel].(string); ok {
						tlsConfig.CertificateLabel = cl
					}
				}
			}
		}
	}

	// Create the IBM MQ input configuration
	inputConfig := &ibmmq.InputConfig{
		CommonMQConfig: ibmmq.CommonMQConfig{
			QueueManagerName: queueManagerName,
			ChannelName:      channelName,
			ConnectionName:   connectionName,
			UserId:           userId,
			Password:         password,
			ApplicationName:  applicationName,
		},
		QueueName:     queueName,
		BatchSize:     batchSize,
		BatchWaitTime: batchWaitTime.String(),
	}

	bp.Count = batchSize

	i := &input{
		inputConfig: inputConfig,
		logger:      mgr.Logger(),
		mgr:         mgr,
	}

	env := wombatwisdom.NewEnvironment(i.logger)

	input, err := ibmmq.NewInput(env, *i.inputConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create IBM MQ input: %w", err)
	}
	i.input = input

	return i, nil
}

func (i *input) Connect(closeAtLeisureCtx context.Context) error {
	ctx, cancel := context.WithCancel(closeAtLeisureCtx)
	i.compCtx = wombatwisdom.NewComponentContext(ctx, i.logger)
	i.compCancel = cancel

	err := i.input.Init(i.compCtx)

	return translateError(err)
}

func (i *input) ReadBatch(closeAtLeisureCtx context.Context) (service.MessageBatch, service.AckFunc, error) {
	specBatch, cb, err := i.input.Read(i.compCtx)
	if err != nil {
		return nil, nil, translateError(err)
	}

	if specBatch == nil {
		// returning a nil error simply tells benthos there's no data, but don't stop processing
		return nil, nil, nil
	}
	// Convert spec.Batch to Benthos message batch
	batch, err := toBenthosMessageBatch(specBatch)
	if err != nil {
		return nil, nil, translateError(err)
	}

	// Return the actual ack function for at-least-once delivery
	return batch, func(closeNowCtx context.Context, err error) error { return cb(closeNowCtx, err) }, nil
}

func toBenthosMessageBatch(batchMsg spec.Batch) (service.MessageBatch, error) {
	var batch service.MessageBatch

	for _, msg := range batchMsg.Messages() {
		if benthosMsg, ok := msg.(*wombatwisdom.BenthosMessage); ok {
			batch = append(batch, benthosMsg.Message)
		} else {
			return nil, errors.New("failed to convert spec message to Benthos format")
		}
	}
	return batch, nil
}

func (i *input) Close(backgroundCtx context.Context) error {
	if i.input == nil {
		return nil
	}

	i.compCancel()

	// Close with the persistent context
	return i.input.Close(i.compCtx)

}
