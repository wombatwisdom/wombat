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
	fldUrls           = "urls"
	fldClientID       = "client_id"
	fldFilters        = "filters"
	fldCleanSession   = "clean_session"
	fldConnectTimeout = "connect_timeout"
	fldKeepalive      = "keepalive"
	fldAuth           = "auth"
	fldTLS            = "tls"
	fldWill           = "will"
	fldEnableAutoAck  = "enable_auto_ack"
	fldUsername       = "username"
	fldPassword       = "password"
	fldTopic          = "topic"
	fldPayload        = "payload"
	fldQOS            = "qos"
	fldRetained       = "retained"
)

func inputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Reads messages from MQTT v3.1.1 topics using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom MQTT v3.1.1 input component into Wombat.

This component uses the wombatwisdom MQTT v3.1.1 implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and quality of service settings.

## MQTT Version

This component supports MQTT v3.1.1 protocol. For MQTT v5 support, use ww_mqtt_5 (coming soon).

## Connection Management

The component automatically handles connection lifecycle and provides robust error
handling and reconnection logic through the wombatwisdom framework.

## Acknowledgment Control (At-Least-Once vs At-Most-Once Delivery)

By default, this component disables auto acknowledgment (set_auto_ack_disabled: true) to ensure
at-least-once delivery semantics. Messages are only acknowledged after successful
pipeline processing. This prevents message loss but limits throughput.

For high-performance scenarios where some message loss is acceptable, you can
set set_auto_ack_disabled: false to enable at-most-once delivery.

### Example: Safe Configuration (Default)

` + "`yaml" + `
input:
  ww_mqtt_3:
    urls:
      - tcp://localhost:1883
    filters:
      sensor/+/data: 1
    # set_auto_ack_disabled: true (default)
    # prefetch_count: 10 (default)
` + "`" + `

### Example: High-Performance Configuration

` + "`yaml" + `
input:
  ww_mqtt_3:
    urls:
      - tcp://localhost:1883
    filters:
      metrics/#: 0
    set_auto_ack_disabled: false  # WARNING: May lose messages!
    clean_session: true
` + "`" + `
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
			Default("30s")).
		Field(service.NewDurationField(fldKeepalive).
			Description("Keep alive interval").
			Default("60s")).
		Field(service.NewObjectField(fldAuth,
			service.NewStringField(fldUsername).Description("Username for authentication").Default(""),
			service.NewStringField(fldPassword).Description("Password for authentication").Default(""),
		).Description("Authentication configuration").Optional()).
		Field(service.NewObjectField(fldTLS,
			service.NewBoolField("enabled").Description("Enable TLS").Default(false),
			service.NewBoolField("skip_cert_verify").Description("Skip certificate verification").Default(false),
		).Description("TLS configuration").Optional()).
		Field(service.NewObjectField(fldWill,
			service.NewStringField(fldTopic).Description("Will message topic").Default(""),
			service.NewStringField(fldPayload).Description("Will message payload").Default(""),
			service.NewIntField(fldQOS).Description("Will message QoS").Default(0),
			service.NewBoolField(fldRetained).Description("Will message retained flag").Default(false),
		).Description("Last will and testament configuration").Optional()).
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
		clientID = ""
	}

	cleanSession, err := conf.FieldBool(fldCleanSession)
	if err != nil {
		cleanSession = true
	}

	// Build wombatwisdom input config
	inputConfig := mqtt.InputConfig{
		CommonMQTTConfig: mqtt.CommonMQTTConfig{
			ClientId: clientID,
			Urls:     urls,
		},
		CleanSession: cleanSession,
		Filters:      make(map[string]byte), // Will be populated from config below
	}

	if conf.Contains(fldConnectTimeout) {
		d, err := conf.FieldDuration(fldConnectTimeout)
		if err != nil {
			d = 30 * time.Second
		}

		inputConfig.CommonMQTTConfig.ConnectTimeout = &d
	}

	if conf.Contains(fldKeepalive) {
		d, err := conf.FieldDuration(fldKeepalive)
		if err != nil {
			d = 60 * time.Second
		}

		inputConfig.CommonMQTTConfig.KeepAlive = &d
	}

	// Extract auto ACK settings
	if conf.Contains(fldEnableAutoAck) {
		enableAutoAck, err := conf.FieldBool(fldEnableAutoAck)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", fldEnableAutoAck, err)
		}
		inputConfig.EnableAutoAck = enableAutoAck
	}
	// If not specified, NewInput will set the default to true

	// Handle auth if provided
	if conf.Contains("auth") {
		username, _ := conf.FieldString(fldAuth, fldUsername)
		password, _ := conf.FieldString(fldAuth, fldPassword)
		inputConfig.Username = username
		inputConfig.Password = password
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
}

func (w *input) contextAdapter(ctx context.Context) *wombatwisdom.ComponentContext {
	return wombatwisdom.NewComponentContext(ctx, w.logger)
}

func (w *input) Connect(ctx context.Context) error {
	err := w.wwInput.Init(w.contextAdapter(ctx))
	return translateConnectError(err)
}

func (w *input) ReadBatch(ctx context.Context) (service.MessageBatch, service.AckFunc, error) {
	batch, cb, err := w.wwInput.Read(w.contextAdapter(ctx))
	if err != nil {
		return nil, nil, translateReadError(err)
	}

	var result service.MessageBatch
	for _, msg := range batch.Messages() {
		bmsg := msg.(*wombatwisdom.BenthosMessage)

		result = append(result, bmsg.Message)
	}

	return result, func(ctx context.Context, err error) error {
		return cb(ctx, err)
	}, nil
}

func (w *input) Close(ctx context.Context) error {
	if w.wwInput == nil {
		return nil
	}

	return w.wwInput.Close(w.contextAdapter(ctx))
}
