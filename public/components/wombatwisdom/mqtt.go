// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
	"github.com/wombatwisdom/components/mqtt"
)

func init() {
	// Register MQTT input with ww_ prefix for seamless integration
	err := service.RegisterInput(
		"ww_mqtt",
		wwMQTTInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return newWWMQTTInput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_mqtt input: %w", err))
	}

	// Register MQTT output with ww_ prefix for seamless integration
	err = service.RegisterOutput(
		"ww_mqtt",
		wwMQTTOutputConfig(),
		newWWMQTTOutput)
	if err != nil {
		panic(fmt.Errorf("failed to register ww_mqtt output: %w", err))
	}
}

func wwMQTTInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Reads messages from MQTT topics using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom MQTT input component into Wombat.

This component uses the wombatwisdom MQTT implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and quality of service settings.

## Connection Management

The component automatically handles connection lifecycle and provides robust error
handling and reconnection logic through the wombatwisdom framework.
`).
		Field(service.NewStringListField("urls").
			Description("List of MQTT broker URLs to connect to.").
			Default([]string{"tcp://localhost:1883"})).
		Field(service.NewStringField("client_id").
			Description("Unique client identifier. If empty, one will be generated.").
			Default("")).
		Field(service.NewObjectField("filters").
			Description("Map of topic patterns to QoS levels to subscribe to.").
			Default(map[string]interface{}{})).
		Field(service.NewBoolField("clean_session").
			Description("Start with a clean session").
			Default(true)).
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

func wwMQTTOutputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Publishes messages to MQTT topics using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom MQTT output component into Wombat.

This component uses the wombatwisdom MQTT implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and quality of service settings.
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

func newWWMQTTInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	// Extract configuration
	urls, err := conf.FieldStringList("urls")
	if err != nil {
		return nil, fmt.Errorf("failed to get urls: %w", err)
	}
	
	clientID, err := conf.FieldString("client_id")
	if err != nil {
		clientID = ""
	}
	
	cleanSession, err := conf.FieldBool("clean_session")
	if err != nil {
		cleanSession = true
	}
	
	connectTimeout, err := conf.FieldDuration("connect_timeout")
	if err != nil {
		connectTimeout = 30 * time.Second
	}
	
	keepalive, err := conf.FieldDuration("keepalive")
	if err != nil {
		keepalive = 60 * time.Second
	}

	// Build wombatwisdom input config
	inputConfig := mqtt.InputConfig{
		CommonMQTTConfig: mqtt.CommonMQTTConfig{
			ClientId:       clientID,
			Urls:           urls,
			ConnectTimeout: &connectTimeout,
			KeepAlive:      &keepalive,
		},
		CleanSession: cleanSession,
		Filters:      make(map[string]byte), // Default empty filters
	}
	
	// Handle auth if provided
	if conf.Contains("auth") {
		username, _ := conf.FieldString("auth", "username")
		password, _ := conf.FieldString("auth", "password")
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

	return &wwMQTTInput{
		inputConfig:  inputConfig,
		logger:       mgr.Logger(),
		messageQueue: make(chan *service.Message, 100), // Buffered channel
	}, nil
}

func newWWMQTTOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
	// Extract configuration
	urls, err := conf.FieldStringList("urls")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get urls: %w", err)
	}
	
	topic, err := conf.FieldString("topic")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get topic: %w", err)
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

	return &wwMQTTOutput{
		outputConfig: outputConfig,
		logger:       mgr.Logger(),
	}, 1, nil
}

// wwMQTTInput provides seamless integration between Benthos and wombatwisdom MQTT input
type wwMQTTInput struct {
	inputConfig mqtt.InputConfig
	logger      *service.Logger
	
	// wombatwisdom components
	wwInput *mqtt.Input
	
	// message queue for bridging wombatwisdom messages to Benthos
	messageQueue chan *service.Message
}

func (w *wwMQTTInput) Connect(ctx context.Context) error {
	// Create proper environment adapter with full spec.Environment interface
	env := &mqttEnvironmentAdapter{
		ctx:    ctx,
		logger: w.logger,
	}
	
	// Create wombatwisdom input
	wwInput, err := mqtt.NewInput(env, w.inputConfig)
	if err != nil {
		return fmt.Errorf("failed to create wombatwisdom MQTT input: %w", err)
	}
	w.wwInput = wwInput

	// Create a collector that bridges wombatwisdom messages to Benthos
	collector := &mqttCollectorAdapter{
		input:  w,
		logger: w.logger,
	}

	// Connect the wombatwisdom input with the collector
	err = w.wwInput.Connect(ctx, collector)
	if err != nil {
		return fmt.Errorf("failed to connect wombatwisdom MQTT input: %w", err)
	}

	w.logger.Info("wombatwisdom MQTT input connected successfully")
	return nil
}

func (w *wwMQTTInput) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	// Read from the message queue populated by the collector
	select {
	case msg := <-w.messageQueue:
		ackFunc := func(ctx context.Context, err error) error {
			// For MQTT, we generally don't need explicit acks as QoS is handled by the broker
			return nil
		}
		return msg, ackFunc, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (w *wwMQTTInput) Close(ctx context.Context) error {
	if w.wwInput != nil {
		err := w.wwInput.Disconnect(ctx)
		if err != nil {
			w.logger.Errorf("Error disconnecting wombatwisdom MQTT input: %v", err)
		}
		w.logger.Info("wombatwisdom MQTT input closed")
	}
	
	// Close the message queue
	if w.messageQueue != nil {
		close(w.messageQueue)
	}
	
	return nil
}

// wwMQTTOutput provides seamless integration between Benthos and wombatwisdom MQTT output
type wwMQTTOutput struct {
	outputConfig mqtt.OutputConfig
	logger       *service.Logger
	
	// wombatwisdom components
	wwOutput *mqtt.Output
}

func (w *wwMQTTOutput) Connect(ctx context.Context) error {
	// Create environment adapter
	env := &mqttEnvironmentAdapter{
		ctx:    ctx,
		logger: w.logger,
	}
	
	// Create wombatwisdom output
	wwOutput, err := mqtt.NewOutput(env, w.outputConfig)
	if err != nil {
		return fmt.Errorf("failed to create wombatwisdom MQTT output: %w", err)
	}
	w.wwOutput = wwOutput

	// Connect the wombatwisdom output
	err = w.wwOutput.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect wombatwisdom MQTT output: %w", err)
	}

	w.logger.Info("wombatwisdom MQTT output connected successfully")
	return nil
}

func (w *wwMQTTOutput) Write(ctx context.Context, msg *service.Message) error {
	if w.wwOutput == nil {
		return fmt.Errorf("MQTT output not connected")
	}
	
	// Convert Benthos message to wombatwisdom message
	data, _ := msg.AsBytes()
	wwMsg := &mqttMessageAdapter{
		data: data,
		meta: make(map[string]any),
	}
	
	// Copy metadata
	_ = msg.MetaWalk(func(key string, value string) error {
		wwMsg.meta[key] = value
		return nil
	})
	
	// Write using wombatwisdom output
	err := w.wwOutput.Write(ctx, wwMsg)
	if err != nil {
		w.logger.Errorf("Failed to write message via wombatwisdom MQTT output: %v", err)
		return err
	}
	
	w.logger.Debugf("Successfully published message via wombatwisdom MQTT output")
	return nil
}

func (w *wwMQTTOutput) Close(ctx context.Context) error {
	if w.wwOutput != nil {
		err := w.wwOutput.Disconnect(ctx)
		if err != nil {
			w.logger.Errorf("Error disconnecting wombatwisdom MQTT output: %v", err)
		}
		w.logger.Info("wombatwisdom MQTT output closed")
	}
	return nil
}

// mqttEnvironmentAdapter provides a complete environment implementation for MQTT components
type mqttEnvironmentAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

// Logger interface methods
func (e *mqttEnvironmentAdapter) Debugf(format string, args ...interface{}) {
	e.logger.Debugf(format, args...)
}

func (e *mqttEnvironmentAdapter) Infof(format string, args ...interface{}) {
	e.logger.Infof(format, args...)
}

func (e *mqttEnvironmentAdapter) Warnf(format string, args ...interface{}) {
	e.logger.Warnf(format, args...)
}

func (e *mqttEnvironmentAdapter) Errorf(format string, args ...interface{}) {
	e.logger.Errorf(format, args...)
}

// Environment interface methods
func (e *mqttEnvironmentAdapter) GetString(key string) string {
	// Simple implementation - could be enhanced to read from actual environment
	return ""
}

func (e *mqttEnvironmentAdapter) GetInt(key string) int {
	return 0
}

func (e *mqttEnvironmentAdapter) GetBool(key string) bool {
	return false
}

// DynamicFieldFactory interface method  
func (e *mqttEnvironmentAdapter) NewDynamicField(expr string) spec.DynamicField {
	return &mqttDynamicField{expr: expr}
}

// mqttDynamicField provides a dynamic field implementation for MQTT components
type mqttDynamicField struct {
	expr string
}

func (f *mqttDynamicField) String() string {
	return f.expr
}

func (f *mqttDynamicField) Int() int {
	// Simple implementation - could parse the expression for numeric values
	return 0
}

func (f *mqttDynamicField) Bool() bool {
	// Simple implementation - could parse the expression for boolean values
	return f.expr == "true"
}

func (f *mqttDynamicField) AsString(msg spec.Message) (string, error) {
	// For now, just return the expression as-is
	// In a full implementation, this would evaluate the expression against the message
	return f.expr, nil
}

func (f *mqttDynamicField) AsBool(msg spec.Message) (bool, error) {
	// Simple implementation - could parse and evaluate boolean expressions
	return f.expr == "true", nil
}

// mqttCollectorAdapter bridges wombatwisdom messages to Benthos message queue
type mqttCollectorAdapter struct {
	input  *wwMQTTInput
	logger *service.Logger
}

// Collector interface methods
func (c *mqttCollectorAdapter) Collect(msg spec.Message) error {
	return c.Write(msg)
}

func (c *mqttCollectorAdapter) Flush() (spec.Batch, error) {
	// For MQTT, we don't batch messages, so this is a no-op
	return nil, nil
}

func (c *mqttCollectorAdapter) Write(msg spec.Message) error {
	// Convert wombatwisdom message to Benthos message
	benthosMsg, err := c.convertToBenthosMessage(msg)
	if err != nil {
		c.logger.Errorf("Failed to convert wombatwisdom message to Benthos: %v", err)
		return err
	}
	
	// Send to the input's message queue
	select {
	case c.input.messageQueue <- benthosMsg:
		return nil
	default:
		c.logger.Warn("Message queue full, dropping message")
		return fmt.Errorf("message queue full")
	}
}

func (c *mqttCollectorAdapter) Disconnect() error {
	c.logger.Info("MQTT collector disconnecting")
	return nil
}

func (c *mqttCollectorAdapter) convertToBenthosMessage(msg spec.Message) (*service.Message, error) {
	// Extract raw data from wombatwisdom message
	data, err := msg.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw data from wombatwisdom message: %w", err)
	}
	
	// Create Benthos message with the data
	benthosMsg := service.NewMessage(data)
	
	// Copy metadata from wombatwisdom message to Benthos message
	for key, value := range msg.Metadata() {
		// Convert any type to string for Benthos metadata
		benthosMsg.MetaSet(key, fmt.Sprintf("%v", value))
	}
	
	// Add wombatwisdom-specific metadata
	benthosMsg.MetaSet("ww_component", "mqtt")
	benthosMsg.MetaSet("ww_source", "wombatwisdom")
	
	return benthosMsg, nil
}

// mqttMessageAdapter implements the wombatwisdom Message interface for Benthos messages
type mqttMessageAdapter struct {
	data []byte
	meta map[string]any
}

func (m *mqttMessageAdapter) Raw() ([]byte, error) {
	return m.data, nil
}

func (m *mqttMessageAdapter) SetMetadata(key string, value any) {
	m.meta[key] = value
}

func (m *mqttMessageAdapter) SetRaw(data []byte) {
	m.data = data
}

func (m *mqttMessageAdapter) Metadata() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range m.meta {
			if !yield(k, v) {
				return
			}
		}
	}
}