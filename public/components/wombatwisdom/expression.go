package wombatwisdom

import (
	"encoding/json"
	"fmt"
	
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/field"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/parser"
	"github.com/wombatwisdom/wombat/internal/benthos/public/message"
	"github.com/wombatwisdom/components/framework/spec"
)

// BenthosExpressionFactory implements spec.ExpressionFactory using internal Benthos parser
type BenthosExpressionFactory struct {
	ctx parser.Context
}

// NewBenthosExpressionFactory creates a new expression factory using internal Benthos parser
func NewBenthosExpressionFactory() *BenthosExpressionFactory {
	return &BenthosExpressionFactory{
		ctx: parser.GlobalContext(),
	}
}

// ParseExpression parses a Benthos expression
func (b *BenthosExpressionFactory) ParseExpression(expr string) (spec.Expression, error) {
	// Wrap the expression in interpolation syntax so ParseField can evaluate it
	wrappedExpr := "${!" + expr + "}"
	fieldExpr, err := parser.ParseField(b.ctx, wrappedExpr)
	if err != nil {
		return nil, err
	}

	return &benthosExpression{
		expr: fieldExpr,
	}, nil
}

// benthosExpression wraps a Benthos field expression to implement spec.Expression
type benthosExpression struct {
	expr *field.Expression
}

// simpleMessage implements field.Message to bridge our context with Benthos
type simpleMessage struct {
	ctx spec.ExpressionContext
}

func (s *simpleMessage) Get(p int) *message.Part {
	// Create a message part from our context
	var content []byte
	
	if jsonData, ok := s.ctx["json"].(map[string]interface{}); ok {
		// If we have parsed JSON, use that
		if data, err := json.Marshal(jsonData); err == nil {
			content = data
		}
	} else if contentStr, ok := s.ctx["content"].(string); ok {
		// Otherwise use raw content
		content = []byte(contentStr)
	}
	
	if content == nil {
		content = []byte("{}")
	}
	
	part := message.NewPart(content)
	
	// Add metadata if available
	if metadata, ok := s.ctx["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
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

// EvalString evaluates the expression and returns a string
func (e *benthosExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	msg := &simpleMessage{ctx: ctx}
	return e.expr.String(0, msg)
}

// EvalInt evaluates the expression and returns an int
func (e *benthosExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	// The field.Expression only supports String/Bytes, so we need to parse the result
	msg := &simpleMessage{ctx: ctx}
	str, err := e.expr.String(0, msg)
	if err != nil {
		return 0, err
	}
	// For now, return an error as int parsing is not implemented
	// This would need proper type conversion logic
	return 0, fmt.Errorf("int evaluation not implemented for value: %s", str)
}

// EvalBool evaluates the expression and returns a bool
func (e *benthosExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	// The field.Expression only supports String/Bytes, so we need to parse the result
	msg := &simpleMessage{ctx: ctx}
	str, err := e.expr.String(0, msg)
	if err != nil {
		return false, err
	}
	// For now, return an error as bool parsing is not implemented
	// This would need proper type conversion logic
	return false, fmt.Errorf("bool evaluation not implemented for value: %s", str)
}

