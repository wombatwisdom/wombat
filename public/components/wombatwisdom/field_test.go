package wombatwisdom

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
)

func TestFieldAccess(t *testing.T) {
	// Create a message with JSON data
	msg := &BenthosMessage{Message: service.NewMessage([]byte(`{"topic": "cat", "id": "Bloop", "type": "cat"}`))}
	ctx := BlobExpressionContext(msg)

	// Log the context structure
	t.Logf("Context structure: %#v", ctx)

	ex := expression(t, `${! this.topic }`)
	result, err := ex.Eval(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "cat", result)

	// Also test that our json() function still works
	ex2 := expression(t, `${! json("topic") }`)
	result2, err := ex2.Eval(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "cat", result2)
}
