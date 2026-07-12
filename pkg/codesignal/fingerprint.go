package codesignal

// Fingerprint and ID computation. Fingerprints are stable across line
// moves within a file, but adding or removing an earlier duplicate with
// the same key can shift occurrence ordinals and change later
// fingerprints.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
)

func normalizePath(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimLeft(p, "/")
	return p
}

func normalizeEvidence(evidence string) string {
	return strings.TrimSpace(evidence)
}

func appendLengthPrefixed(buf []byte, field string) []byte {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(field)))
	buf = append(buf, lenBuf[:]...)
	buf = append(buf, field...)
	return buf
}

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
