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

// Helper functions for creating ServiceErrors

func NewNotFoundError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeNotFound,
		Message: message,
	}
}

func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeConflict,
		Message: message,
	}
}

func NewValidationError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeValidation,
		Message: message,
	}
}

func NewProviderError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeProviderError,
		Message: message,
	}
}

func NewInternalError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrCodeInternal,
		Message: message,
	}
}
