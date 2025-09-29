package wombatwisdom

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareExprLangExpression(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple literal",
			input:    "test",
			expected: `"test"`,
		},
		{
			name:     "literal with slashes",
			input:    "test/mqtt/output",
			expected: `"test/mqtt/output"`,
		},
		{
			name:     "already quoted string",
			input:    `"test/mqtt/output"`,
			expected: `"test/mqtt/output"`,
		},
		{
			name:     "json field access",
			input:    "json.device_id",
			expected: "json.device_id",
		},
		{
			name:     "metadata field access",
			input:    "metadata.topic_prefix",
			expected: "metadata.topic_prefix",
		},
		{
			name:     "string concatenation with json field",
			input:    `"devices/" + json.device_id + "/data"`,
			expected: `"devices/" + json.device_id + "/data"`,
		},
		{
			name:     "complex expression",
			input:    `json.type == "sensor" ? "sensors/" + json.id : "devices/" + json.id`,
			expected: `json.type == "sensor" ? "sensors/" + json.id : "devices/" + json.id`,
		},
		{
			name:     "null coalescing",
			input:    `json.device_id ?? "unknown"`,
			expected: `json.device_id ?? "unknown"`,
		},
		{
			name:     "invalid expression gets quoted",
			input:    "test mqtt output", // spaces make it invalid as identifier
			expected: `"test mqtt output"`,
		},
		{
			name:     "numeric literal",
			input:    "123",
			expected: `"123"`, // Will be quoted as a literal
		},
		{
			name:     "boolean literal",
			input:    "true",
			expected: `"true"`, // true as identifier needs quotes to be a string
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := PrepareExprLangExpression(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
