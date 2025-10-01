package wombatwisdom

import (
	"context"

	"github.com/wombatwisdom/components/framework/spec"
)

func NewComponentContext(ctx context.Context, logger spec.Logger) *ComponentContext {
	benthosFactory := NewBenthosExpressionFactory()
	return &ComponentContext{
		ctx:                       ctx,
		Logger:                    logger,
		benthosFactory:            benthosFactory,
		interpolatedFactory:       spec.NewInterpolatedExpressionFactory(benthosFactory),
		MessageFactory:            &MessageFactory{},
		MetadataFilterFactory:     &MetadataFilterFactory{},
	}
}

type ComponentContext struct {
	ctx context.Context
	spec.Logger
	benthosFactory      *BenthosExpressionFactory
	interpolatedFactory *spec.InterpolatedExpressionFactory
	*MessageFactory
	*MetadataFilterFactory
}

func (c *ComponentContext) Context() context.Context {
	return c.ctx
}

// ParseExpression parses a Benthos expression
func (c *ComponentContext) ParseExpression(expr string) (spec.Expression, error) {
	return c.benthosFactory.ParseExpression(expr)
}

// ParseInterpolatedExpression parses a string that may contain ${!...} interpolations
func (c *ComponentContext) ParseInterpolatedExpression(expr string) (spec.InterpolatedExpression, error) {
	return c.interpolatedFactory.ParseInterpolatedExpression(expr)
}
