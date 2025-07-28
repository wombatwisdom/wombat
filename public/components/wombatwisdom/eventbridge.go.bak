// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"context"
	"fmt"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	eventbridge "github.com/wombatwisdom/components/aws-eventbridge"
	"github.com/wombatwisdom/components/framework/spec"
)

func init() {
	// Register EventBridge input with ww_ prefix for seamless integration
	err := service.RegisterInput(
		"ww_eventbridge",
		wwEventBridgeInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return newWWEventBridgeInput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_eventbridge input: %w", err))
	}
}

func wwEventBridgeInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("AWS", "Services").
		Summary("Reads trigger events from AWS EventBridge using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom EventBridge trigger input component into Wombat.

This component uses the wombatwisdom EventBridge implementation under the hood while providing
a native Benthos interface. It supports multiple integration modes for consuming EventBridge events:

- **SQS Mode**: Uses EventBridge Rules to route events to SQS, then polls SQS
- **Pipes Mode**: Uses EventBridge Pipes for direct streaming integration  
- **Simulation Mode**: Generates fake events for testing purposes

## Authentication

The component supports various AWS authentication methods including:
- IAM roles
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- EC2 instance profiles
- AWS profiles

## Trigger Pattern

This input implements the trigger-retrieval pattern, emitting lightweight trigger events
that reference data locations (like S3 objects) without fetching the actual data.
`).
		Field(service.NewStringField("mode").
			Description("Integration mode: sqs, pipes, or simulation").
			Default("sqs")).
		Field(service.NewStringField("region").
			Description("AWS region for EventBridge").
			Default("us-east-1")).
		Field(service.NewStringField("event_bus_name").
			Description("Name of the EventBridge event bus").
			Default("default")).
		Field(service.NewStringField("rule_name").
			Description("EventBridge rule name (required for SQS mode)").
			Default("")).
		Field(service.NewStringField("event_source").
			Description("Event source to filter on (e.g., aws.s3)")).
		Field(service.NewStringField("detail_type").
			Description("Detail type to filter on (e.g., Object Created)").
			Default("")).
		Field(service.NewObjectField("event_filters").
			Description("Additional event filters as key-value pairs").
			Default(map[string]interface{}{})).
		Field(service.NewIntField("max_batch_size").
			Description("Maximum number of triggers per batch").
			Default(10)).
		Field(service.NewBoolField("enable_dead_letter").
			Description("Enable dead letter queue for failed events").
			Default(false)).
		Field(service.NewObjectField("sqs",
			service.NewStringField("queue_url").Description("SQS queue URL").Default(""),
			service.NewIntField("max_messages").Description("Max messages per SQS receive").Default(10),
			service.NewIntField("wait_time_seconds").Description("SQS long polling wait time").Default(20),
			service.NewIntField("visibility_timeout").Description("SQS visibility timeout").Default(30),
		).Description("SQS mode configuration").Optional()).
		Field(service.NewObjectField("pipes",
			service.NewStringField("name").Description("EventBridge Pipe name").Default(""),
			service.NewStringField("source_arn").Description("Pipe source ARN").Default(""),
			service.NewStringField("target_arn").Description("Pipe target ARN").Default(""),
			service.NewIntField("batch_size").Description("Pipe batch size").Default(10),
		).Description("Pipes mode configuration").Optional()).
		Field(service.NewObjectField("aws",
			service.NewStringField("access_key_id").Description("AWS access key ID").Default(""),
			service.NewStringField("secret_access_key").Description("AWS secret access key").Default(""),
			service.NewStringField("session_token").Description("AWS session token").Default(""),
			service.NewStringField("profile").Description("AWS profile name").Default(""),
			service.NewStringField("role_arn").Description("IAM role ARN to assume").Default(""),
		).Description("AWS authentication configuration").Optional())
}

func newWWEventBridgeInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	// Extract configuration
	mode, err := conf.FieldString("mode")
	if err != nil {
		mode = "sqs"
	}

	region, err := conf.FieldString("region")
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	eventBusName, err := conf.FieldString("event_bus_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get event_bus_name: %w", err)
	}

	eventSource, err := conf.FieldString("event_source")
	if err != nil {
		return nil, fmt.Errorf("failed to get event_source: %w", err)
	}

	ruleName, err := conf.FieldString("rule_name")
	if err != nil {
		ruleName = ""
	}

	detailType, err := conf.FieldString("detail_type")
	if err != nil {
		detailType = ""
	}

	maxBatchSize, err := conf.FieldInt("max_batch_size")
	if err != nil {
		maxBatchSize = 10
	}

	enableDeadLetter, err := conf.FieldBool("enable_dead_letter")
	if err != nil {
		enableDeadLetter = false
	}

	// Build wombatwisdom EventBridge config using the actual import path
	inputConfig := eventbridge.TriggerInputConfig{
		Mode:             eventbridge.IntegrationMode(mode),
		Region:           region,
		EventBusName:     eventBusName,
		RuleName:         ruleName,
		EventSource:      eventSource,
		DetailType:       detailType,
		MaxBatchSize:     maxBatchSize,
		EnableDeadLetter: enableDeadLetter,
		EventFilters:     make(map[string]string),
	}

	// Handle SQS configuration if provided
	if conf.Contains("sqs") && mode == "sqs" {
		if queueURL, err := conf.FieldString("sqs", "queue_url"); err == nil && queueURL != "" {
			inputConfig.SQSQueueURL = queueURL
		}
		if maxMessages, err := conf.FieldInt("sqs", "max_messages"); err == nil {
			inputConfig.SQSMaxMessages = int32(maxMessages)
		}
		if waitTime, err := conf.FieldInt("sqs", "wait_time_seconds"); err == nil {
			inputConfig.SQSWaitTimeSeconds = int32(waitTime)
		}
		if visibilityTimeout, err := conf.FieldInt("sqs", "visibility_timeout"); err == nil {
			inputConfig.SQSVisibilityTimeout = int32(visibilityTimeout)
		}
	}

	// Handle Pipes configuration if provided
	if conf.Contains("pipes") && mode == "pipes" {
		if pipeName, err := conf.FieldString("pipes", "name"); err == nil && pipeName != "" {
			inputConfig.PipeName = pipeName
		}
		if sourceArn, err := conf.FieldString("pipes", "source_arn"); err == nil && sourceArn != "" {
			inputConfig.PipeSourceARN = sourceArn
		}
		if targetArn, err := conf.FieldString("pipes", "target_arn"); err == nil && targetArn != "" {
			inputConfig.PipeTargetARN = targetArn
		}
		if batchSize, err := conf.FieldInt("pipes", "batch_size"); err == nil {
			inputConfig.PipeBatchSize = int32(batchSize)
		}
	}

	// Handle event filters
	if filtersRaw, err := conf.FieldAny("event_filters"); err == nil {
		if filtersMap, ok := filtersRaw.(map[string]interface{}); ok {
			for key, value := range filtersMap {
				if str, ok := value.(string); ok {
					inputConfig.EventFilters[key] = str
				}
			}
		}
	}

	return &wwEventBridgeInput{
		inputConfig:  inputConfig,
		logger:       mgr.Logger(),
		messageQueue: make(chan *service.Message, 100), // Buffered channel
	}, nil
}

// wwEventBridgeInput provides seamless integration between Benthos and wombatwisdom EventBridge input
type wwEventBridgeInput struct {
	inputConfig eventbridge.TriggerInputConfig
	logger      *service.Logger

	// wombatwisdom components
	wwInput *eventbridge.TriggerInput

	// message queue for bridging wombatwisdom messages to Benthos
	messageQueue chan *service.Message
}

func (w *wwEventBridgeInput) Connect(ctx context.Context) error {
	// Create component context adapter
	componentCtx := &eventBridgeComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Create wombatwisdom EventBridge trigger input
	wwInput, err := eventbridge.NewTriggerInput(componentCtx, w.inputConfig)
	if err != nil {
		return fmt.Errorf("failed to create wombatwisdom EventBridge input: %w", err)
	}
	w.wwInput = wwInput

	// Initialize the trigger input
	err = w.wwInput.Init(componentCtx)
	if err != nil {
		return fmt.Errorf("failed to initialize wombatwisdom EventBridge input: %w", err)
	}

	w.logger.Info("wombatwisdom EventBridge input connected successfully")
	return nil
}

func (w *wwEventBridgeInput) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	// Create component context adapter for reading
	componentCtx := &eventBridgeComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Read triggers from the wombatwisdom input
	triggerBatch, processedCallback, err := w.wwInput.ReadTriggers(componentCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read triggers: %w", err)
	}

	// Convert the first trigger to a Benthos message
	// For now, we'll process one trigger at a time
	triggers := triggerBatch.Triggers()
	if len(triggers) == 0 {
		// Return a small delay to prevent busy loop
		time.Sleep(100 * time.Millisecond)
		return nil, nil, nil // No message available
	}

	trigger := triggers[0]
	if trigger == nil {
		return nil, nil, nil
	}

	// Convert trigger to Benthos message
	msg := w.convertTriggerToBenthosMessage(trigger)

	ackFunc := func(ctx context.Context, err error) error {
		// Call the processed callback
		if processedCallback != nil {
			return processedCallback(ctx, err)
		}
		return nil
	}

	return msg, ackFunc, nil
}

func (w *wwEventBridgeInput) Close(ctx context.Context) error {
	if w.wwInput != nil {
		componentCtx := &eventBridgeComponentContextAdapter{
			ctx:    ctx,
			logger: w.logger,
		}
		err := w.wwInput.Close(componentCtx)
		if err != nil {
			w.logger.Errorf("Error closing wombatwisdom EventBridge input: %v", err)
		}
		w.logger.Info("wombatwisdom EventBridge input closed")
	}

	// Close the message queue
	if w.messageQueue != nil {
		close(w.messageQueue)
	}

	return nil
}

func (w *wwEventBridgeInput) convertTriggerToBenthosMessage(trigger spec.TriggerEvent) *service.Message {
	// Create a Benthos message with trigger information
	msg := service.NewMessage([]byte(trigger.Reference()))
	msg.MetaSet("ww_component", "eventbridge")
	msg.MetaSet("ww_source", "wombatwisdom")
	msg.MetaSet("trigger_type", "eventbridge")
	msg.MetaSet("trigger_source", trigger.Source())
	msg.MetaSet("trigger_reference", trigger.Reference())
	msg.MetaSet("trigger_timestamp", fmt.Sprintf("%d", trigger.Timestamp()))

	// Copy trigger metadata
	for key, value := range trigger.Metadata() {
		msg.MetaSet(fmt.Sprintf("trigger_%s", key), fmt.Sprintf("%v", value))
	}

	return msg
}

// eventBridgeComponentContextAdapter provides a ComponentContext implementation for EventBridge components
type eventBridgeComponentContextAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

// Logger interface methods
func (c *eventBridgeComponentContextAdapter) Debugf(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

func (c *eventBridgeComponentContextAdapter) Infof(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

func (c *eventBridgeComponentContextAdapter) Warnf(format string, args ...interface{}) {
	c.logger.Warnf(format, args...)
}

func (c *eventBridgeComponentContextAdapter) Errorf(format string, args ...interface{}) {
	c.logger.Errorf(format, args...)
}

// ComponentContext interface methods
func (c *eventBridgeComponentContextAdapter) Context() context.Context {
	return c.ctx
}

func (c *eventBridgeComponentContextAdapter) GetString(key string) string {
	// Simple implementation - could be enhanced to read from actual environment
	return ""
}

func (c *eventBridgeComponentContextAdapter) GetInt(key string) int {
	return 0
}

func (c *eventBridgeComponentContextAdapter) GetBool(key string) bool {
	return false
}

// ComponentContext interface methods
func (c *eventBridgeComponentContextAdapter) BuildMetadataFilter(patterns []string, invert bool) (spec.MetadataFilter, error) {
	// Simple placeholder implementation
	return &simpleMetadataFilter{}, nil
}

func (c *eventBridgeComponentContextAdapter) Resources() spec.ResourceManager {
	// Simple placeholder implementation
	return nil
}

func (c *eventBridgeComponentContextAdapter) Input(name string) (spec.Input, error) {
	// Simple placeholder implementation
	return nil, fmt.Errorf("input not found: %s", name)
}

func (c *eventBridgeComponentContextAdapter) Output(name string) (spec.Output, error) {
	// Simple placeholder implementation
	return nil, fmt.Errorf("output not found: %s", name)
}

func (c *eventBridgeComponentContextAdapter) System(name string) (spec.System, error) {
	// Simple placeholder implementation
	return nil, fmt.Errorf("system not found: %s", name)
}

// ExpressionFactory methods
func (c *eventBridgeComponentContextAdapter) ParseExpression(expr string) (spec.Expression, error) {
	// Simple placeholder implementation
	return nil, fmt.Errorf("expression parsing not implemented")
}

// MessageFactory methods
func (c *eventBridgeComponentContextAdapter) NewBatch() spec.Batch {
	// Simple placeholder implementation
	return nil
}

func (c *eventBridgeComponentContextAdapter) NewMessage() spec.Message {
	// Simple placeholder implementation
	return spec.NewBytesMessage([]byte{})
}

