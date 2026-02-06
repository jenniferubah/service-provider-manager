package service

// Error codes returned by service operations.
const (
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeValidation    = "VALIDATION"
	ErrCodeProviderError = "PROVIDER_ERROR"
	ErrCodeInternal      = "INTERNAL_ERROR"
)

// ServiceError represents a business logic error with a code for HTTP mapping.
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}
