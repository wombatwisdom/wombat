package wombatwisdom

import (
	"encoding/json"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/field"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/parser"
	"github.com/wombatwisdom/wombat/internal/benthos/public/message"

	"github.com/wombatwisdom/components/framework/spec"
)

func NewBenthosInterpolationExpressionParser() *ExpressionParser {
	return &ExpressionParser{
		ctx: parser.GlobalContext(),
	}
}

type ExpressionParser struct {
	ctx parser.Context
}

func (e *ExpressionParser) ParseExpression(expr string) (spec.Expression, error) {
	fieldExpression, err := parser.ParseField(e.ctx, expr)
	if err != nil {
		return nil, err
	}

	return &FieldExpression{
		expr: fieldExpression,
	}, nil
}

// FieldExpression wraps a benthos field.Expression to implement spec.Expression
type FieldExpression struct {
	expr *field.Expression
}

// simpleMessage implements field.Message (benthos)
type simpleMessage struct {
	ctx spec.ExpressionContext
}

func (s *simpleMessage) Get(p int) *message.Part {
	if jsonData, ok := s.ctx["json"].(map[string]interface{}); ok {
		jsonBytes, err := json.Marshal(jsonData)
		if err == nil {
			part := message.NewPart(jsonBytes)
			// Add metadata if available
			if meta, ok := s.ctx["metadata"].(map[string]interface{}); ok {
				for k, v := range meta {
					if str, ok := v.(string); ok {
						part.MetaSetMut(k, str)
					}
				}
			}

			return part
		}
	}

	// Fallback: create empty message part
	part := message.NewPart([]byte("{}"))

	// Add metadata if available
	if meta, ok := s.ctx["metadata"].(map[string]interface{}); ok {
		for k, v := range meta {
			if str, ok := v.(string); ok {
				part.MetaSetMut(k, str)
			}
		}
	}

	return part
}

func (s *simpleMessage) Len() int {
	return 1
}

func (f *FieldExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	return f.expr.String(0, &simpleMessage{ctx: ctx})
}

func (f *FieldExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	panic("EvalInt not implemented")
}

func (f *FieldExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	panic("EvalBool not implemented")
}
