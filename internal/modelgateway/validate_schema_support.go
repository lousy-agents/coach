package modelgateway

import (
	"strings"
)

func ensureSupportedProperties(sch schemaDoc) error {
	for name, prop := range sch.Properties {
		if err := ensureSupportedPropSchema(name, prop); err != nil {
			return err
		}
	}
	return nil
}

func ensureSupportedPropSchema(name string, prop propSchema) error {
	for _, t := range prop.types {
		switch strings.ToLower(t) {
		case "string", "null":
			// leaf types used by singular seed schemas
		case "array":
			if err := ensureSupportedArrayProp(name, prop); err != nil {
				return err
			}
		default:
			return NewValidationError(name + " has unsupported schema type: " + t)
		}
	}
	return nil
}

func unsupportedLeafType(types []string) (string, bool) {
	for _, t := range types {
		if !supportedLeafPropType(t) {
			return t, true
		}
	}
	return "", false
}

func supportedLeafPropType(t string) bool {
	switch strings.ToLower(t) {
	case "string", "null":
		return true
	default:
		return false
	}
}
