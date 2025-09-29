package wombatwisdom

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/redpanda-data/benthos/v4/public/service"
)

// PrepareExprLangExpression takes a string value and prepares it for expr-lang evaluation.
// If the input is a simple literal (like "test" or "test/mqtt/output"), it wraps it in quotes.
// If it's already a valid expression, it returns it unchanged.
func PrepareExprLangExpression(input string) string {
	tree, err := parser.Parse(input)
	if err == nil {
		if _, isString := tree.Node.(*ast.StringNode); isString {
			// Already quoted, return as-is
			return input
		}
		if !isSimpleLiteral(tree) && !looksLikeLiteral(input) {
			// It's a complex expression, return as-is
			return input
		}
	}

	// In all other cases, wrap it in quotes
	// This catches:
	// - Simple identifiers: test, production → "test", "production"
	// - Paths with slashes: test/mqtt/output → "test/mqtt/output"
	// - Strings with spaces: test mqtt output → "test mqtt output"
	// - Numeric literals: 123 → "123"
	// - Boolean/nil literals: true, false, nil → "true", "false", "nil"
	// - Invalid expr syntax: test@production, test#1 → "test@production", "test#1"
	return fmt.Sprintf(`"%s"`, input)
}

// looksLikeLiteral checks if the input string looks like it should be a literal
// rather than an expression (e.g., contains slashes that would be division operators)
func looksLikeLiteral(input string) bool {
	// If it contains dots (like json.field), it's likely an expression
	if strings.Contains(input, ".") && !strings.Contains(input, `"`) {
		return false
	}

	// If it contains common operators, it's likely an expression
	operators := []string{" + ", " - ", " * ", " == ", " != ", " ? ", " : ", " ?? "}
	for _, op := range operators {
		if strings.Contains(input, op) {
			return false
		}
	}

	// If it starts with a known prefix, it's likely an expression
	knownPrefixes := []string{"json.", "metadata.", "content", "env.", "this."}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(input, prefix) {
			return false
		}
	}

	return true
}

// isSimpleLiteral checks if the parsed AST is just a simple identifier
// that would be interpreted as a variable reference rather than a string literal
func isSimpleLiteral(tree *parser.Tree) bool {
	if tree == nil || tree.Node == nil {
		return false
	}

	// Check if the root node is just an identifier
	switch tree.Node.(type) {
	case *ast.IdentifierNode:
		// It's a simple identifier like "test" or "test/mqtt/output"
		// These need quotes to be treated as string literals
		return true
	case *ast.StringNode:
		// It's already a quoted string, don't need to do anything
		return false
	case *ast.BinaryNode:
		// It's an expression with operators, leave it as-is
		return false
	case *ast.MemberNode:
		// It's a member access like json.field, leave it as-is
		return false
	default:
		// For any other node type, assume it's an expression
		return false
	}
}

// NewExprLangField creates a string field that's designed for expr-lang expressions.
// It provides helpful documentation and examples for users.
func NewExprLangField(name string) *service.ConfigField {
	return service.NewStringField(name).
		Description("An expr-lang expression. Simple string literals will be automatically quoted. " +
			"For dynamic values, use expressions like json.field_name or metadata.key.")
}
