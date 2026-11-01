package main

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const hiddenInputMutationRuleID = "state.hidden_input_mutation"

var _ = Describe("coach codesignal anonymous callback lifecycle identity", func() {
	DescribeTable("a pure line insertion above a hidden-input mutation leaves every signal existing",
		func(source string, wantSignals int) {
			repo := newTempGitRepo()
			sixLinesOfComments := strings.Repeat("// shifts every byte offset below\n", 6)
			initialSHA := commitFile(repo, "callbacks.ts", source)
			commitFile(repo, "callbacks.ts", sixLinesOfComments+source)

			report, _ := runCoachCodesignal(repo, initialSHA)

			var existing []codesignal.Signal
			var movedOrChanged []string
			for _, signal := range signalsForPath(report, "callbacks.ts") {
				if signal.RuleID != hiddenInputMutationRuleID {
					continue
				}
				if signal.Lifecycle == codesignal.Lifecycle("existing") {
					existing = append(existing, signal)
					continue
				}
				movedOrChanged = append(movedOrChanged, fmt.Sprintf("%s %s", signal.Lifecycle, signal.Subject))
			}
			Expect(movedOrChanged).To(BeEmpty(), "line movement alone must not introduce or resolve signals")
			Expect(existing).To(HaveLen(wantSignals), "the fixture must produce signals, or an empty report would pass trivially")
		},
		Entry("two anonymous callbacks with different evidence", `export function markSeen(rows: Row[]) {
  rows.forEach((row) => {
    row.seen = true;
  });
}

export function countEntries(entries: Entry[]) {
  entries.forEach((entry) => {
    entry.count += 1;
  });
}
`, 2),
		Entry("two anonymous callbacks with identical evidence", `export function markFirst(rows: Row[]) {
  rows.forEach((row) => {
    row.seen = true;
  });
}

export function markSecond(rows: Row[]) {
  rows.forEach((row) => {
    row.seen = true;
  });
}
`, 2),
		Entry("a named function as the control", `export function markNamed(row: Row) {
  row.seen = true;
}
`, 1),
	)

	DescribeTable("reports a mutation that moves into a different function as introduced plus resolved",
		func(base, head string) {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "moved.ts", base)
			commitFile(repo, "moved.ts", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			lifecycles := []codesignal.Lifecycle{}
			for _, signal := range signalsForPath(report, "moved.ts") {
				if signal.RuleID == hiddenInputMutationRuleID {
					lifecycles = append(lifecycles, signal.Lifecycle)
				}
			}
			Expect(lifecycles).To(ConsistOf(codesignal.Lifecycle("introduced"), codesignal.Lifecycle("resolved")),
				"the mutation now lives in a different function, so it is new there and gone from the old one")
		},
		Entry("callbacks inside different named functions", `export function first(rows: Row[]) {
  rows.forEach((row) => {
    row.seen = true;
  });
}
`, `export function second(items: Row[]) {
  items.map((row) => {
    row.seen = true;
  });
}
`),
		Entry("callbacks inside arrows bound to different variables", `export const first = (rows: Row[]) => {
  rows.forEach((row) => {
    row.seen = true;
  });
};
`, `export const second = (items: Row[]) => {
  items.map((row) => {
    row.seen = true;
  });
};
`),
		Entry("top-level arrows bound to different variables", `export const first = (row: Row) => {
  row.seen = true;
};
`, `export const second = (row: Row) => {
  row.seen = true;
};
`),
		Entry("function expressions assigned to different variables", `export const first = function (row: Row) {
  row.seen = true;
};
`, `export const second = function (row: Row) {
  row.seen = true;
};
`),
		Entry("arrows held by different class fields", `export class Button {
  onClick = (event: Ev) => {
    event.handled = true;
  };
}
`, `export class Button {
  onKey = (event: Ev) => {
    event.handled = true;
  };
}
`),
		Entry("arrows held by different object properties", `export const handlers = {
  first: (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  second: (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("callbacks inside arrows held by different class fields", `export class Button {
  onClick = (rows: Row[]) => {
    rows.forEach((row) => {
      row.seen = true;
    });
  };
}
`, `export class Button {
  onKey = (rows: Row[]) => {
    rows.forEach((row) => {
      row.seen = true;
    });
  };
}
`),
		Entry("parenthesized arrows bound to different variables", `export const first = ((row: Row) => {
  row.seen = true;
});
`, `export const second = ((row: Row) => {
  row.seen = true;
});
`),
		Entry("arrows asserted with as and bound to different variables", `export const first = ((row: Row) => {
  row.seen = true;
}) as Handler;
`, `export const second = ((row: Row) => {
  row.seen = true;
}) as Handler;
`),
		Entry("arrows checked with satisfies and bound to different variables", `export const first = ((row: Row) => {
  row.seen = true;
}) satisfies Handler;
`, `export const second = ((row: Row) => {
  row.seen = true;
}) satisfies Handler;
`),
		Entry("arrows assigned to different properties of this", `export class Button {
  constructor() {
    this.onClick = (event: Ev) => {
      event.handled = true;
    };
  }
}
`, `export class Button {
  constructor() {
    this.onKey = (event: Ev) => {
      event.handled = true;
    };
  }
}
`),
		Entry("function expressions assigned to different properties of an object", `declare const target: Handlers;
target.first = function (row: Row) {
  row.seen = true;
};
`, `declare const target: Handlers;
target.second = function (row: Row) {
  row.seen = true;
};
`),
	)

	It("reports an extra identical anonymous mutation inside one function as unknown, like a named duplicate", func() {
		repo := newTempGitRepo()
		twoCallbacks := `export function markAll(rows: Row[]) {
  rows.forEach((row) => {
    row.seen = true;
  });
  rows.forEach((row) => {
    row.seen = true;
  });
}
`
		threeCallbacks := strings.Replace(twoCallbacks, "}\n", `  rows.forEach((row) => {
    row.seen = true;
  });
}
`, 1)
		Expect(threeCallbacks).NotTo(Equal(twoCallbacks))
		initialSHA := commitFile(repo, "duplicates.ts", twoCallbacks)
		commitFile(repo, "duplicates.ts", threeCallbacks)

		report, _ := runCoachCodesignal(repo, initialSHA)

		lifecycles := []codesignal.Lifecycle{}
		for _, signal := range signalsForPath(report, "duplicates.ts") {
			if signal.RuleID == hiddenInputMutationRuleID {
				lifecycles = append(lifecycles, signal.Lifecycle)
			}
		}
		Expect(lifecycles).To(ConsistOf(codesignal.Lifecycle("existing"), codesignal.Lifecycle("existing"), codesignal.Lifecycle("unknown")),
			"identical mutations in one function share a key, so the engine cannot tell which of three is new")
	})
})
