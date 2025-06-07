package middleware

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/gin-gonic/gin"
)

// HandleError handles errors by checking if they are ApiErrors and rendering the appropriate error page.
// Returns true if an error was handled, false if there was no error.
func HandleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	var statusCode int
	var errorMessage string

	// Check if it's an ApiError
	if apiErr, ok := ccc.IsApiError(err); ok {
		statusCode = apiErr.StatusCode
		errorMessage = apiErr.UserMessage
	} else {
		// Not an ApiError - use default error response
		statusCode = 500
		errorMessage = "An unexpected error occurred. Please try again later or contact your administrator."
	}

	// Render the error page
	c.HTML(statusCode, "error.html", gin.H{
		"StatusCode":   statusCode,
		"ErrorMessage": errorMessage,
	})

	return true
}

// HandleErrorOnPage handles errors by checking if they are ApiErrors and rendering a custom page.
// It allows specifying a custom template and merging the error information with additional template data.
// Returns true if an error was handled, false if there was no error.
func HandleErrorOnPage(c *gin.Context, err error, templateName string, templateData gin.H, errorKey string) bool {
	if err == nil {
		return false
	}

	var statusCode int
	var errorMessage string

	// Check if it's an ApiError
	if apiErr, ok := ccc.IsApiError(err); ok {
		statusCode = apiErr.StatusCode
		errorMessage = apiErr.UserMessage
	} else {
		// Not an ApiError - use default error response
		statusCode = 500
		errorMessage = "An unexpected error occurred. Please try again later or contact your administrator."
	}

	// Add error message to template data
	if templateData == nil {
		templateData = gin.H{}
	}
	templateData[errorKey] = errorMessage

	// Render the custom page
	c.HTML(statusCode, templateName, templateData)

	return true
}
