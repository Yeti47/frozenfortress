package documents

import (
	"context"
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
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, documentManager documents.DocumentManager, documentFileManager documents.DocumentFileManager, tagManager documents.TagManager, mekStore auth.MekStore, encryptionService encryption.EncryptionService, logger ccc.Logger) {
	// Documents page route - protected by authentication
	router.GET("/documents", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDocumentsPage(c, signInManager, documentManager, tagManager, mekStore, encryptionService, logger)
	})

	// Create document routes - protected by authentication
	router.GET("/create-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleCreateDocumentPage(c, signInManager, logger)
	})
	router.POST("/create-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleCreateDocumentSubmit(c, signInManager, documentManager, tagManager, mekStore, encryptionService, logger)
	})

	// Edit document route - protected by authentication
	router.GET("/edit-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditDocumentPage(c, signInManager, documentManager, tagManager, mekStore, encryptionService, logger)
	})
	router.POST("/edit-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditDocumentSubmit(c, signInManager, documentManager, tagManager, mekStore, encryptionService, logger)
	})

	// View document route - protected by authentication
	router.GET("/view-document", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleViewDocumentPage(c, signInManager, documentManager, mekStore, encryptionService, logger)
	})

	// API routes for document files - protected by authentication
	router.GET("/api/documents/:documentId/files", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleGetDocumentFiles(c, signInManager, documentFileManager, mekStore, encryptionService, logger)
	})
	router.POST("/api/documents/:documentId/files", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleUploadDocumentFile(c, signInManager, documentFileManager, mekStore, encryptionService, logger)
	})
	router.DELETE("/api/documents/:documentId/files/:fileId", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteDocumentFile(c, signInManager, documentFileManager, logger)
	})
	router.GET("/api/documents/:documentId/files/:fileId/download", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDownloadDocumentFile(c, signInManager, documentFileManager, mekStore, encryptionService, logger)
	})
	router.GET("/api/documents/:documentId/files/:fileId/view", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleViewDocumentFile(c, signInManager, documentFileManager, mekStore, encryptionService, logger)
	})

	// Delete document route - protected by authentication
	router.DELETE("/documents/:id", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteDocument(c, signInManager, documentManager, logger)
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
		"Title":        "Frozen Fortress - Edit Document",
		"Username":     user.UserName,
		"Version":      "1.0.0",
		"Document":     document,
		"AllTags":      allTags,
		"DocumentTags": documentTags,
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
		"Version":  "1.0.0",
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
		"Title":    "Frozen Fortress - Create Document",
		"Username": user.UserName,
		"Version":  "1.0.0",
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
			"Title":         "Frozen Fortress - Create Document",
			"Username":      user.UserName,
			"Version":       "1.0.0",
			"DocumentTitle": title,
			"Description":   description,
			"ErrorMessage":  "Document title is required.",
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
			"Title":         "Frozen Fortress - Create Document",
			"Username":      user.UserName,
			"Version":       "1.0.0",
			"DocumentTitle": title,
			"Description":   description,
			"ErrorMessage":  "Failed to process uploaded files.",
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
				"Title":         "Frozen Fortress - Create Document",
				"Username":      user.UserName,
				"Version":       "1.0.0",
				"ErrorMessage":  "File '" + fileHeader.Filename + "' has an unsupported format. Only PNG, JPG, JPEG, and PDF files are allowed.",
				"DocumentTitle": title,
				"Description":   description,
			}
			c.HTML(400, "create-document.html", templateData)
			return
		}

		// Check file size (10MB limit)
		const maxFileSize = 10 * 1024 * 1024 // 10MB
		if fileHeader.Size > maxFileSize {
			logger.Warn("Rejected file that exceeds size limit",
				"filename", fileHeader.Filename,
				"size", fileHeader.Size,
				"max_size", maxFileSize,
				"user_id", user.Id)

			templateData := gin.H{
				"Title":         "Frozen Fortress - Create Document",
				"Username":      user.UserName,
				"Version":       "1.0.0",
				"ErrorMessage":  "File '" + fileHeader.Filename + "' is too large. Maximum file size is 10MB.",
				"DocumentTitle": title,
				"Description":   description,
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
			"Title":         "Frozen Fortress - Create Document",
			"Username":      user.UserName,
			"Version":       "1.0.0",
			"DocumentTitle": title,
			"Description":   description,
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

	// Check file size (10MB limit)
	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if fileHeader.Size > maxFileSize {
		logger.Warn("Rejected file that exceeds size limit",
			"filename", fileHeader.Filename,
			"size", fileHeader.Size,
			"max_size", maxFileSize,
			"user_id", user.Id)
		c.JSON(400, gin.H{"success": false, "error": "File is too large. Maximum file size is 10MB"})
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
