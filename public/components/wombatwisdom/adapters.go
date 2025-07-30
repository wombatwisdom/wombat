package wombatwisdom

import (
	"context"
	"fmt"
	"iter"
	"regexp"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
)

// benthosTowombatwisdomContextAdapter adapts Benthos context to wombatwisdom ComponentContext
type benthosTowombatwisdomContextAdapter struct {
	ctx    context.Context
	logger *service.Logger
}

func (a *benthosTowombatwisdomContextAdapter) Context() context.Context {
	return a.ctx
}

// Logger interface implementation
func (a *benthosTowombatwisdomContextAdapter) Debugf(format string, args ...interface{}) {
	a.logger.Debugf(format, args...)
}

func (a *benthosTowombatwisdomContextAdapter) Infof(format string, args ...interface{}) {
	a.logger.Infof(format, args...)
}

func (a *benthosTowombatwisdomContextAdapter) Warnf(format string, args ...interface{}) {
	a.logger.Warnf(format, args...)
}

func (a *benthosTowombatwisdomContextAdapter) Errorf(format string, args ...interface{}) {
	a.logger.Errorf(format, args...)
}

// ExpressionFactory interface implementation
func (a *benthosTowombatwisdomContextAdapter) ParseExpression(expr string) (spec.Expression, error) {
	return &simpleExpression{expr: expr}, nil
}

// MessageFactory interface implementation
func (a *benthosTowombatwisdomContextAdapter) NewBatch() spec.Batch {
	return &simpleBatch{}
}

func (a *benthosTowombatwisdomContextAdapter) NewMessage() spec.Message {
	return spec.NewBytesMessage([]byte{})
}

// MetadataFilterFactory interface implementation
func (a *benthosTowombatwisdomContextAdapter) BuildMetadataFilter(patterns []string, invert bool) (spec.MetadataFilter, error) {
	return &simpleMetadataFilter{patterns: patterns, invert: invert}, nil
}

// Resources interface implementation (stub)
func (a *benthosTowombatwisdomContextAdapter) Resources() spec.ResourceManager {
	return spec.NewResourceManager(a.ctx, a)
}

// Component access stubs
func (a *benthosTowombatwisdomContextAdapter) Input(name string) (spec.Input, error) {
	return nil, fmt.Errorf("cross-component access not implemented")
}

func (a *benthosTowombatwisdomContextAdapter) Output(name string) (spec.Output, error) {
	return nil, fmt.Errorf("cross-component access not implemented")
}

func (a *benthosTowombatwisdomContextAdapter) System(name string) (spec.System, error) {
	return nil, fmt.Errorf("cross-component access not implemented")
}

// benthosToWombatwisdomMessageAdapter adapts Benthos message to wombatwisdom Message
type benthosToWombatwisdomMessageAdapter struct {
	msg *service.Message
}

func (m *benthosToWombatwisdomMessageAdapter) SetMetadata(key string, value any) {
	m.msg.MetaSet(key, fmt.Sprintf("%v", value))
}

func (m *benthosToWombatwisdomMessageAdapter) SetRaw(b []byte) {
	m.msg = service.NewMessage(b)
}

func (m *benthosToWombatwisdomMessageAdapter) Raw() ([]byte, error) {
	data, err := m.msg.AsBytes()
	return data, err
}

func (m *benthosToWombatwisdomMessageAdapter) Metadata() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		_ = m.msg.MetaWalkMut(func(key string, value any) error {
			if !yield(key, value) {
				return fmt.Errorf("iteration stopped")
			}
			return nil
		})
	}
}

// simpleBatch implements wombatwisdom Batch interface
type simpleBatch struct {
	messages []spec.Message
}

func (b *simpleBatch) Messages() iter.Seq2[int, spec.Message] {
	return func(yield func(int, spec.Message) bool) {
		for i, msg := range b.messages {
			if !yield(i, msg) {
				return
			}
		}
	}
}

func (b *simpleBatch) Append(msg spec.Message) {
	b.messages = append(b.messages, msg)
}

// Helper function to get first message from batch
func getFirstMessage(batch spec.Batch) spec.Message {
	for _, msg := range batch.Messages() {
		return msg
	}
	return nil
}

// Helper function to check if batch is empty
func isBatchEmpty(batch spec.Batch) bool {
	for range batch.Messages() {
		return false
	}
	return true
}

// simpleExpression implements basic expression evaluation
type simpleExpression struct {
	expr string
}

func (e *simpleExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	return e.expr, nil
}

func (e *simpleExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	return 0, nil
}

func (e *simpleExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	return true, nil
}

// simpleMetadataFilter implements basic metadata filtering
type simpleMetadataFilter struct {
	patterns []string
	invert   bool
}

func (f *simpleMetadataFilter) Include(key string) bool {
	matched := false
	for _, pattern := range f.patterns {
		if regexp.MustCompile(pattern).MatchString(key) {
			matched = true
			break
		}
	}
	if f.invert {
		return !matched
	}
	return matched
}

