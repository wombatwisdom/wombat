package wombatwisdom

import (
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/wombatwisdom/components/framework/spec"
)

func NewExpressionParser() *ExpressionParser {
	return &ExpressionParser{
		blob: bloblang.GlobalEnvironment(),
	}
}

type ExpressionParser struct {
	blob *bloblang.Environment
}

func (e *ExpressionParser) ParseExpression(expr string) (spec.Expression, error) {
	be, err := e.blob.Parse(expr)
	if err != nil {
		return nil, err
	}

	return &BloblangExpression{
		expr: be,
	}, nil
}

type BloblangExpression struct {
	expr *bloblang.Executor
}

func (b *BloblangExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	resp, err := b.expr.Query(ctx)
	if err != nil {
		return "", err
	}

	respStr, ok := resp.(string)
	if !ok {
		return "", fmt.Errorf("expected 'string' response, got %T", resp)
	}
	return respStr, nil
}

func (b *BloblangExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	resp, err := b.expr.Query(ctx)
	if err != nil {
		return 0, err
	}
	switch v := resp.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("expected 'int' response, got %T", resp)
	}
}

func (b *BloblangExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	resp, err := b.expr.Query(ctx)
	if err != nil {
		return false, err
	}

	respBool, ok := resp.(bool)
	if !ok {
		return false, fmt.Errorf("expected 'string' response, got %T", resp)
	}
	return respBool, nil
}
