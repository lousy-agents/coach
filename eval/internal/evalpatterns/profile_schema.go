package evalpatterns

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed profile.schema.json
var profileSchemaJSON []byte

const profileSchemaName = "profile.schema.json"

var compiledProfileSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(profileSchemaJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(profileSchemaName, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(profileSchemaName)
})

// ValidateProfile checks raw against the committed profile schema, which is
// closed: the label and the licence are its only free text, so a profile
// that carries a name, a path or source text fails.
func ValidateProfile(raw []byte) error {
	schema, err := compiledProfileSchema()
	if err != nil {
		return fmt.Errorf("compile profile schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("profile is not JSON: %w", err)
	}
	return schema.Validate(instance)
}
