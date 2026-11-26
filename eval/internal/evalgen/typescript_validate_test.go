package evalgen_test

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

var validationCell = evalgen.Cell{Layers: 3, Packages: 2, Entry: "function", Depth: 2, Field: "scalar", Direction: "read", Trap: "layering"}

func program(t *testing.T) evalgen.Program {
	t.Helper()
	generated, err := evalgen.GenerateTypeScript(validationCell, 5)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}

func tools(t *testing.T) evalgen.TypeScriptTools {
	t.Helper()
	t.Setenv("EVAL_TSC", "")
	resolved, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))
	if err != nil {
		t.Fatalf("%v (run `mise run js-install` first)", err)
	}
	return resolved
}

func firstTestFile(files map[string][]byte) string {
	for path := range files {
		if strings.HasSuffix(path, ".test.ts") {
			return path
		}
	}
	return ""
}

func TestValidateTypeScriptAcceptsAGeneratedProgram(t *testing.T) {
	if err := evalgen.ValidateTypeScript(program(t), tools(t)); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTypeScriptRefusesWhatEachStepMustCatch(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*evalgen.Program)
		want   string
	}{
		{"a stub that does not type-check", func(p *evalgen.Program) {
			path := firstSource(p.Tree)
			p.Tree[path] = append(p.Tree[path], []byte("\nexport const broken: number = \"x\";\n")...)
		}, "tsc"},
		{"a visible test that fails on the stub", func(p *evalgen.Program) {
			path := firstTestFile(p.Tree)
			p.Tree[path] = []byte("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\ntest(\"fails\", () => { assert.equal(1, 2); });\n")
		}, "node --test"},
		{"a hidden test that passes on the stub", func(p *evalgen.Program) {
			for path := range p.Tests {
				p.Tests[path] = []byte("import { test } from \"node:test\";\ntest(\"passes\", () => {});\n")
			}
		}, "must fail"},
		{"an oracle that does not pass the hidden tests", func(p *evalgen.Program) {
			for path := range p.Solution {
				p.Solution[path] = p.Tree[path]
			}
		}, "clean oracle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			generated := program(t)
			c.break_(&generated)

			err := evalgen.ValidateTypeScript(generated, tools(t))

			expectRefusal(t, err, c.want)
		})
	}
}

func firstSource(files map[string][]byte) string {
	for path := range files {
		if strings.HasPrefix(path, "src/") && strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".test.ts") {
			return path
		}
	}
	return ""
}

// Each visible test file must run in its own process: a file that leaves state behind must not reach the next one.
func TestVisibleTestsRunInTheirOwnProcesses(t *testing.T) {
	generated := program(t)
	generated.Tree["src/zz_first.test.ts"] = []byte("import { test } from \"node:test\";\ntest(\"leaves state\", () => { (globalThis as { leaked?: number }).leaked = 1; });\n")
	generated.Tree["src/zz_second.test.ts"] = []byte("import assert from \"node:assert/strict\";\nimport { test } from \"node:test\";\ntest(\"sees none\", () => { assert.equal((globalThis as { leaked?: number }).leaked, undefined); });\n")

	if err := evalgen.ValidateTypeScript(generated, tools(t)); err != nil {
		t.Fatal(err)
	}
}
