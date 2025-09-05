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
		Summary("Pushes messages to an MQTT broker.").
		Description(`
Uses mqtt output component found in [wombatwisdom/components](https://github.com/wombatwisdom/components). 

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
		).Description("Authentication configuration").Optional())
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
	mctx := wombatwisdom.NewComponentContext(context.Background(), mgr.Logger())
	wo, err := mqtt.NewOutput(env, outputConfig)
	if err != nil {
		return nil, bp, 0, fmt.Errorf("failed to create wombatwisdom MQTT output: %w", err)
	}

	return &output{
		logger:   mgr.Logger(),
		mctx:     mctx,
		wwOutput: wo,
	}, bp, 1, nil
}

// wwMQTT3Output provides seamless integration between Benthos and wombatwisdom MQTT v3.1.1 output
type output struct {
	logger *service.Logger

	// wombatwisdom components
	wwOutput *mqtt.Output
	mctx     *wombatwisdom.ComponentContext
}

func (w *output) Connect(ctx context.Context) error {
	err := w.wwOutput.Init(w.mctx)
	return translateConnectError(err)
}

func (w *output) WriteBatch(ctx context.Context, batch service.MessageBatch) error {
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

	err := w.wwOutput.Write(w.mctx, w.mctx.NewBatch(msgs...))
	return translateWriteError(err)
}

func (w *output) Close(ctx context.Context) error {
	if w.wwOutput == nil {
		return nil
	}

	// Close errors are typically not critical and don't need translation
	// as the component is shutting down anyway
	return w.wwOutput.Close(w.mctx)
}
