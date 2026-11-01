package semantics

import (
	"reflect"
	"sort"
	"testing"
)

func tsMutatesInputEvidence(source string, findings []Finding) []string {
	evidence := []string{}
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			evidence = append(evidence, source[f.Location.StartByte:f.Location.EndByte])
		}
	}
	sort.Strings(evidence)
	return evidence
}

// The accumulator of a reduce or reduceRight callback is created by the call
// when its initial value is an empty literal or a constructor call with no
// arguments, so mutating it is not a mutation of caller-owned data.
func TestTSMutatesInput_FreshReduceAccumulator(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantEvidence []string
	}{
		{
			name: "empty object literal",
			source: `items.reduce((acc, item) => {
	acc[item.name] = item.value;
	return acc;
}, {});
`,
			wantEvidence: []string{},
		},
		{
			name: "empty object literal with a type assertion",
			source: `items.reduce((acc, item) => {
	acc[item.name] = item.value;
	return acc;
}, {} as Record<string, string>);
`,
			wantEvidence: []string{},
		},
		{
			name: "empty object literal with satisfies",
			source: `items.reduce((acc, item) => {
	acc[item.name] = item.value;
	return acc;
}, {} satisfies Record<string, string>);
`,
			wantEvidence: []string{},
		},
		{
			name: "parenthesized empty array literal",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	return acc;
}, ([] as string[]));
`,
			wantEvidence: []string{},
		},
		{
			name: "constructor call without arguments",
			source: `items.reduce((acc, item) => {
	acc.set(item.name, item);
	return acc;
}, new Map());
`,
			wantEvidence: []string{},
		},
		{
			name: "constructor without an argument list",
			source: `items.reduce((acc, item) => {
	acc.add(item);
	return acc;
}, new Set);
`,
			wantEvidence: []string{},
		},
		{
			name: "reduceRight",
			source: `items.reduceRight((acc, item) => {
	acc[item.name] = item;
	return acc;
}, {});
`,
			wantEvidence: []string{},
		},
		{
			name: "optional call on the array",
			source: `items?.reduce((acc, item) => {
	acc[item.name] = item;
	return acc;
}, {});
`,
			wantEvidence: []string{},
		},
		{
			name: "function expression callback with a typed accumulator",
			source: `items.reduce(function (acc: Record<string, string>, item) {
	acc[item.name] = item.value;
	return acc;
}, {});
`,
			wantEvidence: []string{},
		},
		{
			name: "bare single-parameter arrow",
			source: `items.reduce(acc => {
	acc.total = 1;
	return acc;
}, {});
`,
			wantEvidence: []string{},
		},
		{
			name: "the accumulator shadows an outer parameter of the same name",
			source: `function outer(acc) {
	return items.reduce((acc, item) => {
		acc[item.name] = item;
		return acc;
	}, {});
}
`,
			wantEvidence: []string{},
		},
		{
			name: "a non-empty object literal can hold caller-owned data",
			source: `items.reduce((acc, item) => {
	acc.items.push(item);
	return acc;
}, { items });
`,
			wantEvidence: []string{"acc.items.push"},
		},
		{
			name: "a non-empty array literal",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	return acc;
}, [first]);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a constructor call with arguments",
			source: `items.reduce((acc, item) => {
	acc.add(item);
	return acc;
}, new Set(items));
`,
			wantEvidence: []string{"acc.add"},
		},
		{
			name: "an initial value taken from a caller-owned variable",
			source: `items.reduce((acc, item) => {
	acc.total = item.value;
	return acc;
}, totals);
`,
			wantEvidence: []string{"acc.total"},
		},
		{
			name: "no initial value, so the accumulator starts as the first element",
			source: `items.reduce((acc, item) => {
	acc.total = item.value;
	return acc;
});
`,
			wantEvidence: []string{"acc.total"},
		},
		{
			name: "the callback is not the first argument",
			source: `reduce(items, (acc, item) => {
	acc.total = item.value;
	return acc;
}, {});
`,
			wantEvidence: []string{"acc.total"},
		},
		{
			name: "an empty literal that precedes a callback in third position",
			source: `reduce(items, {}, (acc, item) => {
	acc.total = item.value;
	return acc;
});
`,
			wantEvidence: []string{"acc.total"},
		},
		{
			name: "a method other than reduce",
			source: `items.map((acc, item) => {
	acc.total = item.value;
	return acc;
}, {});
`,
			wantEvidence: []string{"acc.total"},
		},
		{
			name: "the item parameter is still reported",
			source: `items.reduce((acc, item) => {
	item.seen = true;
	return acc;
}, {});
`,
			wantEvidence: []string{"item.seen"},
		},
		{
			name: "the source array parameter is still reported",
			source: `items.reduce((acc, item, index, source) => {
	source.length = 0;
	return acc;
}, []);
`,
			wantEvidence: []string{"source.length"},
		},
		{
			name: "a callback that returns something else makes the accumulator caller-owned from the second call",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	return item.names;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a callback that returns the accumulator on every path",
			source: `items.reduce((acc, item) => {
	if (!item.name) {
		return acc;
	}
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "one path returns something else",
			source: `items.reduce((acc, item) => {
	if (!item.name) {
		return other;
	}
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a bare return on one path",
			source: `items.reduce((acc, item) => {
	if (!item.name) {
		return;
	}
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a callback with no return statement",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "an expression body that returns the accumulator through a comma expression",
			source: `items.reduce((acc, item) => (acc.push(item.name), acc), []);
`,
			wantEvidence: []string{},
		},
		{
			name: "an expression body that returns another value",
			source: `items.reduce((acc, item) => (acc.push(item.name), item.names), []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a return inside a nested function does not count",
			source: `items.reduce((acc, item) => {
	item.names.forEach(() => {
		return item;
	});
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "the accumulator returned through a type assertion",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	return acc as string[];
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "the callback reassigns the accumulator to caller-owned data",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	acc = item.names;
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a conditional reset of the accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	if (item.reset) {
		acc = item.names;
	}
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "destructuring reassigns the accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	[acc] = [item.names];
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "the accumulator is reassigned inside a returned comma expression",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	return (acc = item.names, acc);
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "the accumulator is reassigned inside the right-hand side of another assignment",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	const copy = (acc = item.names);
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "the accumulator is reassigned inside an assignment's right-hand side",
			source: `let copy;
items.reduce((acc, item) => {
	acc.push(item.name);
	copy = (acc = item.names);
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a for-of target reassigns the accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	for (acc of item.lists) {}
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a var declaration with an initializer reassigns the accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item.name);
	var acc = item.names;
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a nested function's own parameter of the same name is a different binding",
			source: `items.reduce((acc, item) => {
	item.names.forEach((acc) => {
		acc = 1;
	});
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "a block-scoped let of the same name is a different binding",
			source: `items.reduce((acc, item) => {
	{
		let acc = item.names;
	}
	acc.push(item.name);
	return acc;
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "the reviewer's reassignment to caller input before the return",
			source: `items.reduce((acc, x) => {
	acc[x.k] = x;
	acc = input;
	return acc;
}, {});
`,
			wantEvidence: []string{"acc[x.k]"},
		},
		{
			name: "the reviewer's var redeclaration to caller input before the return",
			source: `items.reduce(function (acc, x) {
	acc[x.k] = x;
	var acc = input;
	return acc;
}, {});
`,
			wantEvidence: []string{"acc[x.k]"},
		},
		{
			name: "a closure inside the callback reassigns the outer accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	const reset = () => {
		acc = shared;
	};
	reset();
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a nested function declaration reassigns the outer accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	function reset() {
		acc = shared;
	}
	reset();
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "an assertion around the reassigned accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	(acc as any) = shared;
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "an angle-bracket assertion around the reassigned accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	(<any>acc) = shared;
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "an angle-bracket assertion around the returned accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	return <string[]>acc;
}, []);
`,
			wantEvidence: []string{},
		},
		{
			name: "a non-null assertion around the reassigned accumulator",
			source: `items.reduce((acc, item) => {
	acc.push(item);
	acc! = shared;
	return acc;
}, []);
`,
			wantEvidence: []string{"acc.push"},
		},
		{
			name: "a bare function named reduce is not a method call",
			source: `reduce((acc, item) => {
	acc.y = 1;
	return acc;
}, {});
`,
			wantEvidence: []string{"acc.y"},
		},
		{
			name: "a named function expression callback may call itself with other data",
			source: `items.reduce(function g(acc, x) {
	acc.y = 1;
	if (x) {
		g(input, x);
	}
	return acc;
}, {});
`,
			wantEvidence: []string{"acc.y"},
		},
		{
			name: "a receiver that is not an array is matched by method name",
			source: `store.reduce((state, event) => {
	state.last = event;
	return state;
}, {});
`,
			wantEvidence: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			got := tsMutatesInputEvidence(tt.source, findings)
			if !reflect.DeepEqual(got, tt.wantEvidence) {
				t.Errorf("mutates_input evidence for %q = %v, want %v", tt.source, got, tt.wantEvidence)
			}
		})
	}
}
