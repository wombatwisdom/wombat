package wombatwisdom

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/wombatwisdom/components/framework/spec"
)

func expression(t *testing.T, s string) spec.Expression {
	is, err := service.NewInterpolatedString(s)
	assert.NoError(t, err)

	return NewInterpolatedExpression(is)
}

func msgCtx(data string, headers map[string]any) spec.ExpressionContext {
	m := service.NewMessage([]byte(data))
	for k, v := range headers {
		m.MetaSetMut(k, v)
	}

	return BlobExpressionContext(&BenthosMessage{Message: m})
}

func TestBenthosExpression(t *testing.T) {
	ctx := msgCtx(`{"sensor_type": "temperature", "value": 23.5}`, map[string]any{
		"device_id": "device-001",
	})

	// Test json() function
	ex1 := expression(t, `${! json("sensor_type") }`)
	result, err := ex1.Eval(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "temperature", result)

	// Test meta() function
	ex2 := expression(t, `${! meta("device_id") }`)
	result, err = ex2.Eval(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "device-001", result)
}
