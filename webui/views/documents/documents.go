package documents

import (
	"strconv"
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the documents routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, documentManager documents.DocumentManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Documents page route - protected by authentication
	router.GET("/documents", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDocumentsPage(c, signInManager, documentManager, tagManager, mekStore, encryptionService, logger)
	})

	// Edit document route - protected by authentication (placeholder for now)
	router.GET("/edit-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditDocumentPage(c, signInManager, logger)
	})
}

// handleDocumentsPage handles the documents management page with pagination, filtering, and sorting
func handleDocumentsPage(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Parse URL parameters
	pageStr := c.DefaultQuery("page", "1")
	sortBy := c.DefaultQuery("sortBy", "title")
	sortAsc := c.DefaultQuery("sortAsc", "true") == "true"
	tagFilterStr := c.Query("tagIds")

	// Check for success messages
	var successMessage string
	if c.Query("created") == "1" {
		successMessage = "Document created successfully!"
	} else if c.Query("updated") == "1" {
		successMessage = "Document updated successfully!"
	} else if c.Query("deleted") == "1" {
		successMessage = "Document deleted successfully!"
	}

	// Parse page number
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Parse tag filters with 3-tag limit
	var tagIds []string
	if tagFilterStr != "" {
		tagIds = strings.Split(tagFilterStr, ",")
		// Trim whitespace and remove empty strings
		var cleanTagIds []string
		for _, id := range tagIds {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				cleanTagIds = append(cleanTagIds, trimmed)
			}
		}
		tagIds = cleanTagIds

		// Enforce 3-tag limit on backend
		if len(tagIds) > 3 {
			tagIds = tagIds[:3] // Take only first 3 tags
		}
	}

	// Create MekDataProtector for this request
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Prepare filters
	filters := documents.DocumentFilters{
		TagIds: tagIds,
	}

	// Prepare request for document manager
	getDocumentsRequest := documents.GetDocumentsRequest{
		Filters:  filters,
		Page:     page,
		PageSize: 20, // Max 20 per page
		SortBy:   sortBy,
		SortAsc:  sortAsc,
	}

	// Get documents from document manager
	paginatedResponse, err := documentManager.GetDocuments(c.Request.Context(), user.Id, getDocumentsRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to get documents for user", "user_id", user.Id, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Get all tags for filtering dropdown
	allTags, err := tagManager.GetUserTags(c.Request.Context(), user.Id)
	if err != nil {
		logger.Error("Failed to get tags for user", "user_id", user.Id, "error", err)
		// Don't fail the page load if tags can't be loaded
		allTags = []*documents.TagDto{}
	}

	// Calculate pagination info
	totalPages := 1
	if paginatedResponse.PageSize > 0 {
		totalPages = (paginatedResponse.TotalCount + paginatedResponse.PageSize - 1) / paginatedResponse.PageSize
	}

	// Prepare template data
	templateData := gin.H{
		"Title":          "Frozen Fortress - Documents",
		"Username":       user.UserName,
		"Version":        "1.0.0", // Could be passed via services
		"Documents":      paginatedResponse.Documents,
		"TotalCount":     paginatedResponse.TotalCount,
		"Page":           page,
		"TotalPages":     totalPages,
		"PageSize":       paginatedResponse.PageSize,
		"SortBy":         sortBy,
		"SortAsc":        sortAsc,
		"TagIds":         tagIds,
		"AllTags":        allTags,
		"HasPrevious":    page > 1,
		"HasNext":        page < totalPages,
		"SuccessMessage": successMessage,
	}

	// Render the documents template
	c.HTML(200, "documents.html", templateData)
}

// handleEditDocumentPage handles GET requests to the edit-document page (placeholder)
func handleEditDocumentPage(c *gin.Context, signInManager auth.SignInManager, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	templateData := gin.H{
		"Title":    "Frozen Fortress - Edit Document",
		"Username": user.UserName,
		"Version":  "1.0.0",
		"Message":  "Document editing will be implemented in a future update.",
	}

	// Render a simple placeholder template
	c.HTML(200, "edit-document.html", templateData)
}
