package modelgateway

func requireProperties(obj map[string]any, required []string) error {
	for _, key := range required {
		if _, present := obj[key]; !present {
			return NewValidationError("missing required property: " + key)
		}
	}
	return nil
}

func checkPresentProperties(obj map[string]any, properties map[string]propSchema) error {
	for name, prop := range properties {
		raw, present := obj[name]
		if !present {
			continue
		}
		if err := checkProperty(name, raw, prop); err != nil {
			return err
		}
	}
	return nil
}

func checkProperty(name string, raw any, prop propSchema) error {
	if hasType(prop.types, "array") {
		return checkArrayProperty(name, raw, prop)
	}
	if len(prop.Enum) > 0 {
		return checkEnumProperty(name, raw, prop.Enum)
	}
	if len(prop.types) == 0 {
		return nil
	}
	return checkTypedProperty(name, raw, prop.types)
}
