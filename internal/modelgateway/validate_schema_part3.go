package modelgateway

import (
	"encoding/json"
)

func parseSchemaDoc(schema json.RawMessage) (schemaDoc, error) {
	var sch schemaDoc
	if err := json.Unmarshal(schema, &sch); err != nil {
		return schemaDoc{}, NewValidationError("output schema is not valid JSON")
	}
	if sch.Type != "" && sch.Type != "object" {
		return schemaDoc{}, NewValidationError("output schema type must be object")
	}
	return sch, nil
}
