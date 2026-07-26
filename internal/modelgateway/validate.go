package modelgateway

import "encoding/json"

// validateOutputSchema checks OutputSchema shape and supported property types
// without a judgment value. Callers use this to fail closed on static schema
// problems before any upstream inference call.
func validateOutputSchema(schema json.RawMessage) error {
	if len(schema) == 0 {
		return nil
	}
	sch, err := parseSchemaDoc(schema)
	if err != nil {
		return err
	}
	return ensureSupportedProperties(sch)
}

// validateJudgmentJSON ensures judgment is a JSON object. When schema is
// non-empty, it enforces a minimal JSON Schema subset used by seed rubrics:
// object type, required properties, string enums, string|null types, and
// top-level array-of-object properties (batch envelope) whose element leaves
// use the same string|null/enum subset. Other JSON Schema types (integer,
// number, boolean, nested object, array-of-non-object) are rejected so
// incomplete checks cannot silently accept invalid values.
func validateJudgmentJSON(judgment, schema json.RawMessage) error {
	obj, err := parseJudgmentObject(judgment)
	if err != nil {
		return err
	}
	if len(schema) == 0 {
		return nil
	}
	sch, err := parseSchemaDoc(schema)
	if err != nil {
		return err
	}
	if err := ensureSupportedProperties(sch); err != nil {
		return err
	}
	if err := requireProperties(obj, sch.Required); err != nil {
		return err
	}
	return checkPresentProperties(obj, sch.Properties)
}
