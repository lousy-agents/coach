package evalharness

const (
	formatText = "text"
	formatJSON = "json"
)

func validOutputFormat(format string) bool {
	return format == formatText || format == formatJSON
}

// orEmpty keeps an empty JSON array an array, never null, so a consumer can iterate without a check.
func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
