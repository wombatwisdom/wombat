package wombatwisdom

import (
	"testing"

	"github.com/wombatwisdom/components/framework/spec"
)

func TestBenthosExpression(t *testing.T) {
	factory := &BloblangExpressionFactory{}
	
	// Test json() function
	expr, err := factory.ParseExpression(`json("sensor_type")`)
	if err != nil {
		t.Fatalf("Failed to parse json() expression: %v", err)
	}

	ctx := spec.ExpressionContext{
		"json": map[string]interface{}{
			"sensor_type": "temperature",
		},
	}

	result, err := expr.EvalString(ctx)
	if err != nil {
		t.Errorf("Failed to evaluate json() expression: %v", err)
	} else if result != "temperature" {
		t.Errorf("Expected 'temperature', got '%s'", result)
	}

	// Test meta() function
	expr2, err := factory.ParseExpression(`meta("device_id")`)
	if err != nil {
		t.Fatalf("Failed to parse meta() expression: %v", err)
	}

	ctx2 := spec.ExpressionContext{
		"metadata": map[string]interface{}{
			"device_id": "device-001",
		},
	}

	result2, err := expr2.EvalString(ctx2)
	if err != nil {
		t.Errorf("Failed to evaluate meta() expression: %v", err)
	} else if result2 != "device-001" {
		t.Errorf("Expected 'device-001', got '%s'", result2)
	}
}
