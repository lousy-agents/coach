package main_test

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// expectMatchesSchema checks that document is one JSON value that validates against the committed
// schema file at schemaPath.
func expectMatchesSchema(schemaPath string, document []byte) {
	ExpectWithOffset(1, schemaValidationError(schemaPath, document)).To(Succeed(), "stdout must match %s: %s", schemaPath, document)
}

// schemaValidationError returns why document fails the committed schema at schemaPath, or nil when it
// validates. A missing schema or a document that is not one JSON value fails the spec itself.
func schemaValidationError(schemaPath string, document []byte) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	ExpectWithOffset(2, err).NotTo(HaveOccurred(), "the committed schema %s must exist", schemaPath)
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	ExpectWithOffset(2, err).NotTo(HaveOccurred())

	name := filepath.Base(schemaPath)
	compiler := jsonschema.NewCompiler()
	ExpectWithOffset(2, compiler.AddResource(name, schemaDoc)).To(Succeed())
	schema, err := compiler.Compile(name)
	ExpectWithOffset(2, err).NotTo(HaveOccurred())

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	ExpectWithOffset(2, err).NotTo(HaveOccurred(), "stdout must be one JSON document: %s", document)
	return schema.Validate(instance)
}

const (
	trendOutputSchemaPath   = "../../internal/evalharness/trend_output.schema.json"
	compareOutputSchemaPath = "../../internal/evalharness/compare_output.schema.json"
)
