package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
)

// FixtureKey adds to the generator's key what else decides a cached record: the hash of the coach binary that
// reported on the base ("" when there was none) and the pinned detector's version (design §6, "Caching").
func FixtureKey(generatorKey, coachHash, detectorVersion string) string {
	coach := "none:"
	if coachHash != "" {
		coach = "binary:" + coachHash
	}
	sum := sha256.Sum256([]byte(generatorKey + "|" + coach + "|gocognit=" + detectorVersion))
	return hex.EncodeToString(sum[:])
}
