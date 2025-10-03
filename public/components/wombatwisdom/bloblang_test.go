package wombatwisdom

import (
	"testing"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
)

func TestBloblangDirectly(t *testing.T) {
	// Test with regular map[string]interface{}
	env := bloblang.GlobalEnvironment()
	
	expr, err := env.Parse(`this.topic`)
	if err != nil {
		t.Fatalf("Failed to parse expression: %v", err)
	}
	
	// Test with different data structures
	testCases := []struct {
		name string
		data interface{}
	}{
		{
			name: "map[string]interface{}",
			data: map[string]interface{}{
				"topic": "cat",
				"id": "Bloop",
			},
		},
		{
			name: "map[string]any",
			data: map[string]any{
				"topic": "cat", 
				"id": "Bloop",
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := expr.Query(tc.data)
			t.Logf("%s: result=%v, error=%v, type=%T", tc.name, result, err, result)
		})
	}
}