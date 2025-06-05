package login

import (
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the login routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager) {

	// GET /login - Show login page
	router.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", gin.H{})
	})

	// POST /login - Handle login form submission
	router.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		// Create SignInRequest
		request := auth.SignInRequest{
			UserName: username,
			Password: password,
		}

		// Call SignInManager to handle authentication
		response, err := signInManager.SignIn(c.Writer, c.Request, request)

		if err != nil {
			// Handle error - render login page with error message
			c.HTML(500, "login.html", gin.H{
				"ErrorMessage": "Authentication failed. Please try again.",
				"Username":     username, // Pre-fill username field
			})
			return
		}

		if !response.Success {
			// Authentication failed - render login page with error message
			errorMessage := response.Error
			if errorMessage == "" {
				errorMessage = "Invalid username or password"
			}

			// Determine status code based on error type
			statusCode := 401 // Default to Unauthorized
			if strings.ToLower(response.Error) == "internal error" {
				statusCode = 500
			}

			c.HTML(statusCode, "login.html", gin.H{
				"ErrorMessage": errorMessage,
				"Username":     username, // Pre-fill username field
			})
			return
		}

		// Authentication successful - redirect to home page
		c.Redirect(302, "/")
	})

	// GET /logout - Handle logout
	router.GET("/logout", func(c *gin.Context) {
		// Call SignInManager to handle sign out
		err := signInManager.SignOut(c.Writer, c.Request)
		if err != nil {
			// Log error but still redirect to login
			// In a production app, you might want to show an error message
		}

		// Render login page with a success message
		c.HTML(200, "login.html", gin.H{
			"SuccessMessage": "You have been successfully logged out.",
		})
	})
}
