package semantics

import (
	"testing"
)

// Story 3, anonymous naming: an anonymous function expression whose
// identifier-bound parameter is mutated keeps "<func lit>" as the Name's
// function half and takes the variable it is bound to only as a prefix.
func TestTSMutatesInput_AnonymousFunctionExpressionKeepsAnonymousOwnerName(t *testing.T) {
	source := `const f = function (p) {
	p.x = 1;
};
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	if want := "f/<func lit>:p"; got.Name != want {
		t.Errorf("Finding.Name = %q, want %q", got.Name, want)
	}
}

// Story 3, anonymous naming (arrow variant): an arrow function assigned to
// a variable keeps "<func lit>" too, since arrow functions never have a
// syntactic name field of their own.
func TestTSMutatesInput_AnonymousArrowFunctionKeepsAnonymousOwnerName(t *testing.T) {
	source := `const f = (p) => {
	p.x = 1;
};
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	if want := "f/<func lit>:p"; got.Name != want {
		t.Errorf("Finding.Name = %q, want %q", got.Name, want)
	}
}

// An anonymous function's Finding.Name is part of a signal's lifecycle
// identity, so inserting lines above it must not change the name.
func TestTSMutatesInput_AnonymousFunctionNameIsStableUnderPrefixInsertion(t *testing.T) {
	body := `rows.forEach((row) => {
	row.seen = true;
});
`
	names := make([]string, 0, 2)
	for _, source := range []string{body, "// one\n// two\n// three\n" + body} {
		root, closeTree := mustParseTS(t, []byte(source))
		_, findings := computeTSFeatures(root, []byte(source))
		names = append(names, mustFindTSMutatesInput(t, source, findings).Name)
		closeTree()
	}

	if names[0] != names[1] {
		t.Errorf("Finding.Name before insertion = %q, after = %q, want identical", names[0], names[1])
	}
}

// An anonymous function is named "<context>/<func lit>" after the nearest
// function-like node, itself included, that has a name (its own, or the
// variable it is assigned to), and plain "<func lit>" when none is reachable.
// Anonymous functions that are not assigned to anything are skipped.
func TestTSMutatesInput_AnonymousFunctionNameUsesNearestNamedFunction(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantName     string
		wantEvidence string
	}{
		{
			name: "top-level callback has no enclosing function",
			source: `rows.forEach((row) => {
	row.seen = true;
});
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "callback in a named function declaration",
			source: `function markSeen(rows) {
	rows.forEach((row) => {
		row.seen = true;
	});
}
`,
			wantName:     "markSeen/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "callback in an arrow function bound to a variable",
			source: `const markSeen = (rows) => {
	rows.forEach((row) => {
		row.seen = true;
	});
};
`,
			wantName:     "markSeen/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "callback in a class method",
			source: `class Rows {
	run(rows) {
		rows.forEach((row) => {
			row.seen = true;
		});
	}
}
`,
			wantName:     "run/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "callback nested in an unbound callback inside a named function",
			source: `function outer(groups) {
	groups.forEach((group) => {
		group.rows.forEach((row) => {
			row.seen = true;
		});
	});
}
`,
			wantName:     "outer/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "callback nested in an unbound callback with no named function above",
			source: `groups.forEach((group) => {
	group.rows.forEach((row) => {
		row.seen = true;
	});
});
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow function bound to a variable names its own parameter",
			source: `const mark = (row) => {
	row.seen = true;
};
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "function expression assigned to an existing variable",
			source: `let mark;
mark = function (row) {
	row.seen = true;
};
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow held by a class field is named after the field",
			source: `class Button {
	onClick = (event) => {
		event.handled = true;
	};
}
`,
			wantName:     "onClick/<func lit>:event",
			wantEvidence: "event.handled",
		},
		{
			name: "arrow held by a private class field is named after the field",
			source: `class Button {
	#onClick = (event) => {
		event.handled = true;
	};
}
`,
			wantName:     "#onClick/<func lit>:event",
			wantEvidence: "event.handled",
		},
		{
			name: "callback inside an arrow held by a class field",
			source: `class List {
	markAll = (rows) => {
		rows.forEach((row) => {
			row.seen = true;
		});
	};
}
`,
			wantName:     "markAll/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow held by an object property is named after the property",
			source: `const handlers = {
	mark: (row) => {
		row.seen = true;
	},
};
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow held by a quoted object property key has no reachable name",
			source: `const handlers = {
	"mark": (row) => {
		row.seen = true;
	},
};
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow that is a parameter default takes the enclosing function's name",
			source: `function configure(handler = (row) => {
	row.seen = true;
}) {
	return handler;
}
`,
			wantName:     "configure/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "parenthesized arrow bound to a variable",
			source: `const mark = ((row) => {
	row.seen = true;
});
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow bound to a variable through as",
			source: `const mark = ((row) => {
	row.seen = true;
}) as Handler;
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow bound to a variable through satisfies",
			source: `const mark = ((row) => {
	row.seen = true;
}) satisfies Handler;
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow bound to a variable through a non-null assertion",
			source: `const mark = ((row) => {
	row.seen = true;
})!;
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow assigned to a property of this is named after the property",
			source: `class Button {
	constructor() {
		this.onClick = (event) => {
			event.handled = true;
		};
	}
}
`,
			wantName:     "onClick/<func lit>:event",
			wantEvidence: "event.handled",
		},
		{
			name: "function expression assigned to a property of an object",
			source: `target.mark = function (row) {
	row.seen = true;
};
`,
			wantName:     "mark/<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow assigned to a private property of this",
			source: `class Button {
	constructor() {
		this.#onClick = (event) => {
			event.handled = true;
		};
	}
}
`,
			wantName:     "#onClick/<func lit>:event",
			wantEvidence: "event.handled",
		},
		{
			name: "arrow assigned to a computed member has no reachable name",
			source: `target[key] = (row) => {
	row.seen = true;
};
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "arrow passed to a call that is itself bound keeps the call-argument rule",
			source: `const handle = useCallback((row) => {
	row.seen = true;
});
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
		{
			name: "immediately invoked function has no reachable name",
			source: `(function (row) {
	row.seen = true;
})(first);
`,
			wantName:     "<func lit>:row",
			wantEvidence: "row.seen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			if mutatesInputFindingNamed(findings, tt.wantName, tt.wantEvidence) == nil {
				t.Fatalf("computeTSFeatures for %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.wantEvidence, findings)
			}
		})
	}
}

// Story 2/AC4: reassigning the parameter binding itself (`p = other`) is a
// rebind, not a write-through, and must not yield a mutates_input Finding.
func TestTSMutatesInput_ParameterRebindingIsNotFlagged(t *testing.T) {
	source := `function f(p) {
	p = other;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none (plain rebind is not a write-through)", source, f)
		}
	}
}

type tsMutatesInputSourceCase struct {
	name   string
	source string
}

func TestTSMutatesInput_ReboundParameterIsNotTrackedForLaterWrites(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
		{
			name: "assignment rebinding",
			source: `function f(p) {
	p = {};
	p.x = 1;
}
`,
		},
		{
			name: "for-of rebinding",
			source: `function f(p, items) {
	for (p of items) {
		p.x = 1;
	}
}
`,
		},
		{
			name: "augmented assignment rebinding",
			source: `function f(p) {
	p += other;
	p.x = 1;
}
`,
		},
		{
			name: "object destructuring rebinding",
			source: `function f(p, source) {
	({ p } = source);
	p.x = 1;
}
`,
		},
		{
			name: "array destructuring rebinding",
			source: `function f(p, source) {
	[p] = source;
	p.x = 1;
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoFindingAfterParameterRebind(t, tt)
		})
	}
}

func expectNoFindingAfterParameterRebind(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none after parameter rebinding", tt.source, f)
		}
	}
}

func TestTSMutatesInput_VarParameterRebindDoesNotSuppressEarlierMutation(t *testing.T) {
	source := `function f(p) {
	p.x = 1;
	var p = {};
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input before same-name var rebind, got %+v", source, findings)
	}
}

func TestTSMutatesInput_NoInitializerVarParameterDoesNotSuppressLaterMutation(t *testing.T) {
	source := `function f(p) {
	var p;
	p.x = 1;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input after no-initializer var parameter declaration, got %+v", source, findings)
	}
}
