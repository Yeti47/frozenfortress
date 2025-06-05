package login

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
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

		if middleware.HandleError(c, err) {
			return
		}

		if !response.Success {
			// Authentication failed - render login page with error message
			errorMessage := response.Error
			if errorMessage == "" {
				errorMessage = "Invalid username or password"
			}

			c.HTML(401, "login.html", gin.H{
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
		if middleware.HandleError(c, err) {
			return
		}

		// Set security headers to prevent caching and clear history
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("Clear-Site-Data", "\"cache\", \"cookies\", \"storage\", \"executionContexts\"")

		// Render login page with a success message
		c.HTML(200, "login.html", gin.H{
			"SuccessMessage": "You have been successfully logged out.",
		})
	})
}
