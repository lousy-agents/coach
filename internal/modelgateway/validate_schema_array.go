package modelgateway

import (
	"strings"
)

// ensureSupportedArrayProp allows only array-of-object envelopes whose element
// properties are the same string|null leaf subset as singular judgments.
// Nested arrays, array-of-string, and non-object items fail closed.
func ensureSupportedArrayProp(name string, prop propSchema) error {
	if prop.Items == nil {
		return NewValidationError(name + " array schema requires object items")
	}
	if prop.Items.Type != "" && !strings.EqualFold(prop.Items.Type, "object") {
		return NewValidationError(name + " array items must be object")
	}
	for elemName, elemProp := range prop.Items.Properties {
		if t, bad := unsupportedLeafType(elemProp.types); bad {
			return NewValidationError(name + " items." + elemName + " has unsupported schema type: " + t)
		}
		if elemProp.Items != nil {
			return NewValidationError(name + " items." + elemName + " has unsupported nested array")
		}
	}
	return nil
}
