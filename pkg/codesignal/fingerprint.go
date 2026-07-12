package codesignal

// Fingerprint and ID computation. A Signal's Fingerprint is meant to be
// stable across a file's occurrence-agnostic identity (rule, path, subject,
// evidence) plus a within-key occurrence ordinal, so that moving a finding
// to a different line in the same file (without otherwise changing it)
// doesn't change its Fingerprint -- Lifecycle classification depends on
// this to match "the same" finding across Base and Head. The occurrence
// ordinal is assigned by sorting same-key duplicates by location and
// numbering them 0,1,2,...; this means removing or adding a duplicate
// finding at the same key shifts the ordinals (and therefore Fingerprints)
// of every later duplicate in that group -- a genuinely unchanged
// duplicate can appear to "resolve" and "reintroduce" itself when an
// earlier same-key duplicate elsewhere in the file is added or removed.
// Fingerprints are location-independent only up to this ordinal-shift
// caveat; they are not proof against duplicate reordering.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
)

// normalizePath canonicalizes path separators and strips a leading "./" or
// "/" so the same logical file doesn't produce different fingerprints
// depending on how its path was spelled.
func normalizePath(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimLeft(p, "/")
	return p
}

// normalizeEvidence trims incidental leading/trailing whitespace so
// fingerprints don't depend on it.
func normalizeEvidence(evidence string) string {
	return strings.TrimSpace(evidence)
}

// appendLengthPrefixed appends field to buf as a 4-byte big-endian length
// prefix (the field's UTF-8 byte length) followed by the field's bytes,
// preventing field-boundary collisions between adjacent fields.
func appendLengthPrefixed(buf []byte, field string) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(field)))
	buf = append(buf, lenBuf[:]...)
	buf = append(buf, field...)
	return buf
}

// computeFingerprint derives a deterministic, location-independent (up to
// the ordinal-shift caveat documented above) identity for a signal
// occurrence from its rule, normalized path, subject, normalized evidence,
// and within-key occurrence ordinal.
func computeFingerprint(ruleID, path, subject, evidence string, ordinal int) string {
	var buf []byte
	buf = appendLengthPrefixed(buf, ruleID)
	buf = appendLengthPrefixed(buf, normalizePath(path))
	buf = appendLengthPrefixed(buf, subject)
	buf = appendLengthPrefixed(buf, normalizeEvidence(evidence))
	buf = appendLengthPrefixed(buf, strconv.Itoa(ordinal))

	sum := sha256.Sum256(buf)
	return "fp_" + hex.EncodeToString(sum[:])
}

// computeSignalID derives a deterministic per-occurrence identity that,
// unlike computeFingerprint, also incorporates the signal's location, so
// two occurrences of "the same" finding that moved to a different
// row/column get different IDs even though they may share a Fingerprint.
func computeSignalID(ruleID, path, subject, evidence string, startRow, startCol uint, ordinal int) string {
	var buf []byte
	buf = appendLengthPrefixed(buf, ruleID)
	buf = appendLengthPrefixed(buf, normalizePath(path))
	buf = appendLengthPrefixed(buf, subject)
	buf = appendLengthPrefixed(buf, normalizeEvidence(evidence))
	buf = appendLengthPrefixed(buf, strconv.FormatUint(uint64(startRow), 10))
	buf = appendLengthPrefixed(buf, strconv.FormatUint(uint64(startCol), 10))
	buf = appendLengthPrefixed(buf, strconv.Itoa(ordinal))

	sum := sha256.Sum256(buf)
	return "sig_" + hex.EncodeToString(sum[:])
}
