package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal TypeScript reduce accumulators", func() {
	DescribeTable("reports state.hidden_input_mutation only for caller-owned values",
		func(addedFunction string, wantEvidence []string) {
			repo := newTempGitRepo()
			base := "export type Item = { name: string; value: string; seen?: boolean };\n"
			initialSHA := commitFile(repo, "reduce.ts", base)
			commitFile(repo, "reduce.ts", base+"\n"+addedFunction)

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(report.Summary.FilesAnalyzed).To(Equal(1), "the file must be analyzed, or an empty result proves nothing")
			Expect(report.Diagnostics).To(BeEmpty())
			gotEvidence := []string{}
			for _, signal := range signalsForPath(report, "reduce.ts") {
				if signal.RuleID == "state.hidden_input_mutation" {
					gotEvidence = append(gotEvidence, signal.Evidence)
				}
			}
			Expect(gotEvidence).To(ConsistOf(wantEvidence))
		},
		Entry("a fresh object literal accumulator asserted to a record type", `export function byName(items: Item[]) {
  return items.reduce((acc, item) => {
    acc[item.name] = item.value;
    return acc;
  }, {} as Record<string, string>);
}
`, []string{}),
		Entry("a fresh array literal accumulator", `export function names(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.name);
    return acc;
  }, [] as string[]);
}
`, []string{}),
		Entry("a fresh Map accumulator", `export function index(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.set(item.name, item.value);
    return acc;
  }, new Map<string, string>());
}
`, []string{}),
		Entry("reduceRight with a fresh accumulator", `export function reversed(items: Item[]) {
  return items.reduceRight((acc, item) => {
    acc[item.name] = item.value;
    return acc;
  }, {} as Record<string, string>);
}
`, []string{}),
		Entry("a function expression callback with a typed accumulator", `export function typed(items: Item[]) {
  return items.reduce(function (acc: Record<string, string>, item) {
    acc[item.name] = item.value;
    return acc;
  }, {});
}
`, []string{}),
		Entry("a function that mutates its own items parameter", `export function drain(items: Item[]) {
  items.length = 0;
}
`, []string{"items.length"}),
		Entry("an accumulator seeded from a caller-owned parameter", `export function tally(items: Item[], totals: Record<string, number>) {
  return items.reduce((acc, item) => {
    acc[item.name] = 1;
    return acc;
  }, totals);
}
`, []string{"acc[item.name]"}),
		Entry("a reduce with no initial value", `export function merge(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.value = item.value;
    return acc;
  });
}
`, []string{"acc.value"}),
		Entry("a fresh accumulator whose callback mutates the item parameter", `export function markAll(items: Item[]) {
  return items.reduce((acc, item) => {
    item.seen = true;
    return acc;
  }, {} as Record<string, boolean>);
}
`, []string{"item.seen"}),
		Entry("a fresh accumulator whose callback mutates the source array", `export function clearSource(items: Item[]) {
  return items.reduce((acc, item, index, source) => {
    source.length = 0;
    return acc;
  }, [] as string[]);
}
`, []string{"source.length"}),
		Entry("an initial value that captures caller-owned data", `export function wrap(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.items.push(item);
    return acc;
  }, { items });
}
`, []string{"acc.items.push"}),
		Entry("a fresh first accumulator whose callback returns caller-owned data", `export function replace(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.name);
    return item.names;
  }, [] as string[]);
}
`, []string{"acc.push"}),
		Entry("a fresh accumulator the callback reassigns to caller-owned data before returning it", `export function reset(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.name);
    if (!item.name) {
      acc = item.names;
    }
    return acc;
  }, [] as string[]);
}
`, []string{"acc.push"}),
		Entry("a fresh accumulator that a closure inside the callback reassigns", `export function resetByClosure(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.name);
    const reset = () => {
      acc = item.names;
    };
    reset();
    return acc;
  }, [] as string[]);
}
`, []string{"acc.push"}),
		Entry("a fresh accumulator reassigned inside a returned comma expression", `export function resetInReturn(items: Item[]) {
  return items.reduce((acc, item) => {
    acc.push(item.name);
    return (acc = item.names, acc);
  }, [] as string[]);
}
`, []string{"acc.push"}),
		Entry("a fresh accumulator that every branch of the callback returns", `export function branches(items: Item[]) {
  return items.reduce((acc, item) => {
    if (!item.name) {
      return acc;
    }
    acc.push(item.name);
    return acc;
  }, [] as string[]);
}
`, []string{}),
		Entry("a fresh accumulator returned through a comma expression", `export function sequence(items: Item[]) {
  return items.reduce((acc, item) => (acc.push(item.name), acc), [] as string[]);
}
`, []string{}),
	)
})
