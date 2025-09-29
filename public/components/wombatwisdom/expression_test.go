package wombatwisdom

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wombatwisdom/components/framework/spec"
)

func TestExprLangExpression(t *testing.T) {
	parser := NewExpressionParser()

	t.Run("static string", func(t *testing.T) {
		expr, err := parser.ParseExpression(`"hello world"`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("string concatenation", func(t *testing.T) {
		expr, err := parser.ParseExpression(`"hello " + "world"`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("access json field", func(t *testing.T) {
		expr, err := parser.ParseExpression(`json.device_id`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{
				"device_id": "sensor-123",
			},
		}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "sensor-123", result)
	})

	t.Run("concatenate with json field", func(t *testing.T) {
		expr, err := parser.ParseExpression(`"test/devices/" + json.device_id + "/data"`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{
				"device_id": "sensor-123",
			},
		}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "test/devices/sensor-123/data", result)
	})

	t.Run("access metadata field", func(t *testing.T) {
		expr, err := parser.ParseExpression(`metadata.topic_prefix + "/" + json.type`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"metadata": map[string]interface{}{
				"topic_prefix": "alerts",
			},
			"json": map[string]interface{}{
				"type": "critical",
			},
		}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "alerts/critical", result)
	})

	t.Run("null coalescing", func(t *testing.T) {
		expr, err := parser.ParseExpression(`json.device_id ?? "unknown"`)
		require.NoError(t, err)

		// Test with missing field
		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{},
		}
		result, err := expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "unknown", result)

		// Test with present field
		ctx = spec.ExpressionContext{
			"json": map[string]interface{}{
				"device_id": "sensor-456",
			},
		}
		result, err = expr.EvalString(ctx)
		require.NoError(t, err)
		assert.Equal(t, "sensor-456", result)
	})

	t.Run("no this keyword", func(t *testing.T) {
		// This should fail because 'this' is not in the context
		expr, err := parser.ParseExpression(`this.device_id`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{
				"device_id": "sensor-123",
			},
		}
		_, err = expr.EvalString(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "this")
	})

	t.Run("integer evaluation", func(t *testing.T) {
		expr, err := parser.ParseExpression(`json.count + 10`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{
				"count": 5,
			},
		}
		result, err := expr.EvalInt(ctx)
		require.NoError(t, err)
		assert.Equal(t, 15, result)
	})

	t.Run("boolean evaluation", func(t *testing.T) {
		expr, err := parser.ParseExpression(`json.active && metadata.enabled`)
		require.NoError(t, err)

		ctx := spec.ExpressionContext{
			"json": map[string]interface{}{
				"active": true,
			},
			"metadata": map[string]interface{}{
				"enabled": true,
			},
		}
		result, err := expr.EvalBool(ctx)
		require.NoError(t, err)
		assert.True(t, result)
	})
}
