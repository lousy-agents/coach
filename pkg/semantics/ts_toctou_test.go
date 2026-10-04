package semantics

import (
	"testing"
)

// Story 1 (CWE-367, GitHub issue #177), positive case: a bare
// existsSync(p) guard whose consequence block calls a matching fs act
// call on identical path text must yield exactly one
// toctou_check_then_act Finding, located at the act call (not the check
// call), with the documented Confidence/SuggestedSkill fields. Table
// covers every name in tsToctouActCallNames so deleting any one of them
// from that set fails a case here.
func TestComputeTSFeatures_TOCTOUCheckThenAct_PositiveFinding(t *testing.T) {
	tests := []struct {
		name        string
		actCallText string
	}{
		{name: "readFileSync", actCallText: `readFileSync(p, "utf8")`},
		{name: "writeFileSync", actCallText: `writeFileSync(p, "seed")`},
		{name: "appendFileSync", actCallText: `appendFileSync(p, "more")`},
		{name: "unlinkSync", actCallText: `unlinkSync(p)`},
		{name: "rmSync", actCallText: `rmSync(p)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsToctouTest_27(t, tt)
		})
	}
}

// Story 1, exclusion cases: none of these guarded-body shapes match the
// narrow "bare existsSync(path) call gates a matching fs act call on the
// identical path text" pattern, so each must yield zero
// toctou_check_then_act findings.
func TestComputeTSFeatures_TOCTOUCheckThenAct_ExcludedCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "path text differs between check and act",
			source: `function f(a: string, b: string) {
	if (existsSync(a)) {
		readFileSync(b);
	}
}
`,
		},
		{
			name: "no act call anywhere in the guarded body",
			source: `function f(p: string) {
	if (existsSync(p)) {
		doSomethingElse();
	}
}
`,
		},
		{
			name: "condition wrapped in &&",
			source: `function f(p: string, other: boolean) {
	if (existsSync(p) && other) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "existsSync appears only inside a ternary expression condition, not an if/while",
			source: `function f(p: string) {
	const x = existsSync(p) ? readFileSync(p) : undefined;
	return x;
}
`,
		},
		{
			name: "existsSync gates a for loop condition, not an if/while",
			source: `function f(p: string) {
	for (; existsSync(p); ) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "negated check gates the does-not-exist branch, not the exists branch",
			source: `function f(p: string) {
	if (!existsSync(p)) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "act call sits in the else branch, which is not gated by the check",
			source: `function f(p: string) {
	if (existsSync(p)) {
		log(p);
	} else {
		writeFileSync(p, "seed");
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsToctouTest_137(t, tt)
		})
	}
}
