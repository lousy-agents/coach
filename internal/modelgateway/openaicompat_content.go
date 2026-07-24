package modelgateway

import (
	"encoding/json"
	"strings"
)

func extractAndValidateJudgment(content string, schema json.RawMessage) (json.RawMessage, error) {
	raw, err := parseAssistantContentJSON(content)
	if err != nil {
		return nil, err
	}
	if err := validateJudgmentJSON(raw, schema); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseAssistantContentJSON(content string) (json.RawMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, NewValidationError("assistant content is empty")
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, NewValidationError("assistant content is not valid JSON")
	}
	return unwrapJSONStringOnce(raw)
}

// unwrapJSONStringOnce handles models that return a JSON string whose contents
// are the judgment object.
func unwrapJSONStringOnce(raw json.RawMessage) (json.RawMessage, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return raw, nil
	}
	asString = strings.TrimSpace(asString)
	if asString == "" {
		return nil, NewValidationError("assistant content is empty")
	}
	var inner json.RawMessage
	if err := json.Unmarshal([]byte(asString), &inner); err != nil {
		return nil, NewValidationError("assistant content is not valid JSON")
	}
	return inner, nil
}
