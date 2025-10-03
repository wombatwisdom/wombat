package wombatwisdom

import (
	"testing"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/wombatwisdom/components/framework/spec"
)

func TestDebugExpressionContext(t *testing.T) {
	// Create the same context structure
	ctx := spec.ExpressionContext{
		"id": "Bloop",
		"topic": "cat",
		"type": "cat",
		"json": map[string]interface{}{
			"id": "Bloop",
			"topic": "cat", 
			"type": "cat",
		},
		"metadata": map[string]interface{}{},
	}
	
	// Test direct bloblang
	env := bloblang.GlobalEnvironment()
	expr, err := env.Parse(`this.topic`)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	
	// Test with the actual context
	result, err := expr.Query(ctx)
	t.Logf("Direct query result: %v, error: %v", result, err)
	
	// Also test a simple return
	expr2, err := env.Parse(`"hello"`)
	if err != nil {
		t.Fatalf("Failed to parse hello: %v", err)
	}
	result2, err := expr2.Query(ctx)
	t.Logf("Hello query result: %v, error: %v", result2, err)
	
	// Test root
	expr3, err := env.Parse(`root`)
	if err != nil {
		t.Fatalf("Failed to parse root: %v", err)
	}
	result3, err := expr3.Query(ctx)
	t.Logf("Root query result: %#v, error: %v", result3, err)
}