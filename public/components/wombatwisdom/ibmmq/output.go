package ibmmq

import (
	"context"
	"fmt"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	ibmmq "github.com/wombatwisdom/components/bundles/ibm-mq"
	"github.com/wombatwisdom/components/framework/spec"

	"github.com/wombatwisdom/wombat/public/components/wombatwisdom"
)

const (
	fldOutputQueueName  = "queue_name"
	fldQueueExpr        = "queue_expr"
	fldOutputNumThreads = "num_threads"
	fldWriteTimeout     = "write_timeout"
	fldMetadata         = "metadata"
	fldMetaPatterns     = "patterns"
	fldMetaInvert       = "invert"
)

func outputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Pushes messages to IBM MQ queues.").
		Description(`
Pushes messages to IBM MQ queues.

Uses IBM MQ output component found in [wombatwisdom/components](https://github.com/wombatwisdom/components).

## Build Requirements

This component requires IBM MQ client libraries. Build with the ` + "`mqclient`" + ` build tag:

` + "```bash" + `
go build -tags mqclient
` + "```" + `

### Example: Basic configuration

` + "```yaml" + `
output:
  ww_ibm_mq:
    queue_manager_name: QM1
    queue_name: DEV.QUEUE.1
    channel_name: DEV.APP.SVRCONN
    connection_name: localhost(1414)
` + "```" + `

### Example: With dynamic queue routing

` + "```yaml" + `
output:
  ww_ibm_mq:
    queue_manager_name: QM1
    queue_expr: '${! meta("target_queue") }'
    channel_name: DEV.APP.SVRCONN
    connection_name: localhost(1414)
` + "```" + `

### Example: With TLS/SSL

` + "```yaml" + `
output:
  ww_ibm_mq:
    queue_manager_name: QM1
    queue_name: DEV.QUEUE.1
    channel_name: DEV.APP.SVRCONN
    connection_name: secure-mq.example.com(1414)
    tls:
      enabled: true
      cipher_spec: TLS_RSA_WITH_AES_256_CBC_SHA256
      key_repository: /opt/mqm/ssl/key
` + "```" + `
`).
		Field(service.NewStringField(fldQueueManagerName).
			Description("The IBM MQ Queue Manager name to connect to").
			Example("QM1")).
		Field(service.NewStringField(fldOutputQueueName).
			Description("The IBM MQ queue name to write messages to").
			Example("DEV.QUEUE.1").
			Default("")).
		Field(service.NewInterpolatedStringField(fldQueueExpr).
			Description("An expression to dynamically determine the queue name based on message content. If set, this overrides the static queue_name for each message.").
			Example(`${! meta("target_queue") }`).
			Optional()).
		Field(service.NewStringField(fldChannelName).
			Description("The IBM MQ channel name for client connections").
			Example("DEV.APP.SVRCONN")).
		Field(service.NewStringField(fldConnectionName).
			Description("The IBM MQ connection name in the format hostname(port)").
			Example("localhost(1414)")).
		Field(service.NewStringField(fldUserId).
			Description("Optional: The IBM MQ user ID for authentication").
			Default("").
			Optional()).
		Field(service.NewStringField(fldPassword).
			Description("Optional: The IBM MQ user password for authentication").
			Default("").
			Optional().
			Secret()).
		Field(service.NewStringField(fldApplicationName).
			Description("Optional: Application name for MQ connection identification").
			Default("wombat").
			Optional()).
		Field(service.NewIntField(fldOutputNumThreads).
			Description("Number of parallel queue connections to use").
			Default(1).
			Optional()).
		Field(service.NewDurationField(fldWriteTimeout).
			Description("Timeout for write operations").
			Default("30s").
			Optional()).
		Field(service.NewObjectField(fldMetadata,
			service.NewStringListField(fldMetaPatterns).
				Description("Patterns to match metadata fields").
				Default([]string{}).
				Optional(),
			service.NewBoolField(fldMetaInvert).
				Description("If true, exclude matching patterns; if false, include only matching patterns").
				Default(false).
				Optional(),
		).Description("Metadata configuration for filtering message headers").
			Optional()).
		Field(service.NewObjectField(fldTLS,
			service.NewBoolField(fldTLSEnabled).
				Description("Enable TLS encryption for the connection").
				Default(false),
			service.NewStringField(fldTLSCipherSpec).
				Description("The cipher specification to use for TLS").
				Example("TLS_RSA_WITH_AES_256_CBC_SHA256").
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
			Optional())
}

type output struct {
	outputConfig *ibmmq.OutputConfig
	queueExpr    *service.InterpolatedString
	output       *ibmmq.Output
	logger       *service.Logger
	mgr          *service.Resources
	writeTimeout time.Duration
	compCtx      *wombatwisdom.ComponentContext
	compCancel   context.CancelFunc
}

func newOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
	bp := service.BatchPolicy{Count: 1}

	queueManagerName, err := conf.FieldString(fldQueueManagerName)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get queue_manager_name: %w", err)
	}

	queueName, _ := conf.FieldString(fldOutputQueueName)

	// Get dynamic queue expression if provided
	var queueExpr *service.InterpolatedString
	if conf.Contains(fldQueueExpr) {
		queueExpr, err = conf.FieldInterpolatedString(fldQueueExpr)
		if err != nil {
			return nil, bp, 0, fmt.Errorf("failed to parse queue_expr: %w", err)
		}
	}

	// Validate that we have either queue_name or queue_expr
	if queueName == "" && queueExpr == nil {
		return nil, bp, 0, fmt.Errorf("either queue_name or queue_expr must be specified")
	}

	channelName, err := conf.FieldString(fldChannelName)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get channel_name: %w", err)
	}

	connectionName, err := conf.FieldString(fldConnectionName)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get connection_name: %w", err)
	}

	userId, _ := conf.FieldString(fldUserId)
	password, _ := conf.FieldString(fldPassword)
	applicationName, _ := conf.FieldString(fldApplicationName)
	if applicationName == "" {
		applicationName = "wombat"
	}

	numThreads, _ := conf.FieldInt(fldOutputNumThreads)
	if numThreads <= 0 {
		numThreads = 1
	}

	writeTimeout, _ := conf.FieldDuration(fldWriteTimeout)
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}

	// Extract metadata configuration if present
	var metaConfig *ibmmq.MetadataConfig
	if conf.Contains(fldMetadata) {
		if metaRaw, err := conf.FieldAny(fldMetadata); err == nil {
			if metaMap, ok := metaRaw.(map[string]interface{}); ok {
				var patterns []string
				if patternsRaw, ok := metaMap[fldMetaPatterns].([]interface{}); ok {
					for _, p := range patternsRaw {
						if ps, ok := p.(string); ok {
							patterns = append(patterns, ps)
						}
					}
				}
				invert, _ := metaMap[fldMetaInvert].(bool)
				if len(patterns) > 0 {
					metaConfig = &ibmmq.MetadataConfig{
						Patterns: patterns,
						Invert:   invert,
					}
				}
			}
		}
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

	// Create the IBM MQ output configuration
	outputConfig := &ibmmq.OutputConfig{
		CommonMQConfig: ibmmq.CommonMQConfig{
			QueueManagerName: queueManagerName,
			ChannelName:      channelName,
			ConnectionName:   connectionName,
			UserId:           userId,
			Password:         password,
			ApplicationName:  applicationName,
		},
		QueueName:  queueName,
		NumThreads: numThreads,
		Metadata:   metaConfig,
	}

	o := &output{
		outputConfig: outputConfig,
		queueExpr:    queueExpr,
		logger:       mgr.Logger(),
		mgr:          mgr,
		writeTimeout: writeTimeout,
	}

	env := wombatwisdom.NewEnvironment(o.logger)
	output, err := ibmmq.NewOutput(env, *o.outputConfig)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to create IBM MQ output: %w", err)
	}
	o.output = output

	return o, bp, 1, nil
}

func (o *output) Connect(closeAtLeisureCtx context.Context) error {
	ctx, cancel := context.WithCancel(closeAtLeisureCtx)
	o.compCtx = wombatwisdom.NewComponentContext(ctx, o.logger)
	o.compCancel = cancel

	err := o.output.Init(o.compCtx)
	if err != nil {
		o.logger.Error("Failed to connect to output")
	}
	return translateError(err)
}

func (o *output) WriteBatch(writeRequestCtx context.Context, batch service.MessageBatch) error {
	if o.output == nil {
		return service.ErrNotConnected
	} // triggers retry

	msgFactory := &wombatwisdom.MessageFactory{}
	var specMessages []spec.Message

	for _, msg := range batch {
		wwMsg := &wombatwisdom.BenthosMessage{Message: msg}
		specMessages = append(specMessages, wwMsg)
	}

	specBatch := msgFactory.NewBatch(specMessages...)

	err := o.output.Write(o.compCtx, specBatch)

	return translateError(err)
}

func (o *output) Close(backgroundCtx context.Context) error {
	if o.output == nil {
		return nil
	}
	o.compCancel()
	return o.output.Close(o.compCtx)
}
