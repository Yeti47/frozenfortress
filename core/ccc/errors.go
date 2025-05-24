package ccc

import "fmt"

// ErrorCode represents different types of application errors
type ErrorCode string

const (
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeAlreadyExists    ErrorCode = "ALREADY_EXISTS"
	ErrCodeInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	ErrCodeDatabaseError    ErrorCode = "DATABASE_ERROR"
	ErrCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeInternalError    ErrorCode = "INTERNAL_ERROR"
	ErrCodeOperationFailed  ErrorCode = "OPERATION_FAILED"
)

// ApiError represents application-specific errors with both user-friendly and technical details
type ApiError struct {
	StatusCode       int       // HTTP status code (for future web API compatibility)
	Code             ErrorCode // Error code for programmatic handling
	UserMessage      string    // User-friendly message safe to display to end users
	TechnicalMessage string    // Technical details for logging/debugging
	Cause            error     // Underlying error (for logging/debugging)
}

func (e *ApiError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s (technical: %s, caused by: %v)", e.Code, e.UserMessage, e.TechnicalMessage, e.Cause)
	}
	return fmt.Sprintf("[%s] %s (technical: %s)", e.Code, e.UserMessage, e.TechnicalMessage)
}

func (e *ApiError) Unwrap() error {
	return e.Cause
}

// Constructor functions for common error scenarios

// NewResourceNotFoundError creates an error for when a resource cannot be found
func NewResourceNotFoundError(identifier string, resourceType string) *ApiError {
	return &ApiError{
		StatusCode:       404,
		Code:             ErrCodeNotFound,
		UserMessage:      fmt.Sprintf("%s not found", resourceType),
		TechnicalMessage: fmt.Sprintf("%s not found with identifier: %s", resourceType, identifier),
	}
}

// NewResourceAlreadyExistsError creates an error for when a resource already exists
func NewResourceAlreadyExistsError(identifier string, resourceType string) *ApiError {
	return &ApiError{
		StatusCode:       409,
		Code:             ErrCodeAlreadyExists,
		UserMessage:      fmt.Sprintf("%s already exists", resourceType),
		TechnicalMessage: fmt.Sprintf("%s already exists with identifier: %s", resourceType, identifier),
	}
}

// NewInvalidInputError creates an error for invalid input parameters
func NewInvalidInputError(field, reason string) *ApiError {
	return &ApiError{
		StatusCode:       400,
		Code:             ErrCodeInvalidInput,
		UserMessage:      "Invalid input provided",
		TechnicalMessage: fmt.Sprintf("invalid %s: %s", field, reason),
	}
}

// NewValidationFailedError creates an error for validation failures
func NewValidationFailedError(message string) *ApiError {
	return &ApiError{
		StatusCode:       400,
		Code:             ErrCodeValidationFailed,
		UserMessage:      "Validation failed",
		TechnicalMessage: message,
	}
}

// NewDatabaseError creates an error for database operation failures
func NewDatabaseError(operation string, cause error) *ApiError {
	return &ApiError{
		StatusCode:       500,
		Code:             ErrCodeDatabaseError,
		UserMessage:      "A database error occurred",
		TechnicalMessage: fmt.Sprintf("database operation failed: %s", operation),
		Cause:            cause,
	}
}

// NewOperationFailedError creates an error for when an operation fails but not due to validation
func NewOperationFailedError(operation, reason string) *ApiError {
	return &ApiError{
		StatusCode:       500,
		Code:             ErrCodeOperationFailed,
		UserMessage:      "Operation failed",
		TechnicalMessage: fmt.Sprintf("operation '%s' failed: %s", operation, reason),
	}
}

// NewInternalError creates an error for unexpected internal errors
func NewInternalError(message string, cause error) *ApiError {
	return &ApiError{
		StatusCode:       500,
		Code:             ErrCodeInternalError,
		UserMessage:      "An internal error occurred",
		TechnicalMessage: message,
		Cause:            cause,
	}
}

// NewUnauthorizedError creates an error for unauthorized access
func NewUnauthorizedError(message string) *ApiError {
	return &ApiError{
		StatusCode:       401,
		Code:             ErrCodeUnauthorized,
		UserMessage:      "Unauthorized access",
		TechnicalMessage: message,
	}
}

// NewForbiddenError creates an error for forbidden access
func NewForbiddenError(message string) *ApiError {
	return &ApiError{
		StatusCode:       403,
		Code:             ErrCodeForbidden,
		UserMessage:      "Access forbidden",
		TechnicalMessage: message,
	}
}

// Helper functions for error checking

// IsApiError checks if an error is an ApiError and returns it
func IsApiError(err error) (*ApiError, bool) {
	if apiErr, ok := err.(*ApiError); ok {
		return apiErr, true
	}
	return nil, false
}

// IsErrorCode checks if an error has a specific error code
func IsErrorCode(err error, code ErrorCode) bool {
	if apiErr, ok := IsApiError(err); ok {
		return apiErr.Code == code
	}
	return false
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return IsErrorCode(err, ErrCodeNotFound)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return IsErrorCode(err, ErrCodeValidationFailed) || IsErrorCode(err, ErrCodeInvalidInput)
}

// IsDatabaseError checks if an error is a database error
func IsDatabaseError(err error) bool {
	return IsErrorCode(err, ErrCodeDatabaseError)
}
