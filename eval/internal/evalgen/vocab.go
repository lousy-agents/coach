package evalgen

import "strings"

var (
	domainNames  = []string{"shop", "clinic", "library", "depot", "studio"}
	entityNames  = []string{"Account", "Order", "Patient", "Book", "Invoice"}
	packageNames = []string{"catalog", "billing", "search", "audit"}
	layerNames   = map[int][]string{
		3: {"api", "service", "store"},
		4: {"api", "service", "domain", "store"},
		5: {"api", "service", "domain", "repository", "store"},
	}
	fieldNames = map[string][]string{
		"scalar":     {"Label", "Title", "Status"},
		"collection": {"Tags", "Labels", "Notes"},
		"nested":     {"Address", "Contact", "Location"},
	}
)

const (
	sampleName = "sample"
	recordID   = "id-1"
)

// fieldShape holds the Go spelling of a field's type and of two non-zero values for it. {Q} stands for the
// qualifier of a package that declares the nested type, and is empty where the type is local.
type fieldShape struct {
	typeName, sample, value, jsonValue string
}

func shapeOf(kind, fieldName string) fieldShape {
	switch kind {
	case "collection":
		return fieldShape{"[]string", `[]string{"red", "green"}`, `[]string{"blue", "gold"}`, `["blue","gold"]`}
	case "nested":
		return fieldShape{"{Q}" + fieldName, "{Q}" + fieldName + `{City: "Lyon", Code: "69001"}`, "{Q}" + fieldName + `{City: "Nice", Code: "06000"}`, `{"City":"Nice","Code":"06000"}`}
	}
	return fieldShape{"string", `"alpha"`, `"beta"`, `"beta"`}
}

func qualify(text, qualifier string) string { return strings.ReplaceAll(text, "{Q}", qualifier) }
