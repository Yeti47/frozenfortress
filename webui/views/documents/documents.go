package documents

import (
	"context"
	"fmt"
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

const (
	// MaxFileSizeMB defines the maximum allowed file size in MB for document uploads
	MaxFileSizeMB = 30
	// MaxFileSize defines the maximum allowed file size in bytes for document uploads
	MaxFileSize = MaxFileSizeMB * 1024 * 1024 // Convert MB to bytes
)

// getMaxFileSizeMB returns the maximum file size in MB as a string for display
func getMaxFileSizeMB() string {
	return fmt.Sprintf("%dMB", MaxFileSizeMB)
}

// DocumentServices aggregates document-related services for cleaner function signatures
type DocumentServices struct {
	DocumentManager     documents.DocumentManager
	DocumentFileManager documents.DocumentFileManager
	DocumentListService documents.DocumentListService
	TagManager          documents.TagManager
	NoteManager         documents.NoteManager
}

// RegisterRoutes registers the documents routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, documentServices DocumentServices, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Documents page route - protected by authentication
	router.GET("/documents", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDocumentsPage(c, signInManager, documentServices.DocumentListService, documentServices.TagManager, mekStore, encryptionService, logger)
	})

	// Create document routes - protected by authentication
	router.GET("/create-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleCreateDocumentPage(c, signInManager, logger)
	})
	router.POST("/create-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleCreateDocumentSubmit(c, signInManager, documentServices.DocumentManager, documentServices.TagManager, mekStore, encryptionService, logger)
	})

	// Edit document route - protected by authentication
	router.GET("/edit-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditDocumentPage(c, signInManager, documentServices.DocumentManager, documentServices.TagManager, mekStore, encryptionService, logger)
	})
	router.POST("/edit-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditDocumentSubmit(c, signInManager, documentServices.DocumentManager, documentServices.TagManager, mekStore, encryptionService, logger)
	})

	// View document route - protected by authentication
	router.GET("/view-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleViewDocumentPage(c, signInManager, documentServices.DocumentManager, mekStore, encryptionService, logger)
	})

	// API routes for document files - protected by authentication
	router.GET("/api/documents/:documentId/files", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleGetDocumentFiles(c, signInManager, documentServices.DocumentFileManager, mekStore, encryptionService, logger)
	})
	router.POST("/api/documents/:documentId/files", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleUploadDocumentFile(c, signInManager, documentServices.DocumentFileManager, mekStore, encryptionService, logger)
	})
	router.DELETE("/api/documents/:documentId/files/:fileId", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteDocumentFile(c, signInManager, documentServices.DocumentFileManager, logger)
	})
	router.GET("/api/documents/:documentId/files/:fileId/download", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDownloadDocumentFile(c, signInManager, documentServices.DocumentFileManager, mekStore, encryptionService, logger)
	})
	router.GET("/api/documents/:documentId/files/:fileId/view", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleViewDocumentFile(c, signInManager, documentServices.DocumentFileManager, mekStore, encryptionService, logger)
	})

	// API routes for document notes - protected by authentication
	router.GET("/api/documents/:documentId/notes", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleGetDocumentNotes(c, signInManager, documentServices.NoteManager, mekStore, encryptionService, logger)
	})
	router.POST("/api/documents/:documentId/notes", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleCreateDocumentNote(c, signInManager, documentServices.NoteManager, mekStore, encryptionService, logger)
	})
	router.PUT("/api/documents/:documentId/notes/:noteId", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleUpdateDocumentNote(c, signInManager, documentServices.NoteManager, mekStore, encryptionService, logger)
	})
	router.DELETE("/api/documents/:documentId/notes/:noteId", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteDocumentNote(c, signInManager, documentServices.NoteManager, logger)
	})

	// Delete document route - protected by authentication
	router.DELETE("/documents/:id", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteDocument(c, signInManager, documentServices.DocumentManager, logger)
	})
}

// handleDocumentsPage handles the documents management page with pagination, filtering, and sorting
func handleDocumentsPage(c *gin.Context, signInManager auth.SignInManager, documentListService documents.DocumentListService, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Parse URL parameters
	pageStr := c.DefaultQuery("page", "1")
	searchTerm := strings.TrimSpace(c.Query("searchTerm"))

	// Default sort: use relevance for search results, title for browsing
	defaultSort := "title"
	defaultSortAsc := "true"
	if searchTerm != "" {
		defaultSort = "relevance"
		defaultSortAsc = "false" // relevance should be descending (most relevant first)
	}
	sortBy := c.DefaultQuery("sortBy", defaultSort)
	sortAsc := c.DefaultQuery("sortAsc", defaultSortAsc) == "true"
	tagFilterStr := c.Query("tagIds")
	deepSearch := c.Query("deepSearch") == "true"

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

	// Prepare request for document list service
	documentListRequest := documents.DocumentListRequest{
		SearchTerm: searchTerm,
		DeepSearch: deepSearch,
		Filters:    filters,
		Page:       page,
		PageSize:   20, // Max 20 per page
		SortBy:     sortBy,
		SortAsc:    sortAsc,
	}

	// Get documents from document list service
	documentListResponse, err := documentListService.GetDocumentList(c.Request.Context(), user.Id, documentListRequest, dataProtector)
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
	if documentListResponse.PageSize > 0 {
		totalPages = (documentListResponse.TotalCount + documentListResponse.PageSize - 1) / documentListResponse.PageSize
	}

	// Prepare template data
	templateData := gin.H{
		"Title":          "Frozen Fortress - Documents",
		"Username":       user.UserName,
		"Version":        ccc.AppVersion,
		"Documents":      documentListResponse.Items,
		"TotalCount":     documentListResponse.TotalCount,
		"Page":           page,
		"TotalPages":     totalPages,
		"PageSize":       documentListResponse.PageSize,
		"SortBy":         sortBy,
		"SortAsc":        sortAsc,
		"TagIds":         tagIds,
		"AllTags":        allTags,
		"HasPrevious":    page > 1,
		"HasNext":        page < totalPages,
		"SuccessMessage": successMessage,
		"SearchTerm":     searchTerm,
		"DeepSearch":     deepSearch,
		"IsSearchResult": searchTerm != "",
	}

	// Render the documents template
	c.HTML(200, "documents.html", templateData)
}

// handleEditDocumentPage handles GET requests to the edit-document page
func handleEditDocumentPage(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Query("id")
	if documentId == "" {
		c.Redirect(302, "/documents")
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get the document
	document, err := documentManager.GetDocument(c.Request.Context(), user.Id, documentId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document for edit", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Get all tags for the dropdown
	allTags, err := tagManager.GetUserTags(c.Request.Context(), user.Id)
	if err != nil {
		logger.Error("Failed to get tags for user", "user_id", user.Id, "error", err)
		// Don't fail the page load if tags can't be loaded
		allTags = []*documents.TagDto{}
	}

	// Get document tags
	documentTags := document.Tags // Assuming the document has tags loaded

	templateData := gin.H{
		"Title":           "Frozen Fortress - Edit Document",
		"Username":        user.UserName,
		"Version":         ccc.AppVersion,
		"Document":        document,
		"AllTags":         allTags,
		"DocumentTags":    documentTags,
		"MaxFileSize":     MaxFileSize,
		"MaxFileSizeText": getMaxFileSizeMB(),
	}

	// Render the edit document template
	c.HTML(200, "edit-document.html", templateData)
}

// handleViewDocumentPage handles GET requests to the view-document page
func handleViewDocumentPage(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Query("id")
	if documentId == "" {
		c.Redirect(302, "/documents")
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get the document
	document, err := documentManager.GetDocument(c.Request.Context(), user.Id, documentId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document for view", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	templateData := gin.H{
		"Title":    "Frozen Fortress - View Document",
		"Username": user.UserName,
		"Version":  ccc.AppVersion,
		"Document": document,
	}

	// Check for success messages
	if c.Query("updated") == "1" {
		templateData["SuccessMessage"] = "Document updated successfully"
	}

	// Render the view document template
	c.HTML(200, "view-document.html", templateData)
}

// handleCreateDocumentPage handles GET requests to the create-document page
func handleCreateDocumentPage(c *gin.Context, signInManager auth.SignInManager, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	templateData := gin.H{
		"Title":           "Frozen Fortress - Create Document",
		"Username":        user.UserName,
		"Version":         ccc.AppVersion,
		"MaxFileSize":     MaxFileSize,
		"MaxFileSizeText": getMaxFileSizeMB(),
	}

	// Render the create document template
	c.HTML(200, "create-document.html", templateData)
}

// handleCreateDocumentSubmit handles POST requests to create a new document
func handleCreateDocumentSubmit(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	// Parse form data
	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	tagIdsStr := c.PostForm("tagIds")

	// Validate title (required)
	if title == "" {
		templateData := gin.H{
			"Title":           "Frozen Fortress - Create Document",
			"Username":        user.UserName,
			"Version":         ccc.AppVersion,
			"DocumentTitle":   title,
			"Description":     description,
			"ErrorMessage":    "Document title is required.",
			"MaxFileSize":     MaxFileSize,
			"MaxFileSizeText": getMaxFileSizeMB(),
		}
		c.HTML(400, "create-document.html", templateData)
		return
	}

	// Parse tag IDs
	var tagIds []string
	if tagIdsStr != "" {
		tagIds = strings.Split(tagIdsStr, ",")
		// Remove empty strings
		var validTagIds []string
		for _, id := range tagIds {
			if strings.TrimSpace(id) != "" {
				validTagIds = append(validTagIds, strings.TrimSpace(id))
			}
		}
		tagIds = validTagIds
	}

	// Handle file uploads
	form, err := c.MultipartForm()
	if err != nil {
		logger.Error("Failed to parse multipart form", "error", err)
		templateData := gin.H{
			"Title":           "Frozen Fortress - Create Document",
			"Username":        user.UserName,
			"Version":         ccc.AppVersion,
			"DocumentTitle":   title,
			"Description":     description,
			"ErrorMessage":    "Failed to process uploaded files.",
			"MaxFileSize":     MaxFileSize,
			"MaxFileSizeText": getMaxFileSizeMB(),
		}
		c.HTML(400, "create-document.html", templateData)
		return
	}

	files := form.File["files"]
	var addFileRequests []documents.AddFileRequest

	// Define allowed content types
	allowedContentTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"application/pdf": true,
	}

	for _, fileHeader := range files {
		// Get content type from header
		contentType := fileHeader.Header.Get("Content-Type")

		// Validate content type
		if !allowedContentTypes[contentType] {
			logger.Warn("Rejected file with unsupported content type",
				"filename", fileHeader.Filename,
				"content_type", contentType,
				"user_id", user.Id)
			templateData := gin.H{
				"Title":           "Frozen Fortress - Create Document",
				"Username":        user.UserName,
				"Version":         ccc.AppVersion,
				"ErrorMessage":    "File '" + fileHeader.Filename + "' has an unsupported format. Only PNG, JPG, JPEG, and PDF files are allowed.",
				"DocumentTitle":   title,
				"Description":     description,
				"MaxFileSize":     MaxFileSize,
				"MaxFileSizeText": getMaxFileSizeMB(),
			}
			c.HTML(400, "create-document.html", templateData)
			return
		}

		// Check file size limit
		if fileHeader.Size > MaxFileSize {
			logger.Warn("Rejected file that exceeds size limit",
				"filename", fileHeader.Filename,
				"size", fileHeader.Size,
				"max_size", MaxFileSize,
				"user_id", user.Id)
			templateData := gin.H{
				"Title":           "Frozen Fortress - Create Document",
				"Username":        user.UserName,
				"Version":         ccc.AppVersion,
				"ErrorMessage":    "File '" + fileHeader.Filename + "' is too large. Maximum file size is " + getMaxFileSizeMB() + ".",
				"DocumentTitle":   title,
				"Description":     description,
				"MaxFileSize":     MaxFileSize,
				"MaxFileSizeText": getMaxFileSizeMB(),
			}
			c.HTML(400, "create-document.html", templateData)
			return
		}

		// Read file content
		file, err := fileHeader.Open()
		if err != nil {
			logger.Error("Failed to open uploaded file", "filename", fileHeader.Filename, "error", err)
			continue
		}
		defer file.Close()

		fileData := make([]byte, fileHeader.Size)
		_, err = file.Read(fileData)
		if err != nil {
			logger.Error("Failed to read uploaded file", "filename", fileHeader.Filename, "error", err)
			continue
		}

		addFileRequests = append(addFileRequests, documents.AddFileRequest{
			FileName:    fileHeader.Filename,
			ContentType: contentType,
			FileData:    fileData,
		})
	}

	// Create document request
	createRequest := documents.CreateDocumentRequest{
		Title:       title,
		Description: description,
		TagIds:      tagIds,
		Files:       addFileRequests,
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)
	// Create document
	response, err := documentManager.CreateDocument(context.Background(), user.Id, createRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to create document", "user_id", user.Id, "title", title, "error", err)

		templateData := gin.H{
			"Title":           "Frozen Fortress - Create Document",
			"Username":        user.UserName,
			"Version":         ccc.AppVersion,
			"DocumentTitle":   title,
			"Description":     description,
			"MaxFileSize":     MaxFileSize,
			"MaxFileSizeText": getMaxFileSizeMB(),
		}

		if middleware.HandleErrorOnPage(c, err, "create-document.html", templateData, "ErrorMessage") {
			return
		}
	}

	logger.Info("Document created successfully", "user_id", user.Id, "document_id", response.DocumentId, "title", title)

	// Redirect to documents page with success message
	c.Redirect(302, "/documents?created=1")
}

// handleEditDocumentSubmit handles POST requests to update a document
func handleEditDocumentSubmit(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Query("id")
	if documentId == "" {
		c.Redirect(302, "/documents")
		return
	}

	// Check if this is a delete action
	if c.PostForm("action") == "delete" {
		handleDeleteDocument(c, signInManager, documentManager, logger)
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get form data
	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	tagIdsStr := c.PostForm("tagIds")

	// Validate input
	if title == "" {
		c.Redirect(302, "/edit-document?id="+documentId+"&error=title_required")
		return
	}

	// Parse tag IDs
	var tagIds []string
	if tagIdsStr != "" {
		tagIds = strings.Split(tagIdsStr, ",")
		// Clean up tag IDs
		var cleanTagIds []string
		for _, id := range tagIds {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				cleanTagIds = append(cleanTagIds, trimmed)
			}
		}
		tagIds = cleanTagIds
	}

	// Update document
	updateRequest := documents.UpdateDocumentRequest{
		Title:       title,
		Description: description,
		TagIds:      tagIds,
	}

	err = documentManager.UpdateDocument(c.Request.Context(), user.Id, documentId, updateRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to update document", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// TODO: Handle file uploads and deletions here
	// This would require implementing file management in DocumentFileManager

	logger.Info("Document updated successfully", "user_id", user.Id, "document_id", documentId)

	// Determine redirect destination based on returnTo parameter
	returnTo := c.Query("returnTo")
	if returnTo == "documents" {
		c.Redirect(302, "/documents?updated=1")
	} else {
		// Default: redirect to view page with success message
		c.Redirect(302, "/view-document?id="+documentId+"&updated=1")
	}
}

// handleDeleteDocument handles DELETE requests to delete a document
func handleDeleteDocument(c *gin.Context, signInManager auth.SignInManager, documentManager documents.DocumentManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		middleware.HandleErrorWithJson(c, err, "Authentication required")
		return
	}

	// Get document ID from URL
	documentId := c.Param("id")

	// Validate document ID
	if documentId == "" {
		c.JSON(400, gin.H{"error": "Document ID is required"})
		return
	}

	// Delete document
	err = documentManager.DeleteDocument(c.Request.Context(), user.Id, documentId)
	if middleware.HandleErrorWithJson(c, err, "Failed to delete document") {
		logger.Error("Failed to delete document", "user_id", user.Id, "document_id", documentId, "error", err)
		return
	}

	logger.Info("Document deleted successfully", "user_id", user.Id, "document_id", documentId)

	// Respond with success
	c.JSON(200, gin.H{"success": true, "message": "Document deleted successfully"})
}

// handleGetDocumentFiles handles GET requests to retrieve document file previews
func handleGetDocumentFiles(c *gin.Context, signInManager auth.SignInManager, documentFileManager documents.DocumentFileManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Param("documentId")
	if documentId == "" {
		c.JSON(400, gin.H{"error": "Document ID is required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get document file previews (without full file data for performance)
	files, err := documentFileManager.GetDocumentFilePreviews(c.Request.Context(), user.Id, documentId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document file previews", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Respond with file data
	c.JSON(200, gin.H{"success": true, "files": files})
}

// handleDownloadDocumentFile handles GET requests to download a document file
func handleDownloadDocumentFile(c *gin.Context, signInManager auth.SignInManager, documentFileManager documents.DocumentFileManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Param("documentId")
	fileId := c.Param("fileId")
	if documentId == "" || fileId == "" {
		c.JSON(400, gin.H{"error": "Document ID and File ID are required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get the file
	file, err := documentFileManager.GetDocumentFile(c.Request.Context(), user.Id, documentId, fileId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document file for download", "user_id", user.Id, "document_id", documentId, "file_id", fileId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Set headers for download
	c.Header("Content-Disposition", "attachment; filename="+file.FileName)
	c.Header("Content-Type", file.ContentType)
	c.Header("Content-Length", strconv.Itoa(len(file.FileData)))

	// Write file data to response
	c.Data(200, file.ContentType, file.FileData)
}

// handleViewDocumentFile handles GET requests to view a document file
func handleViewDocumentFile(c *gin.Context, signInManager auth.SignInManager, documentFileManager documents.DocumentFileManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user for display
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(302, "/login")
		return
	}

	documentId := c.Param("documentId")
	fileId := c.Param("fileId")
	if documentId == "" || fileId == "" {
		c.JSON(400, gin.H{"error": "Document ID and File ID are required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get the file
	file, err := documentFileManager.GetDocumentFile(c.Request.Context(), user.Id, documentId, fileId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document file for viewing", "user_id", user.Id, "document_id", documentId, "file_id", fileId, "error", err)
		if middleware.HandleError(c, err) {
			return
		}
	}

	// Set headers for viewing
	c.Header("Content-Type", file.ContentType)
	c.Header("Content-Length", strconv.Itoa(len(file.FileData)))

	// Write file data to response
	c.Data(200, file.ContentType, file.FileData)
}

// handleUploadDocumentFile handles POST requests to upload a new file to a document
func handleUploadDocumentFile(c *gin.Context, signInManager auth.SignInManager, documentFileManager documents.DocumentFileManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	if documentId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID is required"})
		return
	}

	// Parse uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		logger.Error("Failed to get uploaded file", "error", err)
		c.JSON(400, gin.H{"success": false, "error": "No file uploaded"})
		return
	}

	// Check file size limit
	if fileHeader.Size > MaxFileSize {
		logger.Warn("Rejected file that exceeds size limit",
			"filename", fileHeader.Filename,
			"size", fileHeader.Size,
			"max_size", MaxFileSize,
			"user_id", user.Id)
		c.JSON(400, gin.H{"success": false, "error": "File is too large. Maximum file size is " + getMaxFileSizeMB()})
		return
	}

	// Get content type
	contentType := fileHeader.Header.Get("Content-Type")

	// Define allowed content types
	allowedContentTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"application/pdf": true,
		"image/gif":       true,
		"text/plain":      true,
	}

	// Validate content type
	if !allowedContentTypes[contentType] {
		logger.Warn("Rejected file with unsupported content type",
			"filename", fileHeader.Filename,
			"content_type", contentType,
			"user_id", user.Id)
		c.JSON(400, gin.H{"success": false, "error": "Unsupported file type. Only images, PDFs, and text files are allowed"})
		return
	}

	// Read file content
	file, err := fileHeader.Open()
	if err != nil {
		logger.Error("Failed to open uploaded file", "filename", fileHeader.Filename, "error", err)
		c.JSON(500, gin.H{"success": false, "error": "Failed to read uploaded file"})
		return
	}
	defer file.Close()

	fileData := make([]byte, fileHeader.Size)
	_, err = file.Read(fileData)
	if err != nil {
		logger.Error("Failed to read uploaded file", "filename", fileHeader.Filename, "error", err)
		c.JSON(500, gin.H{"success": false, "error": "Failed to read uploaded file"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Create add file request
	addFileRequest := documents.AddFileRequest{
		FileName:    fileHeader.Filename,
		ContentType: contentType,
		FileData:    fileData,
	}

	// Add file to document
	addedFile, err := documentFileManager.AddDocumentFile(c.Request.Context(), user.Id, documentId, addFileRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to add file to document", "user_id", user.Id, "document_id", documentId, "filename", fileHeader.Filename, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to add file to document") {
			return
		}
	}

	logger.Info("File uploaded successfully", "user_id", user.Id, "document_id", documentId, "file_id", addedFile.Id, "filename", fileHeader.Filename)

	// Return success response with file info
	c.JSON(200, gin.H{
		"success": true,
		"message": "File uploaded successfully",
		"file":    addedFile,
	})
}

// handleDeleteDocumentFile handles DELETE requests to remove a file from a document
func handleDeleteDocumentFile(c *gin.Context, signInManager auth.SignInManager, documentFileManager documents.DocumentFileManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	fileId := c.Param("fileId")
	if documentId == "" || fileId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID and File ID are required"})
		return
	}

	// Delete the file
	err = documentFileManager.DeleteDocumentFile(c.Request.Context(), user.Id, documentId, fileId)
	if err != nil {
		logger.Error("Failed to delete document file", "user_id", user.Id, "document_id", documentId, "file_id", fileId, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to delete file") {
			return
		}
	}

	logger.Info("File deleted successfully", "user_id", user.Id, "document_id", documentId, "file_id", fileId)

	// Return success response
	c.JSON(200, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})
}

// handleGetDocumentNotes handles GET requests to retrieve document notes
func handleGetDocumentNotes(c *gin.Context, signInManager auth.SignInManager, noteManager documents.NoteManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	if documentId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID is required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Get document notes
	notes, err := noteManager.GetDocumentNotes(c.Request.Context(), user.Id, documentId, dataProtector)
	if err != nil {
		logger.Error("Failed to get document notes", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to get document notes") {
			return
		}
	}

	// Respond with notes data
	c.JSON(200, gin.H{"success": true, "notes": notes})
}

// handleCreateDocumentNote handles POST requests to create a new note for a document
func handleCreateDocumentNote(c *gin.Context, signInManager auth.SignInManager, noteManager documents.NoteManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	if documentId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID is required"})
		return
	}

	// Parse request body
	var requestBody struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	// Validate content
	if strings.TrimSpace(requestBody.Content) == "" {
		c.JSON(400, gin.H{"success": false, "error": "Note content is required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Create note request
	createRequest := documents.CreateNoteRequest{
		UserId:     user.Id,
		DocumentId: documentId,
		Content:    strings.TrimSpace(requestBody.Content),
	}

	// Create note
	response, err := noteManager.CreateNote(c.Request.Context(), createRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to create note", "user_id", user.Id, "document_id", documentId, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to create note") {
			return
		}
	}

	logger.Info("Note created successfully", "user_id", user.Id, "document_id", documentId, "note_id", response.NoteId)

	// Return success response with note info
	c.JSON(200, gin.H{
		"success": true,
		"message": "Note created successfully",
		"note":    response,
	})
}

// handleUpdateDocumentNote handles PUT requests to update an existing note
func handleUpdateDocumentNote(c *gin.Context, signInManager auth.SignInManager, noteManager documents.NoteManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	noteId := c.Param("noteId")
	if documentId == "" || noteId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID and Note ID are required"})
		return
	}

	// Parse request body
	var requestBody struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	// Validate content
	if strings.TrimSpace(requestBody.Content) == "" {
		c.JSON(400, gin.H{"success": false, "error": "Note content is required"})
		return
	}

	// Create data protector
	dataProtector := dataprotection.CreateMekDataProtectorForRequest(
		mekStore,
		encryptionService,
		c.Request,
	)

	// Update note request
	updateRequest := documents.UpdateNoteRequest{
		UserId:  user.Id,
		NoteId:  noteId,
		Content: strings.TrimSpace(requestBody.Content),
	}

	// Update note
	err = noteManager.UpdateNote(c.Request.Context(), updateRequest, dataProtector)
	if err != nil {
		logger.Error("Failed to update note", "user_id", user.Id, "document_id", documentId, "note_id", noteId, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to update note") {
			return
		}
	}

	logger.Info("Note updated successfully", "user_id", user.Id, "document_id", documentId, "note_id", noteId)

	// Return success response
	c.JSON(200, gin.H{
		"success": true,
		"message": "Note updated successfully",
	})
}

// handleDeleteDocumentNote handles DELETE requests to remove a note from a document
func handleDeleteDocumentNote(c *gin.Context, signInManager auth.SignInManager, noteManager documents.NoteManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	documentId := c.Param("documentId")
	noteId := c.Param("noteId")
	if documentId == "" || noteId == "" {
		c.JSON(400, gin.H{"success": false, "error": "Document ID and Note ID are required"})
		return
	}

	// Delete the note
	err = noteManager.DeleteNote(c.Request.Context(), user.Id, noteId)
	if err != nil {
		logger.Error("Failed to delete note", "user_id", user.Id, "document_id", documentId, "note_id", noteId, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to delete note") {
			return
		}
	}

	logger.Info("Note deleted successfully", "user_id", user.Id, "document_id", documentId, "note_id", noteId)

	// Return success response
	c.JSON(200, gin.H{
		"success": true,
		"message": "Note deleted successfully",
	})
}
