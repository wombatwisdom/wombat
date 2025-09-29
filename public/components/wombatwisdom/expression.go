package wombatwisdom

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/wombatwisdom/components/framework/spec"
)

// ExpressionParser uses expr-lang to parse and evaluate expressions.
// This matches the expression language used internally by wombatwisdom components,
// ensuring consistent syntax for users. For example:
// - Access JSON fields: json.device_id
// - Access metadata: metadata.topic_prefix
// - String concatenation: "prefix/" + json.field + "/suffix"

func NewExpressionParser() *ExpressionParser {
	return &ExpressionParser{}
}

type ExpressionParser struct{}

func (e *ExpressionParser) ParseExpression(expression string) (spec.Expression, error) {
	program, err := expr.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression '%s': %w", expression, err)
	}

	return &ExprLangExpression{
		program: program,
		source:  expression,
	}, nil
}

type ExprLangExpression struct {
	program *vm.Program
	source  string
}

func (e *ExprLangExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	result, err := vm.Run(e.program, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate expression '%s': %w", e.source, err)
	}

	if result == nil {
		return "", nil
	}

	if str, ok := result.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", result), nil
}

func (e *ExprLangExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	result, err := vm.Run(e.program, ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate expression '%s': %w", e.source, err)
	}

	if result == nil {
		return 0, nil
	}

	switch v := result.(type) {
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
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("expression '%s' evaluated to %T, expected numeric type", e.source, result)
	}
}

func (e *ExprLangExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	result, err := vm.Run(e.program, ctx)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression '%s': %w", e.source, err)
	}

	if result == nil {
		return false, nil
	}

	if b, ok := result.(bool); ok {
		return b, nil
	}

	return false, fmt.Errorf("expression '%s' evaluated to %T, expected bool", e.source, result)
}
