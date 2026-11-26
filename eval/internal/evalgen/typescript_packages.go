package evalgen

import "strings"

// entityModule is the data layer: the entity, a nested type when the field needs one, and the one function the
// direction needs.
func (g tsSpec) entityModule() []byte {
	var b strings.Builder
	if g.cell.Field == "nested" {
		b.WriteString("export interface " + g.shape.typeName + " {\n  readonly city: string;\n  readonly code: string;\n}\n\n")
	}
	b.WriteString("export interface " + g.entity + " {\n  readonly id: string;\n  readonly name: string;\n  readonly " + g.field + ": " + g.shape.typeName + ";\n}\n\n")
	record := "{ id, name: \"" + sampleName + "\", " + g.field + ": " + g.shape.sample + " }"
	if g.cell.Direction == "read" {
		b.WriteString("export function load(id: string): " + g.entity + " {\n  return " + record + ";\n}\n")
		return []byte(b.String())
	}
	b.WriteString("export function update(id: string, value: " + g.shape.typeName + "): " + g.entity + " {\n  const record: " + g.entity + " = " + record + ";\n  return { ...record, " + g.field + ": value };\n}\n")
	return []byte(b.String())
}

// passthroughModule forwards the one function the direction needs to the next package on the route.
func (g tsSpec) passthroughModule(index int) []byte {
	lower := g.route[index+1]
	var b strings.Builder
	b.WriteString("import * as lower from \"" + importFrom(g.route[index], lower) + "\";\n\n")
	b.WriteString("export type " + g.entity + " = lower." + g.entity + ";\n")
	if g.cell.Field == "nested" {
		b.WriteString("export type " + g.shape.typeName + " = lower." + g.shape.typeName + ";\n")
	}
	if g.cell.Direction == "read" {
		b.WriteString("\nexport function load(id: string): " + g.entity + " {\n  return lower.load(id);\n}\n")
	} else {
		b.WriteString("\nexport function update(id: string, value: " + g.shape.typeName + "): " + g.entity + " {\n  return lower.update(id, value);\n}\n")
	}
	return []byte(b.String())
}

func extraModule(layer, name string) []byte {
	return []byte("export function describe(): string {\n  return \"" + layer + "/" + name + "\";\n}\n")
}

// packageTest is the visible test of one package; it calls only exported functions.
func (g tsSpec) packageTest(layer, name string, onRoute bool, index int) []byte {
	var b strings.Builder
	b.WriteString("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\n")
	if !onRoute || index == 0 {
		b.WriteString("import { describe } from \"./" + name + ".ts\";\n\n")
		b.WriteString("test(\"describe names the package\", () => {\n  assert.equal(describe(), \"" + layer + "/" + name + "\");\n});\n")
		return []byte(b.String())
	}
	if g.cell.Direction == "read" {
		b.WriteString("import { load } from \"./" + name + ".ts\";\n\n")
		b.WriteString("test(\"load returns the sample record\", () => {\n  assert.deepEqual(load(\"" + recordID + "\"), " + g.recordLiteral(g.shape.sample) + ");\n});\n")
		return []byte(b.String())
	}
	b.WriteString("import { update } from \"./" + name + ".ts\";\n\n")
	b.WriteString("test(\"update replaces only the field\", () => {\n  assert.deepEqual(update(\"" + recordID + "\", " + g.shape.value + "), " + g.recordLiteral(g.shape.value) + ");\n});\n")
	return []byte(b.String())
}

func (g tsSpec) recordLiteral(value string) string {
	return "{ id: \"" + recordID + "\", name: \"" + sampleName + "\", " + g.field + ": " + value + " }"
}
