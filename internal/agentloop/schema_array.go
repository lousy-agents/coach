package agentloop

import (
	"fmt"
)

func validateArrayItemsIfPresent(propPath string, raw any, items *argsSchemaDoc) error {
	if items == nil {
		return nil
	}
	return validateArrayItems(propPath, raw, items)
}

func validateArrayItems(path string, value any, itemSchema *argsSchemaDoc) error {
	if itemSchema == nil {
		return nil
	}
	arr, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: %s must be a JSON array", ErrInvalidArgs, rootLabel(path))
	}
	for i, item := range arr {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if err := validateSchemaItem(itemPath, item, itemSchema); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaItem(itemPath string, item any, itemSchema *argsSchemaDoc) error {
	if itemSchema.Type == "object" || itemSchema.Type == "" || len(itemSchema.Required) > 0 || len(itemSchema.Properties) > 0 {
		return validateAgainstSchema(itemPath, item, itemSchema)
	}
	return checkPropType(itemPath, item, []string{itemSchema.Type})
}
