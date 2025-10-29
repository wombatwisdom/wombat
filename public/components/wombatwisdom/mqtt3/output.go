package mqtt3

import (
	"context"
	"fmt"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/bundles/mqtt"
	"github.com/wombatwisdom/components/framework/spec"

	"github.com/wombatwisdom/wombat/public/components/wombatwisdom"
)

const (
	fldWriteTimeout = "write_timeout"
)

func outputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Pushes messages to an MQTT broker.").
		Description(`
Uses mqtt output component found in [wombatwisdom/components](https://github.com/wombatwisdom/components). 

`).
		Field(service.NewStringListField(fldUrls).
			Description("List of MQTT broker URLs to connect to.").
			Default([]string{"tcp://localhost:1883"})).
		Field(service.NewStringField(fldClientID).
			Description("Unique client identifier. If empty, one will be generated.").
			Default("")).
		Field(service.NewInterpolatedStringField(fldTopic).
			Description("Topic to publish to. Can contain interpolation functions.")).
		Field(service.NewIntField(fldQOS).
			Description("Quality of Service level (0, 1, or 2)").
			Default(0)).
		Field(service.NewBoolField(fldCleanSession).
			Description("Start with a clean session. When set to true this will set the \"clean session\" flag in the connect message when the underlying MQTT client connects to an MQTT broker. By setting this flag, you are indicating that no messages saved by the broker for this client should be delivered. Any messages that were going to be sent by this client before disconnecting previously but didn't will not be sent upon connecting to the broker. ").
			Default(true).
			Advanced()).
		Field(service.NewBoolField(fldRetained).
			Description("Set the retained flag on published messages").
			Default(false)).
		Field(service.NewDurationField(fldWriteTimeout).
			Description("Timeout for write operations").
			Default("5s")).
		Field(service.NewDurationField(fldConnectTimeout).
			Description("Connection timeout limits how long the client will wait when trying to open a connection to an MQTT server before timing out. A duration of 0 never times out. Defaults to 5 seconds. Currently only operational on TCP/TLS connections.").
			Default("5s")).
		Field(service.NewBoolField(fldConnectRetry).
			Description("Connect retry sets whether the MQTT client will automatically retry the connection in the event of a failure. Defaults to true.").
			Default(true)).
		Field(service.NewDurationField(fldConnectRetryInterval).
			Description("Connection retry interval sets the time that will be waited between connection attempts when initially connecting if connect_retry is set to true.").
			Default("1s")).
		Field(service.NewDurationField(fldKeepalive).
			Description("Keep alive interval").
			Default("60s")).
		Field(service.NewObjectField(fldAuth,
			service.NewStringField(fldUsername).Description("Username for authentication").Default(""),
			service.NewStringField(fldPassword).Description("Password for authentication").Default(""),
		).Description("Authentication configuration").Optional()).
		Field(service.NewTLSToggledField(fldTLS).
			Description("TLS configuration for secure connections").
			Optional()).
		Field(service.NewObjectField(fldWill,
			service.NewStringField(fldTopic).Description("Topic for will message"),
			service.NewStringField(fldPayload).Description("Payload for will message").Default(""),
			service.NewIntField(fldQOS).Description("QoS level for will message (0, 1, or 2)").Default(0),
			service.NewBoolField(fldRetained).Description("Retained flag for will message").Default(false),
		).Description("Last Will and Testament configuration").Optional())
}

func newOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
	bp := service.BatchPolicy{Count: 1}

	// Extract configuration
	urls, err := conf.FieldStringList(fldUrls)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get urls: %w", err)
	}

	// Get the topic string (may contain interpolations)
	topic, err := conf.FieldInterpolatedString(fldTopic)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get topic: %w", err)
	}

	clientID, err := conf.FieldString(fldClientID)
	if err != nil {
		clientID = ""
	}

	qos, err := conf.FieldInt(fldQOS)
	if err != nil {
		qos = 0
	}

	cleanSession, err := conf.FieldBool(fldCleanSession)
	if err != nil {
		cleanSession = true
	}

	retained, err := conf.FieldBool(fldRetained)
	if err != nil {
		retained = false
	}

	writeTimeout, err := conf.FieldDuration(fldWriteTimeout)
	if err != nil {
		writeTimeout = 5 * time.Second
	}

	connectTimeout, err := conf.FieldDuration(fldConnectTimeout)
	if err != nil {
		connectTimeout = 5 * time.Second
	}

	connectRetry, err := conf.FieldBool(fldConnectRetry)
	if err != nil {
		connectRetry = true
	}

	connectRetryInterval, err := conf.FieldDuration(fldConnectRetryInterval)
	if err != nil {
		connectTimeout = 1 * time.Second
	}

	keepalive, err := conf.FieldDuration(fldKeepalive)
	if err != nil {
		keepalive = 60 * time.Second
	}

	// Build wombatwisdom output config
	outputConfig := mqtt.OutputConfig{
		CommonMQTTConfig: mqtt.CommonMQTTConfig{
			ClientId:             clientID,
			Urls:                 urls,
			ConnectTimeout:       &connectTimeout,
			ConnectRetry:         connectRetry,
			ConnectRetryInterval: connectRetryInterval,
			KeepAlive:            &keepalive,
			Password:             "",
			TLS:                  nil,
			Will:                 nil,
		},
		TopicExpr:    wombatwisdom.NewInterpolatedExpression(topic),
		WriteTimeout: writeTimeout,
		Retained:     retained,
		QOS:          byte(qos),
		CleanSession: cleanSession,
	}

	// Handle auth if provided
	if conf.Contains(fldAuth) {
		username, _ := conf.FieldString(fldAuth, fldUsername)
		password, _ := conf.FieldString(fldAuth, fldPassword)
		outputConfig.Username = username
		outputConfig.Password = password
	}

	// Handle TLS if provided
	tlsConf, tlsEnabled, err := conf.FieldTLSToggled(fldTLS)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to parse TLS config: %w", err)
	}
	if tlsEnabled {
		outputConfig.TLS = tlsConf
	}

	// Handle Will if provided
	if conf.Contains(fldWill) {
		willTopic, _ := conf.FieldString(fldWill, fldTopic)
		willPayload, _ := conf.FieldString(fldWill, fldPayload)
		willQos, _ := conf.FieldInt(fldWill, fldQOS)
		willRetained, _ := conf.FieldBool(fldWill, fldRetained)

		outputConfig.Will = &mqtt.WillConfig{
			Topic:    willTopic,
			Payload:  willPayload,
			QoS:      uint8(willQos),
			Retained: willRetained,
		}
	}

	output := &output{
		logger: mgr.Logger(),
	}

	env := wombatwisdom.NewEnvironment(mgr.Logger())
	wo, err := mqtt.NewOutput(env, outputConfig)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to create wombatwisdom MQTT output: %w", err)
	}

	output.wwOutput = wo
	return output, bp, 1, nil
}

// output integration between Benthos and wombatwisdom MQTT v3.1.1 output
type output struct {
	logger     *service.Logger
	wwOutput   *mqtt.Output
	compCtx    *wombatwisdom.ComponentContext
	compCancel context.CancelFunc
}

func (w *output) Connect(closeAtLeisureCtx context.Context) error {
	ctx, cancel := context.WithCancel(closeAtLeisureCtx)

	w.compCtx = wombatwisdom.NewComponentContext(ctx, w.logger)
	w.compCancel = cancel

	err := w.wwOutput.Init(w.compCtx)
	if err != nil {
		w.logger.Error("Failed to connect to output")
	}
	return translateConnectError(err)
}

func (w *output) WriteBatch(writeRequestCtx context.Context, batch service.MessageBatch) error {
	if w.wwOutput == nil {
		return service.ErrNotConnected
	}

	var msgs []spec.Message
	for _, bmsg := range batch {
		msg := &wombatwisdom.BenthosMessage{
			Message: bmsg,
		}
		msgs = append(msgs, msg)
	}

	err := w.wwOutput.Write(w.compCtx, w.compCtx.NewBatch(msgs...))
	return translateWriteError(err)
}

func (w *output) Close(backgroundCtx context.Context) error {
	if w.wwOutput == nil {
		return nil
	}

	// Cancel the component context to signal shutdown
	w.compCancel()

	// Close with the persistent context
	return w.wwOutput.Close(w.compCtx)
}
