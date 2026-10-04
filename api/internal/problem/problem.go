// Package problem maps application errors to RFC 9457 problem+json responses.
// It is the single place where ccc.ApiError is translated for the API; every
// handler returns its errors through Map.
package problem

import (
	"errors"
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/danielgtaylor/huma/v2"
)

// genericMessage is the detail sent for errors that are not ccc.ApiErrors.
const genericMessage = "An unexpected error occurred. Please try again later or contact your administrator."

// Map converts err into a huma.StatusError.
//
// A ccc.ApiError keeps its status code and user-facing message; for invalid-input
// errors the field name is exposed in errors[].location. Any other error becomes a 500
// with a generic message, and the real error is logged. Map returns nil for a nil error.
func Map(logger ccc.Logger, err error) huma.StatusError {
	if err == nil {
		return nil
	}
	if logger == nil {
		logger = ccc.NopLogger
	}

	var apiErr *ccc.ApiError
	if errors.As(err, &apiErr) {
		status := apiErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		if status >= 500 {
			logger.Error("Request failed", "error", err)
		}

		var details []error
		if apiErr.Field != "" {
			details = append(details, &huma.ErrorDetail{Message: apiErr.UserMessage, Location: apiErr.Field})
		}
		return huma.NewError(status, apiErr.UserMessage, details...)
	}

	logger.Error("Unexpected error", "error", err)
	return huma.NewError(http.StatusInternalServerError, genericMessage)
}
