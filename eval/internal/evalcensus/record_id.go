package evalcensus

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// NewRecordID mints a record_id: a hash of the record's content, so equal
// records share a prefix, and a random suffix, so two writers recording the
// same finding never mint the same id.
func NewRecordID(record Record) string {
	record.RecordID = ""
	content, _ := json.Marshal(record)
	digest := sha256.Sum256(content)
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	return hex.EncodeToString(digest[:8]) + "-" + hex.EncodeToString(suffix)
}
