package account

import (
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	"github.com/gin-gonic/gin"
)

type services struct {
	UserManager   auth.UserManager
	SignInManager auth.SignInManager
}

// RegisterRoutes registers all account-related routes
func RegisterRoutes(router *gin.Engine, userManager auth.UserManager, signInManager auth.SignInManager) {
	s := &services{
		UserManager:   userManager,
		SignInManager: signInManager,
	}

	accountGroup := router.Group("/account")
	accountGroup.Use(middleware.AuthMiddleware(signInManager))
	{
		accountGroup.GET("/", s.showAccountSettings)
		accountGroup.POST("/change-password", s.changePassword)
		accountGroup.POST("/generate-recovery-code", s.generateRecoveryCode)
		accountGroup.POST("/deactivate", s.deactivateAccount)
		accountGroup.POST("/delete", s.deleteAccount)
	}
}

// showAccountSettings displays the account settings page
func (s *services) showAccountSettings(c *gin.Context) {
	user, err := s.SignInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleError(c, err)
		return
	}

	if user.Id == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	c.HTML(http.StatusOK, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	})
}

// changePassword handles password change requests
func (s *services) changePassword(c *gin.Context) {
	user, err := s.SignInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleError(c, err)
		return
	}

	if user.Id == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate input
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":         "Account Settings",
			"Username":      user.UserName,
			"passwordError": "All password fields are required",
		})
		return
	}

	if newPassword != confirmPassword {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":         "Account Settings",
			"Username":      user.UserName,
			"passwordError": "New passwords do not match",
		})
		return
	}

	// Change password (UserManager will validate password requirements)
	request := auth.ChangePasswordRequest{
		UserId:      user.Id,
		OldPassword: currentPassword,
		NewPassword: newPassword,
	}

	success, err := s.UserManager.ChangePassword(request)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "passwordError") {
		return
	}

	if !success {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":         "Account Settings",
			"Username":      user.UserName,
			"passwordError": "Password change failed",
		})
		return
	}

	c.HTML(http.StatusOK, "account.html", gin.H{
		"Title":           "Account Settings",
		"Username":        user.UserName,
		"passwordSuccess": "Password changed successfully",
	})
}

// generateRecoveryCode handles recovery code generation requests
func (s *services) generateRecoveryCode(c *gin.Context) {
	user, err := s.SignInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleError(c, err)
		return
	}

	if user.Id == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	password := c.PostForm("password")
	if password == "" {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":         "Account Settings",
			"Username":      user.UserName,
			"RecoveryError": "Password is required to generate recovery code",
		})
		return
	}

	// Generate recovery code (password verification is handled by UserManager)
	request := auth.GenerateRecoveryCodeRequest{
		UserId:   user.Id,
		Password: password,
	}

	response, err := s.UserManager.GenerateRecoveryCode(request)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "RecoveryError") {
		return
	}

	c.HTML(http.StatusOK, "account.html", gin.H{
		"Title":           "Account Settings",
		"Username":        user.UserName,
		"RecoveryCode":    response.RecoveryCode,
		"RecoveryContext": "account",
		"RecoverySuccess": "Recovery code generated successfully. Please save it in a secure location.",
	})
}

// deactivateAccount handles account deactivation requests
func (s *services) deactivateAccount(c *gin.Context) {
	user, err := s.SignInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleError(c, err)
		return
	}

	if user.Id == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	password := c.PostForm("password")
	if password == "" {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":           "Account Settings",
			"Username":        user.UserName,
			"DeactivateError": "Password is required to deactivate account",
		})
		return
	}

	// Verify password before deactivating
	isValid, err := s.UserManager.VerifyPassword(user.Id, password)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "DeactivateError") {
		return
	}

	if !isValid {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":           "Account Settings",
			"Username":        user.UserName,
			"DeactivateError": "Invalid password",
		})
		return
	}

	// Deactivate account
	success, err := s.UserManager.DeactivateUser(user.Id)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "DeactivateError") {
		return
	}

	if !success {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":           "Account Settings",
			"Username":        user.UserName,
			"DeactivateError": "Failed to deactivate account",
		})
		return
	}

	// Redirect to login page after deactivation
	c.Redirect(http.StatusFound, "/login?message=Account deactivated successfully")
}

// deleteAccount handles account deletion requests
func (s *services) deleteAccount(c *gin.Context) {
	user, err := s.SignInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleError(c, err)
		return
	}

	if user.Id == "" {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	password := c.PostForm("password")
	confirmation := c.PostForm("confirmation")

	if password == "" {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":       "Account Settings",
			"Username":    user.UserName,
			"DeleteError": "Password is required to delete account",
		})
		return
	}

	if confirmation != "DELETE" {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":       "Account Settings",
			"Username":    user.UserName,
			"DeleteError": "Please type 'DELETE' to confirm account deletion",
		})
		return
	}

	// Verify password before deleting
	isValid, err := s.UserManager.VerifyPassword(user.Id, password)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "DeleteError") {
		return
	}

	if !isValid {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":       "Account Settings",
			"Username":    user.UserName,
			"DeleteError": "Invalid password",
		})
		return
	}

	// Delete account
	success, err := s.UserManager.DeleteUser(user.Id)
	if middleware.HandleErrorOnPage(c, err, "account.html", gin.H{
		"Title":    "Account Settings",
		"Username": user.UserName,
	}, "DeleteError") {
		return
	}

	if !success {
		c.HTML(http.StatusBadRequest, "account.html", gin.H{
			"Title":       "Account Settings",
			"Username":    user.UserName,
			"DeleteError": "Failed to delete account",
		})
		return
	}

	// Redirect to login page after deletion
	c.Redirect(http.StatusFound, "/login?message=Account deleted successfully")
}
