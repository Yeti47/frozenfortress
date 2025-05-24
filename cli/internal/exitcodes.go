package internal

import "github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"

// ExitCode represents different CLI exit codes
type ExitCode int

const (
	// ExitSuccess indicates successful execution
	ExitSuccess ExitCode = 0

	// ExitUserNotFound indicates the specified user was not found
	ExitUserNotFound ExitCode = 1

	// ExitInvalidInput indicates invalid input was provided
	ExitInvalidInput ExitCode = 2

	// ExitDatabaseError indicates a database operation failed
	ExitDatabaseError ExitCode = 3

	// ExitOperationFailed indicates the operation failed for business reasons
	ExitOperationFailed ExitCode = 4

	// ExitInternalError indicates an unexpected internal error occurred
	ExitInternalError ExitCode = 5

	// ExitConfigurationError indicates a configuration error
	ExitConfigurationError ExitCode = 6
)

// ExitCodeFromApiError maps an ApiError to an appropriate CLI exit code
func ExitCodeFromApiError(apiErr *ccc.ApiError) ExitCode {
	switch apiErr.Code {
	case ccc.ErrCodeNotFound:
		return ExitUserNotFound
	case ccc.ErrCodeInvalidInput, ccc.ErrCodeValidationFailed:
		return ExitInvalidInput
	case ccc.ErrCodeDatabaseError:
		return ExitDatabaseError
	case ccc.ErrCodeOperationFailed:
		return ExitOperationFailed
	case ccc.ErrCodeInternalError:
		return ExitInternalError
	default:
		return ExitInternalError
	}
}

// ExitCodeFromError maps any error to an appropriate CLI exit code
func ExitCodeFromError(err error) ExitCode {
	if apiErr, ok := ccc.IsApiError(err); ok {
		return ExitCodeFromApiError(apiErr)
	}
	return ExitInternalError
}
