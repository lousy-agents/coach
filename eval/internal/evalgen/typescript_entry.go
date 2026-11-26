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

// entrySymbol is the entry as a caller names it: a method belongs to its class.
func (g tsSpec) entrySymbol() string {
	if g.cell.Entry == "method" {
		return "Service." + g.entryName()
	}
	return g.entryName()
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
