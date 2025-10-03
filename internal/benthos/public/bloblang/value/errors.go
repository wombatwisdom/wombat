// Copyright 2025 Redpanda Data, Inc.

package value

import (
	"bytes"
	"fmt"
)

// TypeError represents an error where a value of a type was required for a
// function, method or operator but instead a different type was found.
type TypeError struct {
	From     string
	Expected []Type
	Actual   Type
	Value    string
}

// Error implements the standard error interface for TypeError.
func (t *TypeError) Error() string {
	var errStr bytes.Buffer
	if len(t.Expected) > 0 {
		errStr.WriteString("expected ")
		for i, exp := range t.Expected {
			if i > 0 {
				if len(t.Expected) > 2 && i < (len(t.Expected)-1) {
					errStr.WriteString(", ")
				} else {
					errStr.WriteString(" or ")
				}
			}
			errStr.WriteString(string(exp))
		}
		errStr.WriteString(" value")
	} else {
		errStr.WriteString("unexpected value")
	}

	fmt.Fprintf(&errStr, ", got %v", t.Actual)

	if t.From != "" {
		fmt.Fprintf(&errStr, " from %v", t.From)
	}

	if t.Value != "" {
		fmt.Fprintf(&errStr, " (%v)", t.Value)
	}

	return errStr.String()
}

// NewTypeError creates a new type error.
func NewTypeError(value any, exp ...Type) *TypeError {
	return NewTypeErrorFrom("", value, exp...)
}

// NewTypeErrorFrom creates a new type error with an annotation of the query
// that provided the wrong type.
func NewTypeErrorFrom(from string, value any, exp ...Type) *TypeError {
	valueStr := ""
	valueType := ITypeOf(value)
	switch valueType {
	case TString:
		valueStr = fmt.Sprintf(`"%v"`, value)
	case TBool, TNumber:
		valueStr = fmt.Sprintf("%v", value)
	}
	return &TypeError{
		From:     from,
		Expected: exp,
		Actual:   valueType,
		Value:    valueStr,
	}
}
