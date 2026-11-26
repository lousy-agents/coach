package antipatternfixtures

import (
	"encoding/json"
	"fmt"
)

// The profile versions whose measures the templates are written against. A
// profile of another version may measure a shape differently, so it is
// refused rather than read.
const (
	supportedSchemaVersion    = "1"
	supportedExtractorVersion = "0.2.0"
)

// Validator checks a profile against the committed profile schema.
type Validator func(raw []byte) error

// profile is the part of a validated profile the templates read.
type profile struct {
	SchemaVersion    string `json:"schema_version"`
	Identity         string `json:"identity"`
	ExtractorVersion string `json:"extractor_version"`
	Shapes           struct {
		Nesting       map[string]int `json:"nesting"`
		Cognitive     map[string]int `json:"cognitive"`
		BranchDensity map[string]int `json:"branch_density"`
		Constructor   struct {
			DoesWork int `json:"does_work"`
		} `json:"constructor"`
	} `json:"shapes"`
}

func readProfile(raw []byte, validate Validator) (profile, error) {
	if err := validate(raw); err != nil {
		return profile{}, refuse(InputInvalid, fmt.Sprintf("the profile does not validate: %v", err))
	}
	var read profile
	if err := json.Unmarshal(raw, &read); err != nil {
		return profile{}, refuse(InputInvalid, fmt.Sprintf("the profile cannot be read: %v", err))
	}
	switch {
	case read.SchemaVersion != supportedSchemaVersion:
		return profile{}, refuse(InputInvalid, fmt.Sprintf("schema_version %q is not %q", read.SchemaVersion, supportedSchemaVersion))
	case read.ExtractorVersion != supportedExtractorVersion:
		return profile{}, refuse(InputInvalid, fmt.Sprintf("extractor_version %q is not %q, which the templates are written against", read.ExtractorVersion, supportedExtractorVersion))
	}
	return read, nil
}
