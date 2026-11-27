package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
)

// FixtureKey adds to the generator's key what else decides a cached record: the hash of the coach binary that
// reported on the base ("" when there was none), the pinned detector's version, the validation fingerprint and
// whether the mutation gate ran, so that a program validated by an older step list is regenerated and not served as
// if it had passed the new steps (design §6, "Caching"), and a program whose gate was skipped, which records
// mutation_checked: false, is not served to a run that asks for the gate (#906).
func FixtureKey(generatorKey, coachHash, detectorVersion, validation, gate string) string {
	coach := "none:"
	if coachHash != "" {
		coach = "binary:" + coachHash
	}
	sum := sha256.Sum256([]byte(generatorKey + "|" + coach + "|gocognit=" + detectorVersion + "|validation=" + validation + "|gate=" + gate))
	return hex.EncodeToString(sum[:])
}
