package render

import (
	"strings"
	"testing"
	"unicode"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestReadinessTextWritesNoControlCharacterWhicheverFieldHoldsOne(t *testing.T) {
	result := resultThatPrintsEveryField()

	text := ReadinessText(&result)

	for position, r := range text {
		if r != '\n' && unicode.IsControl(r) {
			start := max(0, position-40)
			t.Fatalf("ReadinessText wrote the control character %U to the terminal at byte %d, after %q", r, position, text[start:position])
		}
	}
	if !strings.Contains(text, `\x1b]0;PWNED\a`) {
		t.Errorf("the hostile value is not shown escaped anywhere in:\n%s", text)
	}
}

func TestReadinessTextShowsAHostileDeclaredVersionAndDirtyPathEscaped(t *testing.T) {
	var result projectreadiness.Result
	result.Checks.Compiler = projectreadiness.Check{State: projectreadiness.Fail, Code: "typescript_compiler_missing", DeclaredVersion: "7.0.2\x1b]0;T\x07"}
	result.DirtyWorktree = projectreadiness.DirtyWorktree{RelevantChanges: true, Paths: []string{"tsconfig\x1b[2J.json"}}

	text := ReadinessText(&result)

	for _, want := range []string{`declared_version="7.0.2\x1b]0;T\a"`, `"tsconfig\x1b[2J.json"`} {
		if !strings.Contains(text, want) {
			t.Errorf("ReadinessText lacks %s in:\n%s", want, text)
		}
	}
}

func TestReadinessTextLeavesAPrintableValueAsItWas(t *testing.T) {
	var result projectreadiness.Result
	result.Checks.Compiler = projectreadiness.Check{State: projectreadiness.Fail, Code: "typescript_compiler_missing", DeclaredVersion: "^7.0.2 || ~5.6"}
	result.DirtyWorktree = projectreadiness.DirtyWorktree{RelevantChanges: true, Paths: []string{"src/日本語 file.ts", "tsconfig.json"}}

	text := ReadinessText(&result)

	for _, want := range []string{"declared_version=^7.0.2 || ~5.6", "\n  src/日本語 file.ts\n", "\n  tsconfig.json\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("ReadinessText changed a printable value; lacks %q in:\n%s", want, text)
		}
	}
}
