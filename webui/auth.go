package main

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

		c.Next()
	}
}
