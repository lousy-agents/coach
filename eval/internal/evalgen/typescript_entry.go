package evalgen

import "strings"

func (g tsSpec) entryName() string {
	if g.cell.Entry == "command" {
		return "run"
	}
	prefix := "get"
	if g.cell.Direction == "write" {
		prefix = "set"
	}
	return prefix + strings.ToUpper(g.field[:1]) + g.field[1:]
}

// entrySource is the entry module: its stub returns the zero value, and the clean oracle forwards to the next
// package on the route. Both declare describe so the visible tests do not depend on the entry.
func (g tsSpec) entrySource(solution bool) []byte {
	entry := g.route[0]
	var b strings.Builder
	if solution {
		b.WriteString("import * as lower from \"" + importFrom(entry, g.route[1]) + "\";\n")
	} else if g.stubNamesLowerType() {
		b.WriteString("import type { " + g.shape.typeName + " } from \"" + importFrom(entry, g.route[1]) + "\";\n")
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString("export function describe(): string {\n  return \"" + entry.layer + "/" + entry.name + "\";\n}\n\n")
	b.WriteString(g.entryDeclaration(solution))
	return []byte(b.String())
}

// stubNamesLowerType is true when the stub's signature or body names a type the next package owns, which only a
// returning function or method does.
func (g tsSpec) stubNamesLowerType() bool {
	return g.cell.Field == "nested" && (g.cell.Entry == "function" || g.cell.Entry == "method")
}

// fieldType is the field's type as the entry module spells it: the oracle reaches the nested type through lower.
func (g tsSpec) fieldType(solution bool) string {
	if solution && g.cell.Field == "nested" {
		return "lower." + g.shape.typeName
	}
	return g.shape.typeName
}

func (g tsSpec) entryDeclaration(solution bool) string {
	read := "lower.load(id)." + g.field
	arguments := "id: string"
	if g.cell.Direction == "write" {
		read = "lower.update(id, value)." + g.field
		arguments = "id: string, value: " + g.fieldType(solution)
	}
	switch g.cell.Entry {
	case "function":
		return "export function " + g.entryName() + "(" + arguments + "): " + g.fieldType(solution) + " {\n  return " + g.entryResult(solution, read) + ";\n}\n"
	case "method":
		return "export class Service {\n  " + g.entryName() + "(" + arguments + "): " + g.fieldType(solution) + " {\n    return " + g.entryResult(solution, read) + ";\n  }\n}\n"
	case "http_handler":
		return g.handlerDeclaration(solution)
	}
	return g.commandDeclaration(solution)
}

func (g tsSpec) entryResult(solution bool, read string) string {
	if solution {
		return read
	}
	return g.shape.zero
}

const outputInterface = "export interface Output {\n  readonly send: (text: string) => unknown;\n}\n\n"

func (g tsSpec) handlerDeclaration(solution bool) string {
	declaration := "export interface Request {\n  readonly id: string;\n  readonly body: string;\n}\n\n" + outputInterface
	body := "return 0;"
	if solution {
		call := "lower.load(request.id)." + g.field
		if g.cell.Direction == "write" {
			call = "lower.update(request.id, JSON.parse(request.body) as " + g.fieldType(true) + ")." + g.field
		}
		body = "response.send(JSON.stringify(" + call + "));\n  return 200;"
	}
	return declaration + "export function " + g.entryName() + "(request: Request, response: Output): number {\n  " + body + "\n}\n"
}

func (g tsSpec) commandDeclaration(solution bool) string {
	body := "return 0;"
	if solution {
		call := "lower.load(args[0])." + g.field
		if g.cell.Direction == "write" {
			call = "lower.update(args[0], JSON.parse(args[1]) as " + g.fieldType(true) + ")." + g.field
		}
		body = "out.send(JSON.stringify(" + call + "));\n  return 0;"
	}
	return outputInterface + "export function run(args: readonly string[], out: Output): number {\n  " + body + "\n}\n"
}

// featureTest is the hidden FAIL_TO_PASS test. It lives outside the tsconfig include, so the stub still compiles.
func (g tsSpec) featureTest() []byte {
	want := g.shape.sample
	if g.cell.Direction == "write" {
		want = g.shape.value
	}
	var b strings.Builder
	b.WriteString("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\n")
	symbol := g.entryName()
	if g.cell.Entry == "method" {
		symbol = "Service"
	}
	b.WriteString("import { " + symbol + " } from \"../../" + g.path(g.route[0]) + "\";\n\n")
	b.WriteString("test(\"the entry exposes the field\", () => {\n  const want = " + want + ";\n" + g.featureCall() + "});\n")
	return []byte(b.String())
}

func (g tsSpec) featureCall() string {
	argument := ""
	if g.cell.Direction == "write" {
		argument = ", want"
	}
	switch g.cell.Entry {
	case "function":
		return "  assert.deepEqual(" + g.entryName() + "(\"" + recordID + "\"" + argument + "), want);\n"
	case "method":
		return "  assert.deepEqual(new Service()." + g.entryName() + "(\"" + recordID + "\"" + argument + "), want);\n"
	case "http_handler":
		body := "\"\""
		if g.cell.Direction == "write" {
			body = "JSON.stringify(want)"
		}
		return "  const sent: string[] = [];\n  const status = " + g.entryName() + "({ id: \"" + recordID + "\", body: " + body + " }, { send: (text) => sent.push(text) });\n  assert.equal(status, 200);\n  assert.deepEqual(JSON.parse(sent.join(\"\")), want);\n"
	}
	arguments := "\"" + recordID + "\""
	if g.cell.Direction == "write" {
		arguments += ", JSON.stringify(want)"
	}
	return "  const out: string[] = [];\n  const code = run([" + arguments + "], { send: (text) => out.push(text) });\n  assert.equal(code, 0);\n  assert.deepEqual(JSON.parse(out.join(\"\")), want);\n"
}
