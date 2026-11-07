package gitrepo

import "testing"

// The table holds a row for every git subcommand the scan asks through RunBytes, Run or a Runner named run,
// found in the source of the scan and not in a list written beside the table. A subcommand without a row
// falls into the 30 s / 16 MiB default, which fails closed but is nobody's decision.

func TestEverySubcommandTheScanAsksHasARowInTheTable(t *testing.T) {
	asked := subcommandsAsked(t, "../../../internal", "../../../cmd")

	if len(asked) < 6 {
		t.Fatalf("found only %v: the scan of the source for calls is not finding them", asked)
	}
	unnamed := defaultBudgetFor([]string{"a-subcommand-no-one-named"})
	for _, subcommand := range asked {
		if defaultBudgetFor([]string{subcommand}) == unnamed {
			t.Errorf("git %s is asked by the scan and has no row in defaultBudgetFor: it gets the default %+v", subcommand, unnamed)
		}
	}
}

func TestAReadWithNoArgumentsGetsTheUnnamedBudgetAndDoesNotPanic(t *testing.T) {
	if got, want := defaultBudgetFor(nil), defaultBudgetFor([]string{"a-subcommand-no-one-named"}); got != want {
		t.Errorf("defaultBudgetFor(nil) = %+v, want the unnamed budget %+v", got, want)
	}
}
