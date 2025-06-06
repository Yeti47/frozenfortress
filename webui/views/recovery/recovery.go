package recovery

import (
	"net/http"
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the recovery routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager) {

	// GET /recovery - Show recovery page
	router.GET("/recovery", func(c *gin.Context) {
		c.HTML(http.StatusOK, "recovery.html", gin.H{})
	})

	// POST /recovery - Handle recovery form submission
	router.POST("/recovery", func(c *gin.Context) {
		username := strings.TrimSpace(c.PostForm("username"))
		recoveryCode := strings.TrimSpace(c.PostForm("recoveryCode"))
		newPassword := c.PostForm("newPassword")
		confirmPassword := c.PostForm("confirmPassword")

		// Validate input
		if username == "" {
			c.HTML(http.StatusBadRequest, "recovery.html", gin.H{
				"ErrorMessage": "Username is required",
				"Username":     username,
				"RecoveryCode": recoveryCode,
			})
			return
		}

		if recoveryCode == "" {
			c.HTML(http.StatusBadRequest, "recovery.html", gin.H{
				"ErrorMessage": "Recovery code is required",
				"Username":     username,
				"RecoveryCode": recoveryCode,
			})
			return
		}

		if newPassword == "" {
			c.HTML(http.StatusBadRequest, "recovery.html", gin.H{
				"ErrorMessage": "New password is required",
				"Username":     username,
				"RecoveryCode": recoveryCode,
			})
			return
		}

		if newPassword != confirmPassword {
			c.HTML(http.StatusBadRequest, "recovery.html", gin.H{
				"ErrorMessage": "Passwords do not match",
				"Username":     username,
				"RecoveryCode": recoveryCode,
			})
			return
		}

		// Create recovery sign-in request
		request := auth.RecoverySignInRequest{
			UserName:     username,
			RecoveryCode: recoveryCode,
			NewPassword:  newPassword,
		}

		// Call SignInManager to handle recovery authentication
		response, err := signInManager.RecoverySignIn(c.Writer, c.Request, request)

		if middleware.HandleError(c, err) {
			return
		}

		if !response.Success {
			// Recovery failed - render recovery page with error message
			errorMessage := response.Error
			if errorMessage == "" {
				errorMessage = "Invalid username or recovery code"
			}

			c.HTML(http.StatusBadRequest, "recovery.html", gin.H{
				"ErrorMessage": errorMessage,
				"Username":     username,
				"RecoveryCode": recoveryCode,
			})
			return
		}

		// Recovery successful - display the new recovery code
		c.HTML(http.StatusOK, "recovery.html", gin.H{
			"SuccessMessage":  "Password recovery successful! Please save your new recovery code immediately.",
			"NewRecoveryCode": response.NewRecoveryCode,
			"Username":        response.User.UserName,
		})
	})
}
