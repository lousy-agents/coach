package modelgateway

import (
	"errors"
)

var (
	// ErrSchemaValidation indicates the model output failed schema validation
	// after any bounded retries the gateway applies.
	ErrSchemaValidation = errors.New("modelgateway: schema validation failed")

	// ErrUnavailable indicates a transient/unavailable condition (transport,
	// timeout, 5xx, connection failure). Callers may degrade to deterministic-only.
	ErrUnavailable = errors.New("modelgateway: unavailable")
)

func formatPrefixedError(sentinel error, detail string) string {
	if detail == "" {
		return sentinel.Error()
	}
	return sentinel.Error() + ": " + detail
}
