package modelgateway

func NewValidationError(detail string) error {
	return &ValidationError{Detail: detail}
}
