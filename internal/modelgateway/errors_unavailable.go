package modelgateway

import (
	"errors"
)

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
	return formatPrefixedError(ErrUnavailable, joinDetailCause(e.Detail, e.Err))
}

func (e *UnavailableError) Unwrap() error {
	if e == nil || e.Err == nil {
		return ErrUnavailable
	}
	return errors.Join(ErrUnavailable, e.Err)
}

func NewUnavailableError(detail string, cause error) error {
	return &UnavailableError{Detail: detail, Err: cause}
}

func joinDetailCause(detail string, cause error) string {
	switch {
	case detail == "" && cause == nil:
		return ""
	case cause == nil:
		return detail
	case detail == "":
		return cause.Error()
	default:
		return detail + ": " + cause.Error()
	}
}
