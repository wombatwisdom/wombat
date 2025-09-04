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

func outputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Publishes messages to MQTT v3.1.1 topics using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom MQTT v3.1.1 output component into Wombat.

This component uses the wombatwisdom MQTT v3.1.1 implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and quality of service settings.

## MQTT Version

This component supports MQTT v3.1.1 protocol. For MQTT v5 support, use ww_mqtt_5 (coming soon).
`).
		Field(service.NewStringListField("urls").
			Description("List of MQTT broker URLs to connect to.").
			Default([]string{"tcp://localhost:1883"})).
		Field(service.NewStringField("client_id").
			Description("Unique client identifier. If empty, one will be generated.").
			Default("")).
		Field(service.NewStringField("topic").
			Description("Topic to publish to. Can contain interpolation functions.")).
		Field(service.NewIntField("qos").
			Description("Quality of Service level (0, 1, or 2)").
			Default(0)).
		Field(service.NewBoolField("retained").
			Description("Set the retained flag on published messages").
			Default(false)).
		Field(service.NewDurationField("write_timeout").
			Description("Timeout for write operations").
			Default("5s")).
		Field(service.NewDurationField("connect_timeout").
			Description("Connection timeout").
			Default("30s")).
		Field(service.NewDurationField("keepalive").
			Description("Keep alive interval").
			Default("60s")).
		Field(service.NewObjectField("auth",
			service.NewStringField("username").Description("Username for authentication").Default(""),
			service.NewStringField("password").Description("Password for authentication").Default(""),
		).Description("Authentication configuration").Optional()).
		Field(service.NewObjectField("tls",
			service.NewBoolField("enabled").Description("Enable TLS").Default(false),
			service.NewBoolField("skip_cert_verify").Description("Skip certificate verification").Default(false),
		).Description("TLS configuration").Optional()).
		Field(service.NewObjectField("will",
			service.NewStringField("topic").Description("Will message topic").Default(""),
			service.NewStringField("payload").Description("Will message payload").Default(""),
			service.NewIntField("qos").Description("Will message QoS").Default(0),
			service.NewBoolField("retained").Description("Will message retained flag").Default(false),
		).Description("Last will and testament configuration").Optional())
}

func newOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.BatchOutput, service.BatchPolicy, int, error) {
	bp := service.BatchPolicy{Count: 1}

	// Extract configuration
	urls, err := conf.FieldStringList("urls")
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get urls: %w", err)
	}

	topic, err := conf.FieldString("topic")
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to get topic: %w", err)
	}

	clientID, err := conf.FieldString("client_id")
	if err != nil {
		clientID = ""
	}

	qos, err := conf.FieldInt("qos")
	if err != nil {
		qos = 0
	}

	retained, err := conf.FieldBool("retained")
	if err != nil {
		retained = false
	}

	writeTimeout, err := conf.FieldDuration("write_timeout")
	if err != nil {
		writeTimeout = 5 * time.Second
	}

	connectTimeout, err := conf.FieldDuration("connect_timeout")
	if err != nil {
		connectTimeout = 30 * time.Second
	}

	keepalive, err := conf.FieldDuration("keepalive")
	if err != nil {
		keepalive = 60 * time.Second
	}

	// Build wombatwisdom output config
	outputConfig := mqtt.OutputConfig{
		CommonMQTTConfig: mqtt.CommonMQTTConfig{
			ClientId:       clientID,
			Urls:           urls,
			ConnectTimeout: &connectTimeout,
			KeepAlive:      &keepalive,
		},
		TopicExpr:    topic,
		WriteTimeout: writeTimeout,
		QOS:          byte(qos),
		RetainedExpr: fmt.Sprintf("%t", retained), // Convert bool to expression
	}

	// Handle auth if provided
	if conf.Contains("auth") {
		username, _ := conf.FieldString("auth", "username")
		password, _ := conf.FieldString("auth", "password")
		outputConfig.Username = username
		outputConfig.Password = password
	}

	env := wombatwisdom.NewEnvironment(mgr.Logger())
	wo, err := mqtt.NewOutput(env, outputConfig)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to create wombatwisdom MQTT output: %w", err)
	}

	return &output{
		logger:   mgr.Logger(),
		wwOutput: wo,
	}, bp, 1, nil
}

// wwMQTT3Output integration between Benthos and wombatwisdom MQTT v3.1.1 output
type output struct {
	logger   *service.Logger
	wwOutput *mqtt.Output
}

func (w *output) Connect(ctx context.Context) error {
	err := w.wwOutput.Init(wombatwisdom.NewComponentContext(ctx, w.logger))
	return translateConnectError(err)
}

func (w *output) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
	if w.wwOutput == nil {
		return service.ErrNotConnected
	}

	writeCtx := wombatwisdom.NewComponentContext(ctx, w.logger)
	var msgs []spec.Message
	for _, bmsg := range batch {
		msg := &wombatwisdom.BenthosMessage{
			Message: bmsg,
		}
		msgs = append(msgs, msg)
	}

	err := w.wwOutput.Write(writeCtx, writeCtx.NewBatch(msgs...))
	return translateWriteError(err)
}

func (w *output) Close(ctx context.Context) error {
	if w.wwOutput == nil {
		return nil
	}
	
	return w.wwOutput.Close(wombatwisdom.NewComponentContext(ctx, w.logger))
}
