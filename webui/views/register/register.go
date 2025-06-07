package register

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the register routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, userManager auth.UserManager) {

	// GET /register - Show registration page
	router.GET("/register", func(c *gin.Context) {
		c.HTML(200, "register.html", gin.H{})
	})

	// POST /register - Handle registration form submission
	router.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		confirmPassword := c.PostForm("confirm_password")

		// Server-side password confirmation check
		if password != confirmPassword {
			c.HTML(400, "register.html", gin.H{
				"ErrorMessage": "Passwords do not match. Please ensure both password fields contain the same value.",
				"Username":     username, // Pre-fill username field
			})
			return
		}

		// Create CreateUserRequest
		request := auth.CreateUserRequest{
			UserName: username,
			Password: password,
		}

		// Call UserManager to create the user - it will handle all validation
		response, err := userManager.CreateUser(request)
		if err != nil {
			// Check if it's an ApiError to get the user-friendly message
			var errorMessage string
			if apiErr, ok := ccc.IsApiError(err); ok {
				errorMessage = apiErr.UserMessage
			} else {
				errorMessage = "An unexpected error occurred. Please try again later."
			}

			// Don't pre-fill username if it's a duplicate username error
			preFilledUserName := username
			if ccc.IsErrorCode(err, ccc.ErrCodeUserNameTaken) {
				preFilledUserName = ""
			}

			c.HTML(400, "register.html", gin.H{
				"ErrorMessage": errorMessage,
				"Username":     preFilledUserName,
			})
			return
		}

		// Registration successful - show success message with recovery code
		successMessage := "Thank you, " + username + "! Your account creation request has been submitted successfully. " +
			"Your account will be activated by an administrator and you'll be able to sign in once the activation is complete. " +
			"Please check back later or contact your administrator for activation status."

		c.HTML(200, "register.html", gin.H{
			"SuccessMessage":  successMessage,
			"RecoveryCode":    response.RecoveryCode,
			"RecoveryContext": "registration",
			"Username":        username,
		})
	})
}
