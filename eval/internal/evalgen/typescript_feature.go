package evalgen

import "strings"

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

// featureCalls is the body of the hidden test, by entry kind.
var featureCalls = map[string]func(g tsSpec) string{
	"function":     func(g tsSpec) string { return g.callEntry(g.entryName() + "(") },
	"method":       func(g tsSpec) string { return g.callEntry("new Service()." + g.entryName() + "(") },
	"http_handler": tsSpec.handlerCall,
	"command":      tsSpec.commandCall,
}

func (g tsSpec) featureCall() string { return featureCalls[g.cell.Entry](g) }

func (g tsSpec) callEntry(call string) string {
	return "  assert.deepEqual(" + call + "\"" + recordID + "\"" + g.wantArgument() + "), want);\n"
}

func (g tsSpec) wantArgument() string {
	if g.cell.Direction == "write" {
		return ", want"
	}
	return ""
}

func (g tsSpec) handlerCall() string {
	body := "\"\""
	if g.cell.Direction == "write" {
		body = "JSON.stringify(want)"
	}
	return "  const sent: string[] = [];\n  const status = " + g.entryName() + "({ id: \"" + recordID + "\", body: " + body + " }, { send: (text) => sent.push(text) });\n  assert.equal(status, 200);\n  assert.deepEqual(JSON.parse(sent.join(\"\")), want);\n"
}

func (g tsSpec) commandCall() string {
	arguments := "\"" + recordID + "\""
	if g.cell.Direction == "write" {
		arguments += ", JSON.stringify(want)"
	}
	return "  const out: string[] = [];\n  const code = run([" + arguments + "], { send: (text) => out.push(text) });\n  assert.equal(code, 0);\n  assert.deepEqual(JSON.parse(out.join(\"\")), want);\n"
}
