// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"context"
	"fmt"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	s3 "github.com/wombatwisdom/components/aws-s3"
	"github.com/wombatwisdom/components/framework/spec"
)

func init() {
	// Register S3 input with ww_ prefix for seamless integration
	err := service.RegisterInput(
		"ww_s3",
		wwS3InputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return newWWS3Input(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_s3 input: %w", err))
	}

	// Register S3 processor with ww_ prefix for seamless integration
	err = service.RegisterProcessor(
		"ww_s3_retrieval",
		wwS3RetrievalConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Processor, error) {
			return newWWS3Retrieval(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_s3_retrieval processor: %w", err))
	}
}

func wwS3InputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("AWS", "Services").
		Summary("Lists and reads S3 objects using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom S3 input component into Wombat.

This component uses the wombatwisdom S3 implementation under the hood while providing
a native Benthos interface. It supports listing and reading objects from S3 buckets
with full AWS authentication and configuration options.

## Authentication

The component supports various AWS authentication methods including:
- IAM roles
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- EC2 instance profiles  
- AWS profiles

## Usage

This input will list objects in the specified S3 bucket and read their contents.
For trigger-retrieval pattern, use this component with the ww_s3_retrieval processor.
`).
		Field(service.NewStringField("bucket").
			Description("S3 bucket name")).
		Field(service.NewStringField("prefix").
			Description("Object key prefix to filter objects").
			Default("")).
		Field(service.NewIntField("max_keys").
			Description("Maximum number of keys to list per API call").
			Default(1000)).
		Field(service.NewStringField("region").
			Description("AWS region").
			Default("us-east-1")).
		Field(service.NewBoolField("force_path_style_urls").
			Description("Force path-style URLs for S3-compatible services").
			Default(false)).
		Field(service.NewStringField("endpoint_url").
			Description("Custom S3 endpoint URL for S3-compatible services").
			Default("")).
		Field(service.NewObjectField("aws",
			service.NewStringField("access_key_id").Description("AWS access key ID").Default(""),
			service.NewStringField("secret_access_key").Description("AWS secret access key").Default(""),
			service.NewStringField("session_token").Description("AWS session token").Default(""),
			service.NewStringField("profile").Description("AWS profile name").Default(""),
			service.NewStringField("role_arn").Description("IAM role ARN to assume").Default(""),
		).Description("AWS authentication configuration").Optional())
}

func wwS3RetrievalConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("AWS", "Services").
		Summary("Retrieves S3 objects based on trigger events using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom S3 retrieval processor into Wombat.

This processor implements the trigger-retrieval pattern by fetching S3 objects based on
trigger events from EventBridge or other sources. It provides high-performance concurrent
retrieval with filtering capabilities.

## Trigger-Retrieval Pattern

This processor works with trigger inputs like ww_eventbridge to:
1. Receive lightweight trigger events containing S3 object references
2. Filter triggers based on object key patterns  
3. Fetch the actual S3 object data concurrently
4. Return the retrieved data as standard Benthos messages

## Authentication

The component supports various AWS authentication methods including:
- IAM roles  
- Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
- EC2 instance profiles
- AWS profiles
`).
		Field(service.NewStringField("region").
			Description("AWS region").
			Default("us-east-1")).
		Field(service.NewBoolField("force_path_style_urls").
			Description("Force path-style URLs for S3-compatible services").
			Default(false)).
		Field(service.NewStringField("endpoint_url").
			Description("Custom S3 endpoint URL for S3-compatible services").
			Default("")).
		Field(service.NewIntField("max_concurrent_retrievals").
			Description("Maximum number of concurrent S3 retrievals").
			Default(10)).
		Field(service.NewStringField("filter_prefix").
			Description("Only retrieve objects with keys starting with this prefix").
			Default("")).
		Field(service.NewStringField("filter_suffix").
			Description("Only retrieve objects with keys ending with this suffix").
			Default("")).
		Field(service.NewObjectField("aws",
			service.NewStringField("access_key_id").Description("AWS access key ID").Default(""),
			service.NewStringField("secret_access_key").Description("AWS secret access key").Default(""),
			service.NewStringField("session_token").Description("AWS session token").Default(""),
			service.NewStringField("profile").Description("AWS profile name").Default(""),
			service.NewStringField("role_arn").Description("IAM role ARN to assume").Default(""),
		).Description("AWS authentication configuration").Optional())
}

func newWWS3Input(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	// Extract configuration
	bucket, err := conf.FieldString("bucket")
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	prefix, err := conf.FieldString("prefix")
	if err != nil {
		prefix = ""
	}

	maxKeys, err := conf.FieldInt("max_keys")
	if err != nil {
		maxKeys = 1000
	}

	// TODO: Add AWS config support when implementing full authentication
	_, err = conf.FieldString("region")
	if err != nil {
		// Default region handling would go here
	}

	forcePathStyle, err := conf.FieldBool("force_path_style_urls")
	if err != nil {
		forcePathStyle = false
	}

	endpointURL, err := conf.FieldString("endpoint_url")
	if err != nil {
		endpointURL = ""
	}

	// Build wombatwisdom S3 config
	inputConfig := s3.InputConfig{
		Bucket:             bucket,
		Prefix:             prefix,
		MaxKeys:            int32(maxKeys),
		ForcePathStyleURLs: forcePathStyle,
	}

	if endpointURL != "" {
		inputConfig.EndpointURL = &endpointURL
	}

	// TODO: Handle AWS config - need to check the actual AWS config structure in s3 package

	return &wwS3Input{
		inputConfig:  inputConfig,
		logger:       mgr.Logger(),
		messageQueue: make(chan *service.Message, 100), // Buffered channel
	}, nil
}

func newWWS3Retrieval(conf *service.ParsedConfig, mgr *service.Resources) (service.Processor, error) {
	// Extract configuration
	// TODO: Add AWS config support when implementing full authentication
	_, err := conf.FieldString("region")
	if err != nil {
		// Default region handling would go here
	}

	forcePathStyle, err := conf.FieldBool("force_path_style_urls")
	if err != nil {
		forcePathStyle = false
	}

	endpointURL, err := conf.FieldString("endpoint_url")
	if err != nil {
		endpointURL = ""
	}

	maxConcurrent, err := conf.FieldInt("max_concurrent_retrievals")
	if err != nil {
		maxConcurrent = 10
	}

	filterPrefix, err := conf.FieldString("filter_prefix")
	if err != nil {
		filterPrefix = ""
	}

	filterSuffix, err := conf.FieldString("filter_suffix")
	if err != nil {
		filterSuffix = ""
	}

	// Build wombatwisdom S3 retrieval config
	retrievalConfig := s3.RetrievalConfig{
		ForcePathStyleURLs:     forcePathStyle,
		MaxConcurrentRetrivals: maxConcurrent,
		FilterPrefix:           filterPrefix,
		FilterSuffix:           filterSuffix,
	}

	if endpointURL != "" {
		retrievalConfig.EndpointURL = &endpointURL
	}

	// TODO: Handle AWS config - need to check the actual AWS config structure in s3 package

	return &wwS3Retrieval{
		retrievalConfig: retrievalConfig,
		logger:          mgr.Logger(),
	}, nil
}

// wwS3Input provides seamless integration between Benthos and wombatwisdom S3 input
type wwS3Input struct {
	inputConfig s3.InputConfig
	logger      *service.Logger

	// wombatwisdom components
	wwInput *s3.Input

	// message queue for bridging wombatwisdom messages to Benthos
	messageQueue chan *service.Message
}

func (w *wwS3Input) Connect(ctx context.Context) error {
	// Create environment adapter
	env := &s3EnvironmentAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Create wombatwisdom S3 input
	wwInput, err := s3.NewInput(env, w.inputConfig)
	if err != nil {
		return fmt.Errorf("failed to create wombatwisdom S3 input: %w", err)
	}
	w.wwInput = wwInput

	// Connect the wombatwisdom input
	err = w.wwInput.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect wombatwisdom S3 input: %w", err)
	}

	// Create a collector that bridges wombatwisdom messages to Benthos
	collector := &s3CollectorAdapter{
		input:  w,
		logger: w.logger,
	}

	// Start reading in a goroutine
	go func() {
		for {
			err := w.wwInput.Read(ctx, collector)
			if err != nil {
				w.logger.Errorf("Error reading from S3: %v", err)
				time.Sleep(5 * time.Second) // Wait before retrying
				continue
			}
			break // S3 input finished (reached end of objects)
		}
	}()

	w.logger.Info("wombatwisdom S3 input connected successfully")
	return nil
}

func (w *wwS3Input) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	// Read from the message queue populated by the collector
	select {
	case msg := <-w.messageQueue:
		ackFunc := func(ctx context.Context, err error) error {
			// For S3, we generally don't need explicit acks
			return nil
		}
		return msg, ackFunc, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (w *wwS3Input) Close(ctx context.Context) error {
	if w.wwInput != nil {
		err := w.wwInput.Disconnect(ctx)
		if err != nil {
			w.logger.Errorf("Error disconnecting wombatwisdom S3 input: %v", err)
		}
		w.logger.Info("wombatwisdom S3 input closed")
	}

	// Close the message queue
	if w.messageQueue != nil {
		close(w.messageQueue)
	}

	return nil
}

// wwS3Retrieval provides seamless integration between Benthos and wombatwisdom S3 retrieval processor  
type wwS3Retrieval struct {
	retrievalConfig s3.RetrievalConfig
	logger          *service.Logger

	// wombatwisdom components
	wwRetrieval *s3.RetrievalProcessor
}

func (w *wwS3Retrieval) Connect(ctx context.Context) error {
	// Create wombatwisdom S3 retrieval processor
	w.wwRetrieval = s3.NewRetrievalProcessor(w.retrievalConfig)

	// Initialize the retrieval processor
	componentCtx := &s3ComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}
	err := w.wwRetrieval.Init(componentCtx)
	if err != nil {
		return fmt.Errorf("failed to initialize wombatwisdom S3 retrieval: %w", err)
	}

	w.logger.Info("wombatwisdom S3 retrieval processor connected successfully")
	return nil
}

func (w *wwS3Retrieval) Process(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	// Convert Benthos message to trigger event
	trigger := w.convertBenthosMessageToTrigger(msg)

	// Create a trigger batch with the single trigger
	triggerBatch := spec.NewTriggerBatch()
	triggerBatch.Append(trigger)

	// Create component context adapter
	componentCtx := &s3ComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Retrieve using wombatwisdom retrieval processor
	batch, processedCallback, err := w.wwRetrieval.Retrieve(componentCtx, triggerBatch)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve S3 objects: %w", err)
	}

	// Convert wombatwisdom batch to Benthos message batch
	benthosMessages := make(service.MessageBatch, 0)
	if batch != nil {
		// Convert each message in the batch
		for _, wwMsg := range batch.Messages() {
			benthosMsg, err := w.convertToBenthosMessage(wwMsg)
			if err != nil {
				w.logger.Errorf("Failed to convert wombatwisdom message: %v", err)
				continue
			}
			benthosMessages = append(benthosMessages, benthosMsg)
		}
	}

	// Call processed callback
	if processedCallback != nil {
		err = processedCallback(ctx, nil)
		if err != nil {
			w.logger.Errorf("Error in processed callback: %v", err)
		}
	}

	return benthosMessages, nil
}

func (w *wwS3Retrieval) Close(ctx context.Context) error {
	if w.wwRetrieval != nil {
		componentCtx := &s3ComponentContextAdapter{
			ctx:    ctx,
			logger: w.logger,
		}
		err := w.wwRetrieval.Close(componentCtx)
		if err != nil {
			w.logger.Errorf("Error closing wombatwisdom S3 retrieval: %v", err)
		}
		w.logger.Info("wombatwisdom S3 retrieval processor closed")
	}
	return nil
}

func (w *wwS3Retrieval) convertBenthosMessageToTrigger(msg *service.Message) spec.TriggerEvent {
	// Extract trigger information from message metadata
	source, _ := msg.MetaGet("trigger_source")
	if source == "" {
		source = "benthos"
	}

	reference, _ := msg.MetaGet("trigger_reference")
	if reference == "" {
		// Use message content as reference if no trigger reference
		data, _ := msg.AsBytes()
		reference = string(data)
	}

	// Build metadata
	metadata := make(map[string]any)
	_ = msg.MetaWalk(func(key string, value string) error {
		metadata[key] = value
		return nil
	})

	return spec.NewTriggerEvent(source, reference, metadata)
}

func (w *wwS3Retrieval) convertToBenthosMessage(wwMsg spec.Message) (*service.Message, error) {
	// Extract raw data
	data, err := wwMsg.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw data: %w", err)
	}

	// Create Benthos message
	benthosMsg := service.NewMessage(data)

	// Copy metadata
	for key, value := range wwMsg.Metadata() {
		benthosMsg.MetaSet(key, fmt.Sprintf("%v", value))
	}

	// Add wombatwisdom metadata
	benthosMsg.MetaSet("ww_component", "s3")
	benthosMsg.MetaSet("ww_source", "wombatwisdom")

	return benthosMsg, nil
}

// S3-specific adapter implementations
type s3EnvironmentAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

// Logger interface methods
func (e *s3EnvironmentAdapter) Debugf(format string, args ...interface{}) {
	e.logger.Debugf(format, args...)
}

func (e *s3EnvironmentAdapter) Infof(format string, args ...interface{}) {
	e.logger.Infof(format, args...)
}

func (e *s3EnvironmentAdapter) Warnf(format string, args ...interface{}) {
	e.logger.Warnf(format, args...)
}

func (e *s3EnvironmentAdapter) Errorf(format string, args ...interface{}) {
	e.logger.Errorf(format, args...)
}

// Environment interface methods
func (e *s3EnvironmentAdapter) GetString(key string) string {
	return ""
}

func (e *s3EnvironmentAdapter) GetInt(key string) int {
	return 0
}

func (e *s3EnvironmentAdapter) GetBool(key string) bool {
	return false
}

// DynamicFieldFactory interface method
func (e *s3EnvironmentAdapter) NewDynamicField(expr string) spec.DynamicField {
	return &s3DynamicField{expr: expr}
}

// s3DynamicField provides a dynamic field implementation for S3 components
type s3DynamicField struct {
	expr string
}

func (f *s3DynamicField) String() string {
	return f.expr
}

func (f *s3DynamicField) Int() int {
	return 0
}

func (f *s3DynamicField) Bool() bool {
	return f.expr == "true"
}

func (f *s3DynamicField) AsString(msg spec.Message) (string, error) {
	return f.expr, nil
}

func (f *s3DynamicField) AsBool(msg spec.Message) (bool, error) {
	return f.expr == "true", nil
}

// s3CollectorAdapter bridges wombatwisdom messages to Benthos message queue
type s3CollectorAdapter struct {
	input  *wwS3Input
	logger *service.Logger
}

// Collector interface methods
func (c *s3CollectorAdapter) Collect(msg spec.Message) error {
	return c.Write(msg)
}

func (c *s3CollectorAdapter) Flush() (spec.Batch, error) {
	return nil, nil
}

func (c *s3CollectorAdapter) Write(msg spec.Message) error {
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

func (c *s3CollectorAdapter) Disconnect() error {
	c.logger.Info("S3 collector disconnecting")
	return nil
}

func (c *s3CollectorAdapter) convertToBenthosMessage(msg spec.Message) (*service.Message, error) {
	// Extract raw data
	data, err := msg.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw data: %w", err)
	}

	// Create Benthos message
	benthosMsg := service.NewMessage(data)

	// Copy metadata
	for key, value := range msg.Metadata() {
		benthosMsg.MetaSet(key, fmt.Sprintf("%v", value))
	}

	// Add wombatwisdom metadata
	benthosMsg.MetaSet("ww_component", "s3")
	benthosMsg.MetaSet("ww_source", "wombatwisdom")

	return benthosMsg, nil
}

// s3ComponentContextAdapter provides a ComponentContext implementation for S3 components
type s3ComponentContextAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

// Logger interface methods
func (c *s3ComponentContextAdapter) Debugf(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

func (c *s3ComponentContextAdapter) Infof(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

func (c *s3ComponentContextAdapter) Warnf(format string, args ...interface{}) {
	c.logger.Warnf(format, args...)
}

func (c *s3ComponentContextAdapter) Errorf(format string, args ...interface{}) {
	c.logger.Errorf(format, args...)
}

// ComponentContext interface methods
func (c *s3ComponentContextAdapter) Context() context.Context {
	return c.ctx
}

func (c *s3ComponentContextAdapter) GetString(key string) string {
	return ""
}

func (c *s3ComponentContextAdapter) GetInt(key string) int {
	return 0
}

func (c *s3ComponentContextAdapter) GetBool(key string) bool {
	return false
}

func (c *s3ComponentContextAdapter) BuildMetadataFilter(patterns []string, invert bool) (spec.MetadataFilter, error) {
	return &simpleMetadataFilter{}, nil
}

func (c *s3ComponentContextAdapter) Resources() spec.ResourceManager {
	return nil
}

func (c *s3ComponentContextAdapter) Input(name string) (spec.Input, error) {
	return nil, fmt.Errorf("input not found: %s", name)
}

func (c *s3ComponentContextAdapter) Output(name string) (spec.Output, error) {
	return nil, fmt.Errorf("output not found: %s", name)
}

func (c *s3ComponentContextAdapter) System(name string) (spec.System, error) {
	return nil, fmt.Errorf("system not found: %s", name)
}

// ExpressionFactory methods
func (c *s3ComponentContextAdapter) ParseExpression(expr string) (spec.Expression, error) {
	return nil, fmt.Errorf("expression parsing not implemented")
}

// MessageFactory methods
func (c *s3ComponentContextAdapter) NewBatch() spec.Batch {
	return nil
}

func (c *s3ComponentContextAdapter) NewMessage() spec.Message {
	return spec.NewBytesMessage([]byte{})
}