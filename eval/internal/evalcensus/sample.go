package evalcensus

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"strings"
)

// DefaultSampleSize is how many of a rule's findings a repository's sample keeps when its manifest names no size.
const DefaultSampleSize = 30

// Frames a rule's sample can report. A rule with no more findings than the sample size keeps them all.
const (
	FrameAllFindings = "all_findings"
	FrameSampled     = "sampled"
)

// drawSample returns the frame and the sorted keys a seeded draw keeps of one rule's findings. The findings are
// put in a total order first, so the draw does not depend on the order Coach reported them in.
func drawSample(findings []reportSignal, seed int64, size int) (string, []string) {
	slices.SortFunc(findings, compareFindings)
	if len(findings) <= size {
		return FrameAllFindings, sortedKeys(findings, allIndexes(len(findings)))
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	return FrameSampled, sortedKeys(findings, rng.Perm(len(findings))[:size])
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

// sampleSize is the manifest's size, or DefaultSampleSize when it names none.
func sampleSize(repo Repository) int {
	if repo.Sample == nil || repo.Sample.Size == 0 {
		return DefaultSampleSize
	}
	return repo.Sample.Size
}
