package wombatwisdom

import (
	"context"

	"github.com/wombatwisdom/components/framework/spec"
)

func NewComponentContext(ctx context.Context, logger spec.Logger) *ComponentContext {
	return &ComponentContext{
		ctx:                   ctx,
		Logger:                logger,
		ExpressionParser:      NewBenthosInterpolationExpressionParser(),
		MessageFactory:        &MessageFactory{},
		MetadataFilterFactory: &MetadataFilterFactory{},
	}
}

type ComponentContext struct {
	ctx context.Context
	spec.Logger
	*ExpressionParser
	*MessageFactory
	*MetadataFilterFactory
}

func (c *ComponentContext) Context() context.Context {
	return c.ctx
}
