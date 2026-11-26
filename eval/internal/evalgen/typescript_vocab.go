package evalgen

import "strings"

// TypeScriptGeneratorVersion changes whenever the emitted TypeScript trees can change for the same cell and seed.
const TypeScriptGeneratorVersion = "ts-1"

// tsShape holds the TypeScript spelling of a field's type and of two non-zero values and a zero value for it.
type tsShape struct {
	typeName, sample, value, zero, jsonValue string
}

func tsShapeOf(kind, field string) tsShape {
	switch kind {
	case "collection":
		return tsShape{"readonly string[]", `["red", "green"]`, `["blue", "gold"]`, "[]", `["blue","gold"]`}
	case "nested":
		return tsShape{field, `{ city: "Lyon", code: "69001" }`, `{ city: "Nice", code: "06000" }`, `{ city: "", code: "" }`, `{"city":"Nice","code":"06000"}`}
	}
	return tsShape{"string", `"alpha"`, `"beta"`, `""`, `"beta"`}
}

func lowerFirst(text string) string { return strings.ToLower(text[:1]) + text[1:] }
