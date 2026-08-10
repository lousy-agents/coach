package codesignal

// computeProjectFingerprint and computeProjectChangeID mirror the
// file-signal hashing scheme while including every identity input that can
// change the meaning of a project observation. Anchors and evidence are
// deliberately excluded: they affect Changed, not lifecycle identity.

import (
	"crypto/sha256"
	"encoding/hex"
)

func appendProjectIdentity(buf []byte, change ProjectChange) []byte {
	for _, value := range []string{
		change.SemanticKey,
		change.RuleID,
		change.RuleVersion,
		change.BackendVersion,
		change.AlgorithmVersion,
		change.ConfigDigest,
	} {
		buf = appendLengthPrefixed(buf, value)
	}
	return buf
}

func computeProjectFingerprint(change ProjectChange) string {
	buf := appendProjectIdentity(nil, change)

	sum := sha256.Sum256(buf)
	return "pfp_" + hex.EncodeToString(sum[:])
}

func computeProjectChangeID(change ProjectChange) string {
	buf := appendProjectIdentity(nil, change)

	sum := sha256.Sum256(buf)
	return "pchg_" + hex.EncodeToString(sum[:])
}
