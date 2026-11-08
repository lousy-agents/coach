package gitrepo

// emptyBlobIDs are the ids of the empty blob in each object format, keyed by the
// length of the id: 40 hexadecimal digits for SHA-1 and 64 for SHA-256. An index
// entry holds the empty blob when no content was staged for it.
var emptyBlobIDs = map[int]string{
	40: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
	64: "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813",
}

// IsEmptyBlob reports whether id is the empty blob of the object format its length
// belongs to. An id of any other length is not.
func IsEmptyBlob(id string) bool {
	known, ok := emptyBlobIDs[len(id)]
	return ok && id == known
}
