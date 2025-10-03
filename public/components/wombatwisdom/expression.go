package wombatwisdom

import (
	"encoding/json"
	"fmt"
	"github.com/wombatwisdom/components/framework/spec"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/mapping"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/parser"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/query"
	"github.com/wombatwisdom/wombat/internal/benthos/public/bloblang/value"
	"github.com/wombatwisdom/wombat/internal/benthos/public/message"
)

// BloblangExpressionFactory implements ExpressionFactory using bloblang
type BloblangExpressionFactory struct{}

func (b BloblangExpressionFactory) ParseExpression(exprString string) (spec.Expression, error) {
	// Parse using Benthos's parser directly to get access to all built-in functions
	parserCtx := parser.GlobalContext()
	mappingExec, err := parser.ParseMapping(parserCtx, exprString)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression '%s': %w", exprString, err)
	}

	return &blobExpression{
		mappingExec: mappingExec,
	}, nil
}

func BlobExpressionContext(msg spec.Message) spec.ExpressionContext {
	ctx := make(spec.ExpressionContext)

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

type BloblangExpression interface {
	EvalString(ctx spec.ExpressionContext) (string, error)
	EvalInt(ctx spec.ExpressionContext) (int, error)
	EvalBool(ctx spec.ExpressionContext) (bool, error)
}

type blobExpression struct {
	mappingExec *mapping.Executor
}

func (b *blobExpression) EvalString(ctx spec.ExpressionContext) (string, error) {
	// Extract JSON data and metadata from context
	jsonData := ctx["json"]
	metadata := make(map[string]string)

	// Convert metadata to string map
	if metaRaw, ok := ctx["metadata"].(map[string]interface{}); ok {
		for k, v := range metaRaw {
			metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	// Create raw bytes from JSON data
	var rawBytes []byte
	if jsonData != nil {
		var err error
		rawBytes, err = json.Marshal(jsonData)
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON: %w", err)
		}
	}

	// Create a message part with our data
	msgPart := message.NewPart(rawBytes)

	// Set metadata on the part
	for k, v := range metadata {
		msgPart.MetaSetMut(k, v)
	}

	// If we have structured data, set it directly
	if jsonData != nil {
		msgPart.SetStructuredMut(jsonData)
	}

	// Create a message batch with our part
	batch := message.Batch{msgPart}

	// Create function context with our message batch
	// For 'this.' references to work, we need to pass just the JSON data as the root value
	var rootValue interface{} = ctx
	if jsonData != nil {
		// If we have JSON data, use that as the root value for 'this.' references
		rootValue = jsonData
	}
	
	funcCtx := query.FunctionContext{
		Maps:     b.mappingExec.Maps(),
		Vars:     map[string]any{},
		Index:    0,
		MsgBatch: batch,
	}.WithValue(rootValue)

	// Execute the mapping
	result, err := b.mappingExec.Exec(funcCtx)
	if err != nil {
		return "", err
	}

	// Handle the result
	switch v := result.(type) {
	case value.Delete:
		return "", fmt.Errorf("value was deleted")
	case value.Nothing:
		// Return empty string for Nothing
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case nil:
		return "", nil
	default:
		// For complex types, try to marshal to JSON
		if bytes, err := json.Marshal(v); err == nil {
			return string(bytes), nil
		}
		return fmt.Sprintf("%v", v), nil
	}
}

func (b *blobExpression) EvalInt(ctx spec.ExpressionContext) (int, error) {
	panic("implement me")
}

func (b *blobExpression) EvalBool(ctx spec.ExpressionContext) (bool, error) {
	panic("implement me")
}
