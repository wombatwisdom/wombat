package mqtt3

import (
	"context"
	"fmt"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/bundles/mqtt"

	"github.com/wombatwisdom/wombat/public/components/wombatwisdom"
)

const (
	fldUrls                 = "urls"
	fldClientID             = "client_id"
	fldFilters              = "filters"
	fldCleanSession         = "clean_session"
	fldConnectTimeout       = "connect_timeout"
	fldConnectRetry         = "connect_retry"
	fldConnectRetryInterval = "connect_retry_interval"
	fldKeepalive            = "keepalive"
	fldAuth                 = "auth"
	fldTLS                  = "tls"
	fldWill                 = "will"
	fldEnableAutoAck        = "enable_auto_ack"
	fldUsername             = "username"
	fldPassword             = "password"
	fldTopic                = "topic"
	fldPayload              = "payload"
	fldQOS                  = "qos"
	fldRetained             = "retained"
)

func inputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Subscribe to topics on MQTT brokers.").
		Description(`
Uses mqtt input component found in [wombatwisdom/components](https://github.com/wombatwisdom/components). It differs from the ` + "`mqtt`" + `
input in that it exposes ` + "`enable_auto_ack`" + `, which is set to ` + "false" + ` by default.

## MQTT Version

This component supports MQTT v3.1.1 protocol. For MQTT v5 support, use ww_mqtt_5 (coming soon).

## Delivery guarantees

By default, this input disables auto acknowledgment to ensure at-least-once delivery semantics.

If message loss is acceptable, you can set ` + "`enable_auto_ack: true`" + ` to enable at-most-once delivery.

### Example: At least once

` + "```yaml" + `
input:
  ww_mqtt_3:
    urls:
      - tcp://localhost:1883
    filters:
      sensor/+/data: 1
    clean_session: false
    enable_auto_ack: false # Ack will happen once output is done processing.
` + "```" + `

### Example: at most once

` + "```yaml" + `
input:
  ww_mqtt_3:
    urls:
      - tcp://localhost:1883
    filters:
      metrics/#: 0
    clean_session: true
    enable_auto_ack: true # Ack will happen once input receives message
` + "```" + `
`).
		Field(service.NewStringListField(fldUrls).
			Description("List of MQTT broker URLs to connect to.").
			Default([]string{"tcp://localhost:1883"})).
		Field(service.NewStringField(fldClientID).
			Description("Unique client identifier. If empty, one will be generated.").
			Default("")).
		Field(service.NewObjectField(fldFilters).
			Description("Map of topic patterns to QoS levels to subscribe to.").
			Default(map[string]interface{}{})).
		Field(service.NewBoolField(fldCleanSession).
			Description("Start with a clean session").
			Default(true)).
		Field(service.NewDurationField(fldConnectTimeout).
			Description("Connection timeout").
			Default("5s")).
		Field(service.NewBoolField(fldConnectRetry).
			Description("Connect retry").
			Default(true)).
		Field(service.NewDurationField(fldConnectRetryInterval).
			Description("Connection retry interval").
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
		).Description("Last Will and Testament configuration").Optional()).
		Field(service.NewBoolField(fldEnableAutoAck).
			Description("Enable automatic acknowledgment (paho SetAutoAckDisabled). When false (default), messages are ACK'd after processing (at-least-once). When true, messages are ACK'd immediately (at-most-once with higher throughput but message loss risk).").
			Default(false).
			Advanced())
}

func newInput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchInput, error) {
	// Extract configuration
	urls, err := conf.FieldStringList(fldUrls)
	if err != nil {
		return nil, fmt.Errorf("failed to get urls: %w", err)
	}

	clientID, err := conf.FieldString(fldClientID)
	if err != nil {
		// If not specified, use default empty string
		clientID = ""
	}

	cleanSession, err := conf.FieldBool(fldCleanSession)
	if err != nil {
		cleanSession = true
	}

	inputConfig := mqtt.InputConfig{
		CommonMQTTConfig: mqtt.CommonMQTTConfig{
			ClientId: clientID,
			Urls:     urls,
		},
		CleanSession: cleanSession,
		Filters:      make(map[string]byte),
	}

	if conf.Contains(fldConnectTimeout) {
		d, err := conf.FieldDuration(fldConnectTimeout)
		if err != nil {
			d = 5 * time.Second
		}

		inputConfig.ConnectTimeout = &d
	}

	if conf.Contains(fldConnectRetry) {
		b, err := conf.FieldBool(fldConnectRetry)
		if err != nil {
			b = true
		}

		inputConfig.ConnectRetry = b
	}

	if conf.Contains(fldConnectRetryInterval) {
		d, err := conf.FieldDuration(fldConnectRetryInterval)
		if err != nil {
			d = 1 * time.Second
		}

		inputConfig.ConnectRetryInterval = d
	}

	if conf.Contains(fldKeepalive) {
		d, err := conf.FieldDuration(fldKeepalive)
		if err != nil {
			d = 60 * time.Second
		}

		inputConfig.KeepAlive = &d
	}

	// Extract auto ACK settings, if not specified, NewInput will set the default to true
	if conf.Contains(fldEnableAutoAck) {
		enableAutoAck, err := conf.FieldBool(fldEnableAutoAck)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", fldEnableAutoAck, err)
		}
		inputConfig.EnableAutoAck = enableAutoAck
	}

	// Handle auth if provided
	if conf.Contains("auth") {
		username, _ := conf.FieldString(fldAuth, fldUsername)
		password, _ := conf.FieldString(fldAuth, fldPassword)
		inputConfig.Username = username
		inputConfig.Password = password
	}

	// Handle TLS if provided
	tlsConf, tlsEnabled, err := conf.FieldTLSToggled(fldTLS)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS config: %w", err)
	}
	if tlsEnabled {
		inputConfig.TLS = tlsConf
	}

	// Handle Will if provided
	if conf.Contains(fldWill) {
		willTopic, _ := conf.FieldString(fldWill, fldTopic)
		willPayload, _ := conf.FieldString(fldWill, fldPayload)
		willQos, _ := conf.FieldInt(fldWill, fldQOS)
		willRetained, _ := conf.FieldBool(fldWill, fldRetained)

		inputConfig.Will = &mqtt.WillConfig{
			Topic:    willTopic,
			Payload:  willPayload,
			QoS:      uint8(willQos),
			Retained: willRetained,
		}
	}

	// Handle filters
	if filtersRaw, err := conf.FieldAny("filters"); err == nil {
		if filtersMap, ok := filtersRaw.(map[string]interface{}); ok {
			filters := make(map[string]byte)
			for topic, qosRaw := range filtersMap {
				if qosInt, ok := qosRaw.(int); ok {
					filters[topic] = byte(qosInt)
				} else {
					filters[topic] = 0 // Default QoS
				}
			}
			inputConfig.Filters = filters
		}
	}

	if len(inputConfig.Filters) == 0 {
		return nil, fmt.Errorf("MQTT input requires at least one topic filter to be configured")
	}

	input := &input{
		inputConfig: inputConfig,
		logger:      mgr.Logger(),
	}

	env := wombatwisdom.NewEnvironment(mgr.Logger())

	input.wwInput, err = mqtt.NewInput(env, inputConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create wombatwisdom MQTT input: %w", err)
	}

	return input, nil
}

// input integration between Benthos and wombatwisdom MQTT v3.1.1 input
type input struct {
	inputConfig mqtt.InputConfig
	logger      *service.Logger
	wwInput     *mqtt.Input
	compCtx     *wombatwisdom.ComponentContext
	compCancel  context.CancelFunc
}

func (w *input) Connect(closeAtLeisureCtx context.Context) error {
	ctx, cancel := context.WithCancel(closeAtLeisureCtx)
	w.compCtx = wombatwisdom.NewComponentContext(ctx, w.logger)
	w.compCancel = cancel

	err := w.wwInput.Init(w.compCtx)

	return translateConnectError(err)
}

func (w *input) ReadBatch(closeAtLeisureCtx context.Context) (service.MessageBatch, service.AckFunc, error) {
	batch, cb, err := w.wwInput.Read(w.compCtx)
	if err != nil {
		return nil, nil, translateReadError(err)
	}

	var result service.MessageBatch
	for _, msg := range batch.Messages() {
		bmsg := msg.(*wombatwisdom.BenthosMessage)

		result = append(result, bmsg.Message)
	}

	return result, func(closeNowCtx context.Context, err error) error {
		return cb(closeNowCtx, err)
	}, nil
}

func (w *input) Close(backgroundCtx context.Context) error {
	if w.wwInput == nil {
		return nil
	}

	w.compCancel()

	// Close with the persistent context
	return w.wwInput.Close(w.compCtx)
}
