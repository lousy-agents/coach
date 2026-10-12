package codesignal

import (
	"cmp"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

type rankingRule struct {
	id       string
	evidence func(spread int) string
}

func metricRankingRule(rule metricRule) rankingRule {
	return rankingRule{id: rule.ruleID, evidence: func(spread int) string { return rule.evidence(rule.threshold + spread) }}
}

func noMagnitudeRankingRule(id string) rankingRule {
	return rankingRule{id: id, evidence: func(int) string { return "no numeric metric" }}
}

var rankingRules = []rankingRule{
	metricRankingRule(cognitiveComplexityRule),
	metricRankingRule(branchDensityRule),
	metricRankingRule(maxNestingDepthRule),
	noMagnitudeRankingRule("state.hidden_input_mutation"),
	noMagnitudeRankingRule("security.toctou_check_then_act"),
}

func pick[T any](rng *rand.Rand, options []T) T {
	return options[rng.Intn(len(options))]
}

// randomRankingSignal spreads magnitudes over a few values so ties occur, and
// gives every signal a distinct ID.
func randomRankingSignal(rng *rand.Rand, id int) Signal {
	rule := pick(rng, rankingRules)
	signal := sortableSignal(
		fmt.Sprintf("sig_%03d", id),
		rule.id,
		fmt.Sprintf("%c.go", 'a'+rng.Intn(6)),
		pick(rng, []Lifecycle{"introduced", "existing", "resolved"}),
		rng.Intn(2) == 0,
		pick(rng, []Severity{"high", "medium", "low"}),
		pick(rng, []Confidence{"high", "medium"}),
		uint(rng.Intn(4)),
		0,
	)
	signal.Evidence = rule.evidence(rng.Intn(6))
	return signal
}

// pathOrder is the ordering that predates magnitude ranking, spelled out so the
// properties do not follow the production comparator they check. It copies the
// tier keys on purpose: the properties check only movement within a tier, and
// tier order itself is pinned independently by the magnitude-ranking and
// severity-escalation acceptance specs.
func pathOrder(a, b Signal) int {
	return cmp.Or(
		cmp.Compare(signalPriorityGroup(a), signalPriorityGroup(b)),
		cmp.Compare(severityRank(b.Severity), severityRank(a.Severity)),
		cmp.Compare(confidenceRank(b.Confidence), confidenceRank(a.Confidence)),
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Location.StartRow, b.Location.StartRow),
		cmp.Compare(a.Location.StartCol, b.Location.StartCol),
		cmp.Compare(a.RuleID, b.RuleID),
		cmp.Compare(a.ID, b.ID),
	)
}

func signalIDs(signals []Signal) []string {
	ids := make([]string, len(signals))
	for i, signal := range signals {
		ids[i] = signal.ID
	}
	return ids
}

func sortedCopy(signals []Signal) []Signal {
	out := slices.Clone(signals)
	sortSignals(out)
	return out
}

// rankingTier is the (group, severity, confidence) tier a signal sorts into,
// spelled out apart from the production peer key.
type rankingTier struct {
	group, severity, confidence int
}

func tierOf(signal Signal) rankingTier {
	return rankingTier{signalPriorityGroup(signal), severityRank(signal.Severity), confidenceRank(signal.Confidence)}
}

// assertPositionsKeepRuleAndTier checks that ranking only permutes signals
// within the positions their own rule and tier already hold in path order, so
// magnitude is never compared across rules or tiers. A signal without a
// magnitude keeps its exact path-order position.
func assertPositionsKeepRuleAndTier(t *testing.T, sorted, input []Signal) {
	t.Helper()
	pathOrdered := slices.Clone(input)
	slices.SortStableFunc(pathOrdered, pathOrder)
	for position, signal := range sorted {
		want := pathOrdered[position]
		if signal.RuleID != want.RuleID || tierOf(signal) != tierOf(want) {
			t.Fatalf("position %d holds %s (rule %s, tier %v); path order has rule %s, tier %v there",
				position, signal.ID, signal.RuleID, tierOf(signal), want.RuleID, tierOf(want))
		}
		if _, hasMagnitude := signalMagnitude(signal); !hasMagnitude && want.ID != signal.ID {
			t.Fatalf("no-magnitude signal %s moved from path position %d (path order has %s there)", signal.ID, position, want.ID)
		}
	}
}

func assertPeersNonIncreasing(t *testing.T, sorted []Signal) {
	t.Helper()
	type peerGroup struct {
		tier   rankingTier
		ruleID string
	}
	lastRatio := make(map[peerGroup]float64)
	for position, signal := range sorted {
		ratio, hasMagnitude := signalMagnitude(signal)
		key := peerGroup{tierOf(signal), signal.RuleID}
		previous, seen := lastRatio[key]
		if hasMagnitude && seen && ratio > previous {
			t.Fatalf("%s at %d has ratio %v above the earlier peer's %v", signal.ID, position, ratio, previous)
		}
		if hasMagnitude {
			lastRatio[key] = ratio
		}
	}
}

func assertRankingProperties(t *testing.T, rng *rand.Rand) {
	t.Helper()
	input := make([]Signal, 5+rng.Intn(40))
	for i := range input {
		input[i] = randomRankingSignal(rng, i)
	}
	sorted := sortedCopy(input)

	assertPositionsKeepRuleAndTier(t, sorted, input)
	assertPeersNonIncreasing(t, sorted)
	if again := sortedCopy(sorted); !slices.Equal(signalIDs(again), signalIDs(sorted)) {
		t.Fatalf("sorting a sorted list moved signals:\n got %v\nwant %v", signalIDs(again), signalIDs(sorted))
	}
	shuffled := slices.Clone(input)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	if got := sortedCopy(shuffled); !slices.Equal(signalIDs(got), signalIDs(sorted)) {
		t.Fatalf("input order changed the result:\n got %v\nwant %v", signalIDs(got), signalIDs(sorted))
	}
}

func TestSortSignals_RankingPropertiesOverRandomSignalSets(t *testing.T) {
	rng := rand.New(rand.NewSource(272))
	for round := range 200 {
		t.Run(fmt.Sprintf("round %d", round), func(t *testing.T) { assertRankingProperties(t, rng) })
	}
}
