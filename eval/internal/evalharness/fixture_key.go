package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
)

// FixtureKey adds to the generator's key what else decides a cached record: the hash of the coach binary that
// reported on the base ("" when there was none), the pinned detector's version and the validation fingerprint, so
// that a program validated by an older step list is regenerated and not served as if it had passed the new steps
// (design §6, "Caching").
func FixtureKey(generatorKey, coachHash, detectorVersion, validation string) string {
	coach := "none:"
	if coachHash != "" {
		coach = "binary:" + coachHash
	}
	sum := sha256.Sum256([]byte(generatorKey + "|" + coach + "|gocognit=" + detectorVersion + "|validation=" + validation))
	return hex.EncodeToString(sum[:])
}
