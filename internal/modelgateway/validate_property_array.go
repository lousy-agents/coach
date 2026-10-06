package modelgateway

import (
	"fmt"
)

func checkArrayProperty(name string, raw any, prop propSchema) error {
	if raw == nil {
		return NewValidationError(name + " must not be null")
	}
	arr, ok := raw.([]any)
	if !ok {
		return NewValidationError(name + " must be an array")
	}
	if prop.Items == nil {
		return NewValidationError(name + " array schema requires object items")
	}
	for i, elem := range arr {
		elemPath := fmt.Sprintf("%s[%d]", name, i)
		obj, ok := elem.(map[string]any)
		if !ok {
			return NewValidationError(elemPath + " must be an object")
		}
		if err := requireProperties(obj, prop.Items.Required); err != nil {
			return NewValidationError(elemPath + ": " + validationDetail(err))
		}
		if err := checkPresentProperties(obj, prop.Items.Properties); err != nil {
			return NewValidationError(elemPath + ": " + validationDetail(err))
		}
	}
	return nil
}

func validationDetail(err error) string {
	if ve, ok := err.(*ValidationError); ok && ve != nil && ve.Detail != "" {
		return ve.Detail
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
