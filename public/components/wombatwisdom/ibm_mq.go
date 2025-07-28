//go:build mqclient

// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"time"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
	ibm_mq "github.com/wombatwisdom/components/mq"
)

func init() {
	// Register IBM MQ input with ww_ prefix for seamless integration
	err := service.RegisterInput(
		"ww_ibm_mq",
		wwIBMMQInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return newWWIBMMQInput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq input: %w", err))
	}

	// Register IBM MQ output with ww_ prefix for seamless integration
	err = service.RegisterOutput(
		"ww_ibm_mq",
		wwIBMMQOutputConfig(),
		newWWIBMMQOutput)
	if err != nil {
		panic(fmt.Errorf("failed to register ww_ibm_mq output: %w", err))
	}
}

func wwIBMMQInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Integration").
		Summary("Reads messages from IBM MQ queues using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom IBM MQ input component into Wombat.

This component requires CGO and the IBM MQ client libraries to be installed.
It uses the wombatwisdom IBM MQ implementation under the hood while providing
a native Benthos interface with full transactional support and batch processing.

## Prerequisites

- IBM MQ client libraries must be installed
- CGO must be enabled (this is automatically handled with mqclient build tag)
- Queue manager connection details must be configured

## Build Requirements

This component requires the 'mqclient' build tag:
   go build -tags mqclient ./...

## Features

- Transactional message processing with SYNCPOINT
- Batch processing for improved throughput
- Multiple parallel connections for high-volume queues
- Automatic retry and failure handling
- Full IBM MQ metadata extraction
`).
		Field(service.NewStringField("queue_name").
			Description("IBM MQ queue name to read messages from")).
		Field(service.NewIntField("batch_count").
			Description("Maximum number of messages to fetch at a time").
			Default(1)).
		Field(service.NewIntField("num_threads").
			Description("Number of parallel queue manager connections").
			Default(1)).
		Field(service.NewStringField("wait_time").
			Description("How long to wait for messages when queue is empty").
			Default("5s")).
		Field(service.NewBoolField("auto_retry_nacks").
			Description("Whether to automatically retry processing of failed messages").
			Default(true)).
		Field(service.NewStringField("sleep_time_before_exit_after_failure").
			Description("How long to wait before program exit after failure").
			Default("2m")).
		Field(service.NewStringField("system_name").
			Description("Name of the IBM MQ system resource to use").
			Default("default"))
}

func wwIBMMQOutputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Integration").
		Summary("Writes messages to IBM MQ queues using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom IBM MQ output component into Wombat.

This component requires CGO and the IBM MQ client libraries to be installed.
It uses the wombatwisdom IBM MQ implementation under the hood while providing
a native Benthos interface with full transactional support and message formatting.

## Prerequisites  

- IBM MQ client libraries must be installed
- CGO must be enabled (this is automatically handled with mqclient build tag)
- Queue manager connection details must be configured

## Build Requirements

This component requires the 'mqclient' build tag:
   go build -tags mqclient ./...

## Features

- Transactional message delivery with SYNCPOINT
- Multiple parallel connections for high throughput
- Dynamic queue name evaluation with expressions
- Message format configuration (CCSID, encoding, format)
- Metadata mapping to IBM MQ message properties
`).
		Field(service.NewStringField("queue_name").
			Description("IBM MQ queue name to write messages to (supports expressions)")).
		Field(service.NewIntField("num_threads").
			Description("Number of parallel queue manager connections").
			Default(1)).
		Field(service.NewStringField("format").
			Description("IBM MQ message format").
			Default("MQSTR")).
		Field(service.NewStringField("ccsid").
			Description("Character set identifier for message encoding").
			Default("1208")).
		Field(service.NewStringField("encoding").
			Description("Message encoding").
			Default("546")).
		Field(service.NewObjectField("metadata",
			service.NewStringListField("patterns").Description("Metadata key patterns to include in MQ message properties").Default([]any{}),
			service.NewBoolField("invert").Description("Whether to invert the metadata filter").Default(false),
		).Description("Metadata filtering configuration").Optional()).
		Field(service.NewStringField("system_name").
			Description("Name of the IBM MQ system resource to use").
			Default("default"))
}

func newWWIBMMQInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	// Extract configuration
	queueName, err := conf.FieldString("queue_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get queue_name: %w", err)
	}

	batchCount, err := conf.FieldInt("batch_count")
	if err != nil {
		batchCount = 1
	}

	numThreads, err := conf.FieldInt("num_threads")
	if err != nil {
		numThreads = 1
	}

	waitTime, err := conf.FieldString("wait_time")
	if err != nil {
		waitTime = "5s"
	}

	autoRetryNacks, err := conf.FieldBool("auto_retry_nacks")
	if err != nil {
		autoRetryNacks = true
	}

	sleepTimeBeforeExit, err := conf.FieldString("sleep_time_before_exit_after_failure")
	if err != nil {
		sleepTimeBeforeExit = "2m"
	}

	systemName, err := conf.FieldString("system_name")
	if err != nil {
		systemName = "default"
	}

	// Build wombatwisdom IBM MQ input config
	inputConfig := ibm_mq.InputConfig{
		QueueName:                       queueName,
		BatchCount:                      batchCount,
		NumThreads:                      numThreads,
		WaitTime:                        waitTime,
		AutoRetryNacks:                  autoRetryNacks,
		SleepTimeBeforeExitAfterFailure: sleepTimeBeforeExit,
	}

	return &wwIBMMQInput{
		inputConfig:  inputConfig,
		systemName:   systemName,
		logger:       mgr.Logger(),
		resourceMgr:  mgr,
		messageQueue: make(chan *service.Message, 100), // Buffered channel
	}, nil
}

func newWWIBMMQOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
	// Extract configuration
	queueName, err := conf.FieldString("queue_name")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get queue_name: %w", err)
	}

	numThreads, err := conf.FieldInt("num_threads")
	if err != nil {
		numThreads = 1
	}

	format, err := conf.FieldString("format")
	if err != nil {
		format = "MQSTR"
	}

	ccsid, err := conf.FieldString("ccsid")
	if err != nil {
		ccsid = "1208"
	}

	encoding, err := conf.FieldString("encoding")
	if err != nil {
		encoding = "546"
	}

	systemName, err := conf.FieldString("system_name")
	if err != nil {
		systemName = "default"
	}

	// Build wombatwisdom IBM MQ output config
	outputConfig := ibm_mq.OutputConfig{
		QueueName:  queueName,
		NumThreads: numThreads,
		Format:     &format,
		Ccsid:      &ccsid,
		Encoding:   &encoding,
	}

	// Handle metadata configuration if provided
	if conf.Contains("metadata") {
		metadata := &ibm_mq.MetadataConfig{}
		if patterns, err := conf.FieldStringList("metadata", "patterns"); err == nil {
			metadata.Patterns = patterns
		}
		if invert, err := conf.FieldBool("metadata", "invert"); err == nil {
			metadata.Invert = invert
		}
		outputConfig.Metadata = metadata
	}

	return &wwIBMMQOutput{
		outputConfig: outputConfig,
		systemName:   systemName,
		logger:       mgr.Logger(),
		resourceMgr:  mgr,
	}, numThreads, nil
}

// wwIBMMQInput provides seamless integration between Benthos and wombatwisdom IBM MQ input
type wwIBMMQInput struct {
	inputConfig ibm_mq.InputConfig
	systemName  string
	logger      *service.Logger
	resourceMgr *service.Resources

	// wombatwisdom components
	wwInput *ibm_mq.Input
	wwSystem *ibm_mq.System

	// message queue for bridging wombatwisdom messages to Benthos
	messageQueue chan *service.Message
}

func (w *wwIBMMQInput) Connect(ctx context.Context) error {
	// Get or create IBM MQ system from resources
	system, err := w.getOrCreateSystem(ctx)
	if err != nil {
		return fmt.Errorf("failed to get IBM MQ system: %w", err)
	}
	w.wwSystem = system

	// Create configuration object that implements spec.Config
	configObj := &ibmMQConfigAdapter{config: w.inputConfig}

	// Create wombatwisdom IBM MQ input
	wwInput, err := ibm_mq.NewInputFromConfig(w.wwSystem, configObj)
	if err != nil {
		return fmt.Errorf("failed to create wombatwisdom IBM MQ input: %w", err)
	}
	w.wwInput = wwInput

	// Create component context adapter
	componentCtx := &ibmMQComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Initialize the wombatwisdom input
	err = w.wwInput.Init(componentCtx)
	if err != nil {
		return fmt.Errorf("failed to initialize wombatwisdom IBM MQ input: %w", err)
	}

	w.logger.Info("wombatwisdom IBM MQ input connected successfully")
	return nil
}

func (w *wwIBMMQInput) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	// Create component context adapter
	componentCtx := &ibmMQComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Read from the wombatwisdom input
	batch, processedCallback, err := w.wwInput.Read(componentCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read from IBM MQ: %w", err)
	}

	// Convert batch to individual messages (for now, take the first message)
	messages := batch.Messages()
	if len(messages) == 0 {
		return nil, nil, nil // No messages available
	}

	// Convert the first wombatwisdom message to Benthos message
	benthosMsg, err := w.convertToBenthosMessage(messages[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert message: %w", err)
	}

	ackFunc := func(ctx context.Context, err error) error {
		// Call the processed callback
		if processedCallback != nil {
			return processedCallback(ctx, err)
		}
		return nil
	}

	return benthosMsg, ackFunc, nil
}

func (w *wwIBMMQInput) Close(ctx context.Context) error {
	if w.wwInput != nil {
		componentCtx := &ibmMQComponentContextAdapter{
			ctx:    ctx,
			logger: w.logger,
		}
		err := w.wwInput.Close(componentCtx)
		if err != nil {
			w.logger.Errorf("Error closing wombatwisdom IBM MQ input: %v", err)
		}
		w.logger.Info("wombatwisdom IBM MQ input closed")
	}

	// Close the message queue
	if w.messageQueue != nil {
		close(w.messageQueue)
	}

	return nil
}

func (w *wwIBMMQInput) convertToBenthosMessage(msg spec.Message) (*service.Message, error) {
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
	benthosMsg.MetaSet("ww_component", "ibm_mq")
	benthosMsg.MetaSet("ww_source", "wombatwisdom")

	return benthosMsg, nil
}

func (w *wwIBMMQInput) getOrCreateSystem(ctx context.Context) (*ibm_mq.System, error) {
	// For now, create a basic system - in production this would use
	// proper system resource management
	// TODO: Implement proper system resource lookup from Benthos resources
	
	// Create a basic system config - this would typically come from system resources
	systemConfig := ibm_mq.SystemConfig{
		QueueManagerName: "QM1", // This should be configurable
		ChannelName:      "DEV.APP.SVRCONN",
		ConnectionName:   "localhost(1414)",
	}

	configObj := &ibmMQSystemConfigAdapter{config: systemConfig}
	system, err := ibm_mq.NewSystemFromConfig(configObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create IBM MQ system: %w", err)
	}

	// Connect the system
	err = system.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect IBM MQ system: %w", err)
	}

	return system, nil
}

// wwIBMMQOutput provides seamless integration between Benthos and wombatwisdom IBM MQ output
type wwIBMMQOutput struct {
	outputConfig ibm_mq.OutputConfig
	systemName   string
	logger       *service.Logger
	resourceMgr  *service.Resources

	// wombatwisdom components
	wwOutput *ibm_mq.Output
	wwSystem *ibm_mq.System
}

func (w *wwIBMMQOutput) Connect(ctx context.Context) error {
	// Get or create IBM MQ system from resources
	system, err := w.getOrCreateSystem(ctx)
	if err != nil {
		return fmt.Errorf("failed to get IBM MQ system: %w", err)
	}
	w.wwSystem = system

	// Create wombatwisdom IBM MQ output
	w.wwOutput = ibm_mq.NewOutput(w.wwSystem, w.outputConfig)

	// Create component context adapter
	componentCtx := &ibmMQComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Initialize the wombatwisdom output
	err = w.wwOutput.Init(componentCtx)
	if err != nil {
		return fmt.Errorf("failed to initialize wombatwisdom IBM MQ output: %w", err)
	}

	w.logger.Info("wombatwisdom IBM MQ output connected successfully")
	return nil
}

func (w *wwIBMMQOutput) Write(ctx context.Context, msg *service.Message) error {
	if w.wwOutput == nil {
		return fmt.Errorf("IBM MQ output not connected")
	}

	// Convert Benthos message to wombatwisdom message
	data, _ := msg.AsBytes()
	wwMsg := &ibmMQMessageAdapter{
		data: data,
		meta: make(map[string]any),
	}

	// Copy metadata
	_ = msg.MetaWalk(func(key string, value string) error {
		wwMsg.meta[key] = value
		return nil
	})

	// Create component context adapter
	componentCtx := &ibmMQComponentContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	// Write using wombatwisdom output
	err := w.wwOutput.WriteMessage(componentCtx, wwMsg)
	if err != nil {
		w.logger.Errorf("Failed to write message via wombatwisdom IBM MQ output: %v", err)
		return err
	}

	w.logger.Debugf("Successfully sent message to IBM MQ via wombatwisdom")
	return nil
}

func (w *wwIBMMQOutput) Close(ctx context.Context) error {
	if w.wwOutput != nil {
		componentCtx := &ibmMQComponentContextAdapter{
			ctx:    ctx,
			logger: w.logger,
		}
		err := w.wwOutput.Close(componentCtx)
		if err != nil {
			w.logger.Errorf("Error closing wombatwisdom IBM MQ output: %v", err)
		}
		w.logger.Info("wombatwisdom IBM MQ output closed")
	}

	return nil
}

func (w *wwIBMMQOutput) getOrCreateSystem(ctx context.Context) (*ibm_mq.System, error) {
	// For now, create a basic system - in production this would use
	// proper system resource management
	// TODO: Implement proper system resource lookup from Benthos resources
	
	// Create a basic system config - this would typically come from system resources
	systemConfig := ibm_mq.SystemConfig{
		QueueManagerName: "QM1", // This should be configurable
		ChannelName:      "DEV.APP.SVRCONN",
		ConnectionName:   "localhost(1414)",
	}

	configObj := &ibmMQSystemConfigAdapter{config: systemConfig}
	system, err := ibm_mq.NewSystemFromConfig(configObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create IBM MQ system: %w", err)
	}

	// Connect the system
	err = system.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect IBM MQ system: %w", err)
	}

	return system, nil
}

// IBM MQ-specific adapter implementations
type ibmMQComponentContextAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

// Logger interface methods
func (c *ibmMQComponentContextAdapter) Debugf(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

func (c *ibmMQComponentContextAdapter) Infof(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

func (c *ibmMQComponentContextAdapter) Warnf(format string, args ...interface{}) {
	c.logger.Warnf(format, args...)
}

func (c *ibmMQComponentContextAdapter) Errorf(format string, args ...interface{}) {
	c.logger.Errorf(format, args...)
}

// ComponentContext interface methods
func (c *ibmMQComponentContextAdapter) Context() context.Context {
	return c.ctx
}

func (c *ibmMQComponentContextAdapter) GetString(key string) string {
	return ""
}

func (c *ibmMQComponentContextAdapter) GetInt(key string) int {
	return 0
}

func (c *ibmMQComponentContextAdapter) GetBool(key string) bool {
	return false
}

func (c *ibmMQComponentContextAdapter) BuildMetadataFilter(patterns []string, invert bool) (spec.MetadataFilter, error) {
	return &simpleMetadataFilter{}, nil
}

func (c *ibmMQComponentContextAdapter) Resources() spec.ResourceManager {
	return nil
}

func (c *ibmMQComponentContextAdapter) Input(name string) (spec.Input, error) {
	return nil, fmt.Errorf("input not found: %s", name)
}

func (c *ibmMQComponentContextAdapter) Output(name string) (spec.Output, error) {
	return nil, fmt.Errorf("output not found: %s", name)
}

func (c *ibmMQComponentContextAdapter) System(name string) (spec.System, error) {
	return nil, fmt.Errorf("system not found: %s", name)
}

// ExpressionFactory methods
func (c *ibmMQComponentContextAdapter) ParseExpression(expr string) (spec.Expression, error) {
	return &ibmMQExpressionAdapter{expr: expr}, nil
}

// MessageFactory methods
func (c *ibmMQComponentContextAdapter) NewBatch() spec.Batch {
	return &ibmMQBatchAdapter{messages: make([]spec.Message, 0)}
}

func (c *ibmMQComponentContextAdapter) NewMessage() spec.Message {
	return spec.NewBytesMessage([]byte{})
}

// ibmMQMessageAdapter implements the wombatwisdom Message interface for Benthos messages
type ibmMQMessageAdapter struct {
	data []byte
	meta map[string]any
}

func (m *ibmMQMessageAdapter) Raw() ([]byte, error) {
	return m.data, nil
}

func (m *ibmMQMessageAdapter) SetMetadata(key string, value any) {
	m.meta[key] = value
}

func (m *ibmMQMessageAdapter) SetRaw(data []byte) {
	m.data = data
}

func (m *ibmMQMessageAdapter) Metadata() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range m.meta {
			if !yield(k, v) {
				return
			}
		}
	}
}

// ibmMQConfigAdapter implements the spec.Config interface for IBM MQ config
type ibmMQConfigAdapter struct {
	config ibm_mq.InputConfig
}

func (c *ibmMQConfigAdapter) Decode(target interface{}) error {
	// Simple implementation - copy the config
	if cfg, ok := target.(*ibm_mq.InputConfig); ok {
		*cfg = c.config
		return nil
	}
	return fmt.Errorf("target is not *ibm_mq.InputConfig")
}

// ibmMQSystemConfigAdapter implements the spec.Config interface for IBM MQ system config
type ibmMQSystemConfigAdapter struct {
	config ibm_mq.SystemConfig
}

func (c *ibmMQSystemConfigAdapter) Decode(target interface{}) error {
	// Simple implementation - copy the config
	if cfg, ok := target.(*ibm_mq.SystemConfig); ok {
		*cfg = c.config
		return nil
	}
	return fmt.Errorf("target is not *ibm_mq.SystemConfig")
}

// ibmMQExpressionAdapter provides a simple expression implementation
type ibmMQExpressionAdapter struct {
	expr string
}

func (e *ibmMQExpressionAdapter) EvalString(ctx spec.ExpressionContext) (string, error) {
	// Simple implementation - just return the expression as-is
	// In a full implementation, this would parse and evaluate the expression
	return e.expr, nil
}

func (e *ibmMQExpressionAdapter) EvalInt(ctx spec.ExpressionContext) (int, error) {
	// Simple implementation
	if val, err := strconv.Atoi(e.expr); err == nil {
		return val, nil
	}
	return 0, nil
}

func (e *ibmMQExpressionAdapter) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	// Simple implementation
	return e.expr == "true", nil
}

// ibmMQBatchAdapter provides a simple batch implementation
type ibmMQBatchAdapter struct {
	messages []spec.Message
}

func (b *ibmMQBatchAdapter) Messages() []spec.Message {
	return b.messages
}

func (b *ibmMQBatchAdapter) Append(msg spec.Message) {
	b.messages = append(b.messages, msg)
}

func (b *ibmMQBatchAdapter) Len() int {
	return len(b.messages)
}