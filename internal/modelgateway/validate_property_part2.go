package modelgateway

func checkTypedProperty(name string, raw any, types []string) error {
	if raw == nil {
		if hasType(types, "null") {
			return nil
		}
		return NewValidationError(name + " must not be null")
	}
	kind := jsonValueKind(raw)
	if kind == "" {
		return NewValidationError(name + " has unsupported JSON type")
	}
	if kind == "string" {
		if hasType(types, "string") {
			return nil
		}
		return NewValidationError(name + " must not be a string")
	}
	return NewValidationError(name + " must not be a " + kind)
}
func checkEnumProperty(name string, raw any, allowed []string) error {
	s, ok := raw.(string)
	if !ok {
		return NewValidationError(name + " must be a string enum value")
	}
	for _, v := range allowed {
		if s == v {
			return nil
		}
	}
	return NewValidationError(name + " value not in enum")
}
func requireProperties(obj map[string]any, required []string) error {
	for _, key := range required {
		if _, present := obj[key]; !present {
			return NewValidationError("missing required property: " + key)
		}
	}
	return nil
}
