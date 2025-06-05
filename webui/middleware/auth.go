package middleware

import (
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware is a Gin middleware that checks if the user is authenticated.
// If the user is not authenticated, it redirects them to the login page.
func AuthMiddleware(signInManager auth.SignInManager) gin.HandlerFunc {

	return func(c *gin.Context) {

		user, err := signInManager.GetCurrentUser(c.Request)
		if err != nil || user.Id == "" {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Set security headers for authenticated pages to prevent caching
		// This helps ensure sensitive content doesn't remain in browser history/cache
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")

		c.Next()
	}
}
