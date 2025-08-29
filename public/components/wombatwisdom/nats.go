// Package wombatwisdom provides seamless integration of wombatwisdom components into Wombat
package wombatwisdom

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/bundles/nats"
	"github.com/wombatwisdom/components/bundles/nats/core"
	"github.com/wombatwisdom/components/framework/spec"
)

func init() {
	// Register NATS input with ww_ prefix for seamless integration
	err := service.RegisterInput(
		"ww_nats",
		wwNatsInputConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
			return newWWNatsInput(conf, mgr)
		})
	if err != nil {
		panic(fmt.Errorf("failed to register ww_nats input: %w", err))
	}

	// Register NATS output with ww_ prefix for seamless integration
	err = service.RegisterOutput(
		"ww_nats",
		wwNatsOutputConfig(),
		newWWNatsOutput)
	if err != nil {
		panic(fmt.Errorf("failed to register ww_nats output: %w", err))
	}
}

func wwNatsInputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Reads messages from NATS subjects using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom NATS input component into Wombat.

This component uses the wombatwisdom NATS implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and monitoring capabilities.

## Connection Management

The component automatically handles connection lifecycle and provides robust error
handling and reconnection logic through the wombatwisdom framework.
`).
		Field(service.NewStringField("url").
			Description("URL of the NATS server to connect to. Multiple URLs can be specified by separating them with commas.").
			Default("nats://localhost:4222")).
		Field(service.NewStringField("subject").
			Description("The subject to subscribe to. May contain wildcards.")).
		Field(service.NewStringField("queue").
			Description("Optional queue group to join for load balancing.").
			Default("").
			Optional()).
		Field(service.NewIntField("batch_count").
			Description("Maximum number of messages to fetch at a time.").
			Default(1)).
		Field(service.NewStringField("name").
			Description("Optional connection name for monitoring.").
			Default("wombat").
			Optional()).
		Field(service.NewObjectField("auth",
			service.NewStringField("jwt").Description("User JWT token for authentication").Default(""),
			service.NewStringField("seed").Description("User seed for authentication").Default(""),
		).Description("Authentication configuration").Optional())
}

func wwNatsOutputConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Stable().
		Categories("Services").
		Summary("Publishes messages to NATS subjects using wombatwisdom components").
		Description(`
Seamless integration of wombatwisdom NATS output component into Wombat.

This component uses the wombatwisdom NATS implementation under the hood while providing
a native Benthos interface. All wombatwisdom features are available including advanced
connection management, authentication, and monitoring capabilities.
`).
		Field(service.NewStringField("url").
			Description("URL of the NATS server to connect to. Multiple URLs can be specified by separating them with commas.").
			Default("nats://localhost:4222")).
		Field(service.NewStringField("subject").
			Description("The subject to publish to. May contain interpolation functions.")).
		Field(service.NewStringField("name").
			Description("Optional connection name for monitoring.").
			Default("wombat").
			Optional()).
		Field(service.NewObjectField("auth",
			service.NewStringField("jwt").Description("User JWT token for authentication").Default(""),
			service.NewStringField("seed").Description("User seed for authentication").Default(""),
		).Description("Authentication configuration").Optional()).
		Field(service.NewObjectField("metadata",
			service.NewBoolField("invert").Description("Invert metadata field matching").Default(false),
			service.NewStringListField("patterns").Description("Regex patterns for metadata fields").Default([]string{}),
		).Description("Metadata filtering configuration").Optional())
}

func newWWNatsInput(conf *service.ParsedConfig, mgr *service.Resources) (service.Input, error) {
	// Extract configuration
	url, err := conf.FieldString("url")
	if err != nil {
		return nil, fmt.Errorf("failed to get url: %w", err)
	}

	subject, err := conf.FieldString("subject")
	if err != nil {
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}

	batchCount, err := conf.FieldInt("batch_count")
	if err != nil {
		return nil, fmt.Errorf("failed to get batch_count: %w", err)
	}

	name, err := conf.FieldString("name")
	if err != nil {
		name = "wombat"
	}

	// Build wombatwisdom system config
	systemConfig := core.SystemConfig{
		Url:  url,
		Name: name,
	}

	// Handle auth if provided
	if conf.Contains("auth") {
		jwt, _ := conf.FieldString("auth", "jwt")
		seed, _ := conf.FieldString("auth", "seed")
		if jwt != "" && seed != "" {
			systemConfig.Auth = &core.SystemConfigAuth{
				Jwt:  jwt,
				Seed: seed,
			}
		}
	}

	// Build wombatwisdom input config
	inputConfig := core.InputConfig{
		Subject:    subject,
		BatchCount: batchCount,
	}

	// Handle queue if provided
	if queue, err := conf.FieldString("queue"); err == nil && queue != "" {
		inputConfig.Queue = &queue
	}

	return &wwNatsInput{
		systemConfig: systemConfig,
		inputConfig:  inputConfig,
		logger:       mgr.Logger(),
	}, nil
}

func newWWNatsOutput(conf *service.ParsedConfig, mgr *service.Resources) (service.Output, int, error) {
	// Extract configuration
	url, err := conf.FieldString("url")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get url: %w", err)
	}

	subject, err := conf.FieldString("subject")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get subject: %w", err)
	}

	name, err := conf.FieldString("name")
	if err != nil {
		name = "wombat"
	}

	// Build wombatwisdom system config
	systemConfig := core.SystemConfig{
		Url:  url,
		Name: name,
	}

	// Handle auth if provided
	if conf.Contains("auth") {
		jwt, _ := conf.FieldString("auth", "jwt")
		seed, _ := conf.FieldString("auth", "seed")
		if jwt != "" && seed != "" {
			systemConfig.Auth = &core.SystemConfigAuth{
				Jwt:  jwt,
				Seed: seed,
			}
		}
	}

	// Build wombatwisdom output config
	outputConfig := core.OutputConfig{
		Subject: subject,
	}

	// Handle metadata config if provided
	if conf.Contains("metadata") {
		invert, _ := conf.FieldBool("metadata", "invert")
		patterns, _ := conf.FieldStringList("metadata", "patterns")
		outputConfig.Metadata = &core.OutputConfigMetadata{
			Invert:   invert,
			Patterns: patterns,
		}
	}

	return &wwNatsOutput{
		systemConfig: systemConfig,
		outputConfig: outputConfig,
		logger:       mgr.Logger(),
	}, 1, nil
}

// wwNatsInput provides seamless integration between Benthos and wombatwisdom NATS input
type wwNatsInput struct {
	systemConfig core.SystemConfig
	inputConfig  core.InputConfig
	logger       *service.Logger

	// wombatwisdom components
	wwSystem *nats.System
	wwInput  *nats.Input
}

func (w *wwNatsInput) Connect(ctx context.Context) error {
	w.logger.Info("Starting wombatwisdom NATS input connection...")

	// Create wombatwisdom config objects
	w.logger.Debugf("Creating NATS system config: url=%s, name=%s", w.systemConfig.Url, w.systemConfig.Name)
	systemConfigMap := make(map[string]interface{})
	systemConfigBytes, err := json.Marshal(w.systemConfig)
	if err != nil {
		w.logger.Errorf("Failed to marshal system config: %v", err)
		return fmt.Errorf("failed to marshal system config: %w", err)
	}
	if err := json.Unmarshal(systemConfigBytes, &systemConfigMap); err != nil {
		w.logger.Errorf("Failed to unmarshal system config: %v", err)
		return fmt.Errorf("failed to unmarshal system config: %w", err)
	}
	systemSpecConfig := spec.NewMapConfig(systemConfigMap)

	w.logger.Debugf("Creating NATS input config: subject=%s, batch_count=%d", w.inputConfig.Subject, w.inputConfig.BatchCount)
	inputConfigMap := make(map[string]interface{})
	inputConfigBytes, err := json.Marshal(w.inputConfig)
	if err != nil {
		w.logger.Errorf("Failed to marshal input config: %v", err)
		return fmt.Errorf("failed to marshal input config: %w", err)
	}
	if err := json.Unmarshal(inputConfigBytes, &inputConfigMap); err != nil {
		w.logger.Errorf("Failed to unmarshal input config: %v", err)
		return fmt.Errorf("failed to unmarshal input config: %w", err)
	}
	inputSpecConfig := spec.NewMapConfig(inputConfigMap)

	// Create wombatwisdom system
	w.logger.Debugf("Creating wombatwisdom NATS system...")
	wwSystem, err := nats.NewSystemFromConfig(systemSpecConfig)
	if err != nil {
		w.logger.Errorf("Failed to create wombatwisdom system: %v", err)
		return fmt.Errorf("failed to create wombatwisdom system: %w", err)
	}
	w.wwSystem = wwSystem
	w.logger.Debugf("wombatwisdom NATS system created successfully")

	// Create wombatwisdom input
	w.logger.Debugf("Creating wombatwisdom NATS input...")
	wwInput, err := nats.NewInputFromConfig(wwSystem, inputSpecConfig)
	if err != nil {
		w.logger.Errorf("Failed to create wombatwisdom input: %v", err)
		return fmt.Errorf("failed to create wombatwisdom input: %w", err)
	}
	w.wwInput = wwInput
	w.logger.Debugf("wombatwisdom NATS input created successfully")

	// Connect the system (wombatwisdom systems use Connect, not Init)
	w.logger.Debugf("Connecting wombatwisdom NATS system...")
	if err := w.wwSystem.Connect(ctx); err != nil {
		w.logger.Errorf("Failed to connect wombatwisdom system: %v", err)
		return fmt.Errorf("failed to connect wombatwisdom system: %w", err)
	}
	w.logger.Debugf("wombatwisdom NATS system connected successfully")

	// Initialize the input component with adapter context
	w.logger.Debugf("Initializing wombatwisdom NATS input...")
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	if err := w.wwInput.Init(componentCtx); err != nil {
		w.logger.Errorf("Failed to initialize wombatwisdom input: %v", err)
		return fmt.Errorf("failed to initialize wombatwisdom input: %w", err)
	}
	w.logger.Debugf("wombatwisdom NATS input initialized successfully")

	w.logger.Info("wombatwisdom NATS input connected successfully")
	return nil
}

func (w *wwNatsInput) Read(ctx context.Context) (*service.Message, service.AckFunc, error) {
	if w.wwInput == nil {
		return nil, nil, fmt.Errorf("input not connected")
	}

	// Read from wombatwisdom input
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	batch, callback, err := w.wwInput.Read(componentCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("wombatwisdom input read failed: %w", err)
	}

	// Convert wombatwisdom batch to Benthos message
	if isBatchEmpty(batch) {
		return nil, nil, fmt.Errorf("empty batch received from wombatwisdom")
	}

	// Get first message from batch
	wwMessage := getFirstMessage(batch)
	if wwMessage == nil {
		return nil, nil, fmt.Errorf("no message in batch from wombatwisdom")
	}

	rawData, err := wwMessage.Raw()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get raw data from wombatwisdom message: %w", err)
	}
	benthosMsg := service.NewMessage(rawData)

	// Copy metadata from wombatwisdom message to Benthos message
	for key, value := range wwMessage.Metadata() {
		benthosMsg.MetaSet(key, fmt.Sprintf("%v", value))
	}

	// Add wombatwisdom source metadata
	benthosMsg.MetaSet("ww_component", "nats")
	benthosMsg.MetaSet("ww_source", "wombatwisdom")

	// Create ack function that bridges to wombatwisdom callback
	ackFunc := func(ctx context.Context, err error) error {
		return callback(ctx, err)
	}

	return benthosMsg, ackFunc, nil
}

func (w *wwNatsInput) Close(ctx context.Context) error {
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	var errs []error

	if w.wwInput != nil {
		if err := w.wwInput.Close(componentCtx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close wombatwisdom input: %w", err))
		}
	}

	if w.wwSystem != nil {
		if err := w.wwSystem.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close wombatwisdom system: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	w.logger.Info("wombatwisdom NATS input closed successfully")
	return nil
}

// wwNatsOutput provides seamless integration between Benthos and wombatwisdom NATS output
type wwNatsOutput struct {
	systemConfig core.SystemConfig
	outputConfig core.OutputConfig
	logger       *service.Logger

	// wombatwisdom components
	wwSystem *nats.System
	wwOutput *nats.Output
}

func (w *wwNatsOutput) Connect(ctx context.Context) error {
	w.logger.Info("Starting wombatwisdom NATS output connection...")

	// Create wombatwisdom config objects
	w.logger.Debugf("Creating NATS output system config: url=%s, name=%s", w.systemConfig.Url, w.systemConfig.Name)
	systemConfigMap := make(map[string]interface{})
	systemConfigBytes, err := json.Marshal(w.systemConfig)
	if err != nil {
		w.logger.Errorf("Failed to marshal system config: %v", err)
		return fmt.Errorf("failed to marshal system config: %w", err)
	}
	if err := json.Unmarshal(systemConfigBytes, &systemConfigMap); err != nil {
		w.logger.Errorf("Failed to unmarshal system config: %v", err)
		return fmt.Errorf("failed to unmarshal system config: %w", err)
	}
	systemSpecConfig := spec.NewMapConfig(systemConfigMap)

	w.logger.Debugf("Creating NATS output config: subject=%s", w.outputConfig.Subject)
	outputConfigMap := make(map[string]interface{})
	outputConfigBytes, err := json.Marshal(w.outputConfig)
	if err != nil {
		w.logger.Errorf("Failed to marshal output config: %v", err)
		return fmt.Errorf("failed to marshal output config: %w", err)
	}
	if err := json.Unmarshal(outputConfigBytes, &outputConfigMap); err != nil {
		w.logger.Errorf("Failed to unmarshal output config: %v", err)
		return fmt.Errorf("failed to unmarshal output config: %w", err)
	}
	outputSpecConfig := spec.NewMapConfig(outputConfigMap)

	// Create wombatwisdom system
	w.logger.Debugf("Creating wombatwisdom NATS output system...")
	wwSystem, err := nats.NewSystemFromConfig(systemSpecConfig)
	if err != nil {
		w.logger.Errorf("Failed to create wombatwisdom system: %v", err)
		return fmt.Errorf("failed to create wombatwisdom system: %w", err)
	}
	w.wwSystem = wwSystem
	w.logger.Debugf("wombatwisdom NATS output system created successfully")

	// Create wombatwisdom output
	w.logger.Debugf("Creating wombatwisdom NATS output...")
	wwOutput, err := nats.NewOutputFromConfig(wwSystem, outputSpecConfig)
	if err != nil {
		w.logger.Errorf("Failed to create wombatwisdom output: %v", err)
		return fmt.Errorf("failed to create wombatwisdom output: %w", err)
	}
	w.wwOutput = wwOutput
	w.logger.Debugf("wombatwisdom NATS output created successfully")

	// Connect the system (wombatwisdom systems use Connect, not Init)
	w.logger.Debugf("Connecting wombatwisdom NATS output system...")
	if err := w.wwSystem.Connect(ctx); err != nil {
		w.logger.Errorf("Failed to connect wombatwisdom system: %v", err)
		return fmt.Errorf("failed to connect wombatwisdom system: %w", err)
	}
	w.logger.Debugf("wombatwisdom NATS output system connected successfully")

	// Initialize the output component with adapter context
	w.logger.Debugf("Initializing wombatwisdom NATS output...")
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	if err := w.wwOutput.Init(componentCtx); err != nil {
		w.logger.Errorf("Failed to initialize wombatwisdom output: %v", err)
		return fmt.Errorf("failed to initialize wombatwisdom output: %w", err)
	}
	w.logger.Debugf("wombatwisdom NATS output initialized successfully")

	w.logger.Info("wombatwisdom NATS output connected successfully")
	return nil
}

func (w *wwNatsOutput) Write(ctx context.Context, msg *service.Message) error {
	if w.wwOutput == nil {
		return fmt.Errorf("output not connected")
	}

	// Convert Benthos message to wombatwisdom message and batch
	wwMsg := &benthosToWombatwisdomMessageAdapter{msg: msg}
	wwBatch := &simpleBatch{}
	wwBatch.Append(wwMsg)

	// Write to wombatwisdom output
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	if err := w.wwOutput.Write(componentCtx, wwBatch); err != nil {
		return fmt.Errorf("wombatwisdom output write failed: %w", err)
	}

	return nil
}

func (w *wwNatsOutput) Close(ctx context.Context) error {
	componentCtx := &benthosTowombatwisdomContextAdapter{
		ctx:    ctx,
		logger: w.logger,
	}

	var errs []error

	if w.wwOutput != nil {
		if err := w.wwOutput.Close(componentCtx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close wombatwisdom output: %w", err))
		}
	}

	if w.wwSystem != nil {
		if err := w.wwSystem.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close wombatwisdom system: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	w.logger.Info("wombatwisdom NATS output closed successfully")
	return nil
}
