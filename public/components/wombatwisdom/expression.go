package wombatwisdom

import (
	"encoding/json"
	"fmt"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/components/framework/spec"
)

func BlobExpressionContext(msg spec.Message) spec.ExpressionContext {
	ctx := make(spec.ExpressionContext)
	ctx["message"] = msg

	// add json fields to root
	rawData, err := msg.Raw()
	if err == nil && len(rawData) > 0 {
		var jsonData interface{}
		if err := json.Unmarshal(rawData, &jsonData); err == nil {
			// Add parsed JSON fields at root for field access
			if m, ok := jsonData.(map[string]interface{}); ok {
				for k, v := range m {
					ctx[k] = v
				}
			}
			// Also store complete JSON under "json" key for json() function
			ctx["json"] = jsonData
		}
	}

	// handle metadata
	metadata := make(map[string]interface{})
	for k, v := range msg.Metadata() {
		metadata[k] = v
	}
	ctx["metadata"] = metadata

	return ctx
}

func NewInterpolatedExpression(is *service.InterpolatedString) spec.Expression {
	return &interpolatedExpression{
		ble: is,
	}
}

type interpolatedExpression struct {
	ble *service.InterpolatedString
}

func (b *interpolatedExpression) Eval(ctx spec.ExpressionContext) (string, error) {
	msg, fnd := ctx["message"]
	if !fnd {
		return "", fmt.Errorf("expression context missing 'message' key")
	}

	bmsg, ok := msg.(*BenthosMessage)
	if !ok {
		return "", fmt.Errorf("message is not a benthos message")
	}

	return b.ble.TryString(bmsg.Message)
}
