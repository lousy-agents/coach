package modelgateway

// ValidationError is a typed schema/validation failure.
// errors.Is(err, ErrSchemaValidation) and errors.As(err, *ValidationError) both work.
type ValidationError struct {
	Detail string
}

func (e *ValidationError) Error() string {
	return formatPrefixedError(ErrSchemaValidation, e.detail())
}

func (e *ValidationError) detail() string {
	if e == nil {
		return ""
	}
	return e.Detail
}

func (e *ValidationError) Unwrap() error { return ErrSchemaValidation }

func NewValidationError(detail string) error {
	return &ValidationError{Detail: detail}
}
