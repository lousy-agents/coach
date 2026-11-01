package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func lifecyclesAfterMove(base, head string) []codesignal.Lifecycle {
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
	return lifecycles
}

var _ = Describe("coach codesignal anonymous callback holders", func() {
	DescribeTable("reports a mutation that moves between holders with the same bare name as introduced plus resolved",
		func(base, head string) {
			Expect(lifecyclesAfterMove(base, head)).To(
				ConsistOf(codesignal.Lifecycle("introduced"), codesignal.Lifecycle("resolved")),
				"the mutation now lives in a different holder, so it is new there and gone from the old one")
		},
		Entry("callbacks in same-named methods of different classes", `export class First {
  run(rows: Row[]) {
    rows.forEach((row) => {
      row.seen = true;
    });
  }
}
`, `export class Second {
  run(rows: Row[]) {
    rows.forEach((row) => {
      row.seen = true;
    });
  }
}
`),
		Entry("callbacks in a getter and a setter with the same name", `export class Box {
  get value() {
    this.rows.forEach((row) => {
      row.seen = true;
    });
    return 1;
  }
}
`, `export class Box {
  set value(next: number) {
    this.rows.forEach((row) => {
      row.seen = true;
    });
  }
}
`),
		Entry("arrows under different quoted object keys", `export const handlers = {
  "first": (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  "second": (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("arrows under different numeric object keys", `export const handlers = {
  1: (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  2: (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("arrows under different computed identifier object keys", `export const handlers = {
  [FIRST]: (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  [SECOND]: (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("arrows under different computed literal object keys", `export const handlers = {
  ["first"]: (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  ["second"]: (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("arrows under different quoted class field names", `export class Button {
  "onClick" = (event: Ev) => {
    event.handled = true;
  };
}
`, `export class Button {
  "onKey" = (event: Ev) => {
    event.handled = true;
  };
}
`),
		Entry("arrows under different computed class field names", `export class Button {
  [FIRST] = (event: Ev) => {
    event.handled = true;
  };
}
`, `export class Button {
  [SECOND] = (event: Ev) => {
    event.handled = true;
  };
}
`),
		Entry("arrows assigned to different computed identifier members", `declare const target: Handlers;
target[FIRST] = (row: Row) => {
  row.seen = true;
};
`, `declare const target: Handlers;
target[SECOND] = (row: Row) => {
  row.seen = true;
};
`),
		Entry("an arrow under a quoted key that spells a class-qualified name, and the real method", `export const handlers = {
  "First.run": (row: Row) => {
    row.seen = true;
  },
};
`, `export class First {
  run(rows: Row[]) {
    rows.forEach((row) => {
      row.seen = true;
    });
  }
}
`),
		Entry("an arrow under a quoted key that spells an accessor name, and the real accessor", `export const holder = {
  "get value": (row: Row) => {
    row.seen = true;
  },
};
`, `export const holder = {
  get value() {
    this.rows.forEach((row) => {
      row.seen = true;
    });
    return 1;
  },
};
`),
		Entry("an arrow under a quoted key that spells a computed key, and the computed key", `export const handlers = {
  "[KEY]": (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  [KEY]: (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("arrows assigned to different quoted members", `declare const target: Handlers;
target["first"] = (row: Row) => {
  row.seen = true;
};
`, `declare const target: Handlers;
target["second"] = (row: Row) => {
  row.seen = true;
};
`),
	)

	DescribeTable("keeps a holder's identity when only the spelling of its key changes",
		func(base, head string) {
			Expect(lifecyclesAfterMove(base, head)).To(ConsistOf(codesignal.Lifecycle("existing")),
				"the same property under another spelling is the same holder")
		},
		Entry("an identifier key that gains quotes", `export const handlers = {
  mark: (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  "mark": (row: Row) => {
    row.seen = true;
  },
};
`),
		Entry("a quoted key that moves into brackets", `export const handlers = {
  "mark": (row: Row) => {
    row.seen = true;
  },
};
`, `export const handlers = {
  ["mark"]: (row: Row) => {
    row.seen = true;
  },
};
`),
	)

	DescribeTable("documents the limit for code with no stable holder name",
		func(base, head string) {
			Expect(lifecyclesAfterMove(base, head)).To(ConsistOf(codesignal.Lifecycle("existing")),
				"module-level code and immediately invoked functions have no name to key on, so a move between them reads existing")
		},
		Entry("callbacks at module level", `rows.forEach((row) => {
  row.seen = true;
});
`, `items.map((row) => {
  row.seen = true;
});
`),
		Entry("immediately invoked functions", `(function (row: Row) {
  row.seen = true;
})(first);
`, `(function (row: Row) {
  row.seen = true;
})(second);
`),
	)
})
