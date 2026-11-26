package evalcensus

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strings"
)

// SampleSize is how many of a rule's findings a repository's sample keeps. It is fixed, as #260 and the design's
// §11 fix it, so a manifest cannot change it.
const SampleSize = 30

// Frames a rule's sample can report. A rule with no more findings than SampleSize keeps them all. The design's
// not_sampled frame is in the schema but never emitted, since every census draws with a recorded seed.
const (
	FrameAllFindings = "all_findings"
	FrameRandom30    = "random_30"
)

// drawSample returns the frame and the sorted keys a seeded draw keeps of one rule's findings. The findings are
// put in a total order first, so the draw does not depend on the order Coach reported them in.
func drawSample(findings []reportSignal, seed int64) (string, []string) {
	slices.SortFunc(findings, compareFindings)
	if len(findings) <= SampleSize {
		return FrameAllFindings, sortedKeys(findings, allIndexes(len(findings)))
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	return FrameRandom30, sortedKeys(findings, rng.Perm(len(findings))[:SampleSize])
}

func allIndexes(count int) []int {
	indexes := make([]int, count)
	for i := range indexes {
		indexes[i] = i
	}
	return indexes
}

func sortedKeys(findings []reportSignal, indexes []int) []string {
	keys := make([]string, 0, len(indexes))
	for _, index := range indexes {
		keys = append(keys, findings[index].Fingerprint)
	}
	slices.Sort(keys)
	return keys
}

// compareFindings orders findings by rule, path, lines, subject and fingerprint, which together are total.
func compareFindings(a, b reportSignal) int {
	return cmp.Or(
		strings.Compare(a.RuleID, b.RuleID),
		strings.Compare(a.Path, b.Path),
		cmp.Compare(a.Location.StartRow, b.Location.StartRow),
		cmp.Compare(a.Location.EndRow, b.Location.EndRow),
		strings.Compare(a.Subject, b.Subject),
		strings.Compare(a.Fingerprint, b.Fingerprint),
	)
}

// sampleSeed is the manifest's seed, or zero for a repository with no sample.
func sampleSeed(repo Repository) int64 {
	if repo.Sample == nil || repo.Sample.Seed == nil {
		return 0
	}
	return *repo.Sample.Seed
}
