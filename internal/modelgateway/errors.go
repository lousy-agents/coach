package modelgateway

import "errors"

var (
	// ErrSchemaValidation indicates the model output failed schema validation
	// after any bounded retries the gateway applies.
	ErrSchemaValidation = errors.New("modelgateway: schema validation failed")

	// ErrUnavailable indicates a transient/unavailable condition (transport,
	// timeout, 5xx, connection failure). Callers may degrade to deterministic-only.
	ErrUnavailable = errors.New("modelgateway: unavailable")
)

// ValidationError is a typed schema/validation failure.
// errors.Is(err, ErrSchemaValidation) and errors.As(err, *ValidationError) both work.
type ValidationError struct {
	Detail string
}

func (e *ValidationError) Error() string {
	if e == nil || e.Detail == "" {
		return ErrSchemaValidation.Error()
	}
	return ErrSchemaValidation.Error() + ": " + e.Detail
}

func (e *ValidationError) Unwrap() error { return ErrSchemaValidation }

func NewValidationError(detail string) error {
	return &ValidationError{Detail: detail}
}

// UnavailableError is a typed unavailable/transient failure.
// errors.Is(err, ErrUnavailable) and errors.As(err, *UnavailableError) both work.
type UnavailableError struct {
	Detail string
	Err    error
}

func (e *UnavailableError) Error() string {
	if e == nil {
		return ErrUnavailable.Error()
	}
	if e.Detail == "" && e.Err == nil {
		return ErrUnavailable.Error()
	}
	if e.Err == nil {
		return ErrUnavailable.Error() + ": " + e.Detail
	}
	if e.Detail == "" {
		return ErrUnavailable.Error() + ": " + e.Err.Error()
	}
	return ErrUnavailable.Error() + ": " + e.Detail + ": " + e.Err.Error()
}

func (e *UnavailableError) Unwrap() error {
	if e == nil {
		return ErrUnavailable
	}
	if e.Err == nil {
		return ErrUnavailable
	}
	return errors.Join(ErrUnavailable, e.Err)
}

func NewUnavailableError(detail string, cause error) error {
	return &UnavailableError{Detail: detail, Err: cause}
}
