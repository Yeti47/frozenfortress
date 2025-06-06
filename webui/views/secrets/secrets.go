package secrets

import (
	"strconv"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/secrets"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the secrets routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, secretManager secrets.SecretManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Home page route - protected by authentication - serves secrets management
	router.GET("/", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleSecretsPage(c, signInManager, secretManager, mekStore, encryptionService, logger)
	})

	// Edit secret routes - protected by authentication
	router.GET("/edit-secret", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditSecretPage(c, signInManager, secretManager, mekStore, encryptionService, logger)
	})

	router.POST("/edit-secret", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditSecretSubmit(c, signInManager, secretManager, mekStore, encryptionService, logger)
	})

	// Delete secret route - protected by authentication
	router.DELETE("/delete-secret/:id", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteSecret(c, signInManager, secretManager, logger)
	})
}

// handleSecretsPage handles the secrets management page with pagination, filtering, and sorting
func handleSecretsPage(c *gin.Context, signInManager auth.SignInManager, secretManager secrets.SecretManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Parse URL parameters
	searchTerm := c.Query("searchTerm")
	pageStr := c.DefaultQuery("page", "1")
	sortBy := c.DefaultQuery("sortBy", "Name")
	sortAsc := c.DefaultQuery("sortAsc", "true") == "true"

	// Check for success messages
	var successMessage string
	if c.Query("created") == "1" {
		successMessage = "Secret created successfully!"
	} else if c.Query("updated") == "1" {
		successMessage = "Secret updated successfully!"
	} else if c.Query("deleted") == "1" {
		successMessage = "Secret deleted successfully!"
	}

	// Parse page number
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Create MekDataProtector for this request
	dataProtector := secrets.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Prepare request for secret manager
	getSecretsRequest := secrets.GetSecretsRequest{
		Name:     searchTerm,
		PageSize: 20, // Max 20 per page as specified
		Page:     page,
		SortBy:   sortBy,
		SortAsc:  sortAsc,
	}

	// Get secrets from secret manager
	paginatedResponse, err := secretManager.GetSecrets(user.Id, getSecretsRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to get secrets for user", "user_id", user.Id, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Calculate pagination info
	totalPages := 1
	if paginatedResponse.PageSize > 0 {
		totalPages = (paginatedResponse.TotalCount + paginatedResponse.PageSize - 1) / paginatedResponse.PageSize
	}

	// Prepare template data
	templateData := gin.H{
		"Title":          "Frozen Fortress - Secrets",
		"Username":       user.UserName,
		"Version":        "1.0.0", // Could be passed via services
		"Secrets":        paginatedResponse.Secrets,
		"TotalCount":     paginatedResponse.TotalCount,
		"Page":           page,
		"TotalPages":     totalPages,
		"PageSize":       paginatedResponse.PageSize,
		"SearchTerm":     searchTerm,
		"SortBy":         sortBy,
		"SortAsc":        sortAsc,
		"HasPrevious":    page > 1,
		"HasNext":        page < totalPages,
		"SuccessMessage": successMessage,
	}

	// Render the secrets template
	c.HTML(200, "secrets.html", templateData)
}

// handleEditSecretPage handles GET requests to the edit-secret page
func handleEditSecretPage(c *gin.Context, signInManager auth.SignInManager, secretManager secrets.SecretManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Check if we're editing an existing secret (secretId query parameter)
	secretId := c.Query("id")

	templateData := gin.H{
		"Title":    "Frozen Fortress - Edit Secret",
		"Username": user.UserName,
		"Version":  "1.0.0",
	}

	if secretId != "" {
		// Editing existing secret - fetch its details
		templateData["SecretId"] = secretId

		// Create MekDataProtector for this request
		dataProtector := secrets.CreateMekDataProtectorForRequest(
			mekStore,
			encryptionService,
			c.Request,
		)

		// Get the secret details
		secretDto, err := secretManager.GetSecret(user.Id, secretId, dataProtector)
		if err != nil {
			logger.Error("Failed to get secret for editing", "user_id", user.Id, "secret_id", secretId, "error", err)
			templateData["ErrorMessage"] = "Failed to load secret details. Please try again."
		} else {
			templateData["SecretName"] = secretDto.Name
			templateData["SecretValue"] = secretDto.Value
			templateData["CreatedAt"] = secretDto.CreatedAt
			templateData["ModifiedAt"] = secretDto.ModifiedAt
		}
	}

	// Render the edit secret template
	c.HTML(200, "edit-secret.html", templateData)
}

// handleEditSecretSubmit handles POST requests to create or update secrets
func handleEditSecretSubmit(c *gin.Context, signInManager auth.SignInManager, secretManager secrets.SecretManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Parse form data
	secretId := c.PostForm("secretId")
	secretName := c.PostForm("secretName")
	secretValue := c.PostForm("secretValue")

	// Validate input
	if secretName == "" {
		renderEditSecretWithError(c, user, secretId, secretName, secretValue, "Secret name is required")
		return
	}
	if secretValue == "" {
		renderEditSecretWithError(c, user, secretId, secretName, secretValue, "Secret value is required")
		return
	}

	// Create MekDataProtector for this request
	dataProtector := secrets.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Prepare the upsert request
	request := secrets.UpsertSecretRequest{
		SecretName:  secretName,
		SecretValue: secretValue,
	}

	if secretId != "" {
		// Update existing secret
		success, err := secretManager.UpdateSecret(user.Id, secretId, request, dataProtector)
		if err != nil {
			logger.Error("Failed to update secret", "user_id", user.Id, "secret_id", secretId, "error", err)
			renderEditSecretWithError(c, user, secretId, secretName, secretValue, "Failed to update secret. Please try again.")
			return
		}
		if !success {
			renderEditSecretWithError(c, user, secretId, secretName, secretValue, "Failed to update secret. Please try again.")
			return
		}

		logger.Info("Secret updated successfully", "user_id", user.Id, "secret_id", secretId, "secret_name", secretName)

		// Redirect to secrets list with success message
		c.Redirect(302, "/?updated=1")
	} else {
		// Create new secret
		createResponse, err := secretManager.CreateSecret(user.Id, request, dataProtector)
		if err != nil {
			logger.Error("Failed to create secret", "user_id", user.Id, "secret_name", secretName, "error", err)
			renderEditSecretWithError(c, user, "", secretName, secretValue, "Failed to create secret. Please try again.")
			return
		}

		logger.Info("Secret created successfully", "user_id", user.Id, "secret_id", createResponse.SecretId, "secret_name", secretName)

		// Redirect to secrets list with success message
		c.Redirect(302, "/?created=1")
	}
}

// renderEditSecretWithError renders the edit secret page with an error message
func renderEditSecretWithError(c *gin.Context, user auth.UserDto, secretId, secretName, secretValue, errorMessage string) {
	templateData := gin.H{
		"Title":        "Frozen Fortress - Edit Secret",
		"Username":     user.UserName,
		"Version":      "1.0.0",
		"SecretId":     secretId,
		"SecretName":   secretName,
		"SecretValue":  secretValue,
		"ErrorMessage": errorMessage,
	}

	c.HTML(400, "edit-secret.html", templateData)
}

// handleDeleteSecret handles DELETE requests to delete a secret
func handleDeleteSecret(c *gin.Context, signInManager auth.SignInManager, secretManager secrets.SecretManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// Get secret ID from URL parameter
	secretId := c.Param("id")
	if secretId == "" {
		c.JSON(400, gin.H{"error": "Secret ID is required"})
		return
	}

	// Delete the secret
	success, err := secretManager.DeleteSecret(user.Id, secretId)
	if err != nil {
		logger.Error("Failed to delete secret", "user_id", user.Id, "secret_id", secretId, "error", err)
		c.JSON(500, gin.H{"error": "Failed to delete secret. Please try again."})
		return
	}

	if !success {
		logger.Warn("Secret deletion returned false", "user_id", user.Id, "secret_id", secretId)
		c.JSON(404, gin.H{"error": "Secret not found"})
		return
	}

	logger.Info("Secret deleted successfully", "user_id", user.Id, "secret_id", secretId)
	c.JSON(200, gin.H{"success": true, "message": "Secret deleted successfully"})
}
