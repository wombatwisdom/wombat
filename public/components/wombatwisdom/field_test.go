package wombatwisdom

import (
	"testing"

	"github.com/wombatwisdom/components/framework/spec"
)

func TestFieldAccess(t *testing.T) {
	factory := &BloblangExpressionFactory{}

	// Test field access with this.topic
	expr, err := factory.ParseExpression(`this.topic`)
	if err != nil {
		t.Fatalf("Failed to parse this.topic expression: %v", err)
	}

	// Create a message with JSON data
	msg := spec.NewBytesMessage([]byte(`{"topic": "cat", "id": "Bloop", "type": "cat"}`))
	ctx := BlobExpressionContext(msg)

	// Log the context structure
	t.Logf("Context structure: %#v", ctx)

	// Try with explicit map[string]interface{} conversion
	convertedCtx := make(map[string]interface{})
	for k, v := range ctx {
		convertedCtx[k] = v
	}
	t.Logf("Converted context type: %T", convertedCtx)

	result, err := expr.EvalString(ctx)
	t.Logf("EvalString result: %v, error: %v", result, err)
	if err != nil {
		t.Errorf("Failed to evaluate this.topic expression: %v", err)
	} else {
		t.Logf("Result: %s", result)
		if result != "cat" {
			t.Errorf("Expected 'cat', got '%s'", result)
		}
	}

	// Also test that our json() function still works
	expr2, err := factory.ParseExpression(`json("topic")`)
	if err != nil {
		t.Fatalf("Failed to parse json() expression: %v", err)
	}

	result2, err := expr2.EvalString(ctx)
	t.Logf("json() result: %v, error: %v", result2, err)
}
