package tags

import (
	"context"
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers the tags routes with the provided Gin router.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, tagManager documents.TagManager, logger ccc.Logger) {
	// Tags page route - protected by authentication
	router.GET("/tags", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleTagsPage(c, signInManager, tagManager, logger)
	})

	// Edit tag page routes - protected by authentication
	router.GET("/edit-tag", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditTagPage(c, signInManager, tagManager, logger)
	})
	router.POST("/edit-tag", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleEditTagSubmit(c, signInManager, tagManager, logger)
	})

	// Delete tag route - protected by authentication
	router.DELETE("/tags/:id", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleDeleteTag(c, signInManager, tagManager, logger)
	})
}

// handleTagsPage handles the tags management page
func handleTagsPage(c *gin.Context, signInManager auth.SignInManager, tagManager documents.TagManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get success message from query parameters
	var successMessage string
	switch c.Query("success") {
	case "created":
		successMessage = "Tag created successfully!"
	case "updated":
		successMessage = "Tag updated successfully!"
	}

	// Handle deleted parameter for consistency with secrets page
	if c.Query("deleted") == "1" {
		successMessage = "Tag deleted successfully!"
	}

	// Get all tags for the user
	tags, err := tagManager.GetUserTags(context.Background(), user.Id)
	if middleware.HandleError(c, err) {
		return
	}

	// Render the tags page
	c.HTML(http.StatusOK, "tags.html", gin.H{
		"Title":          "Frozen Fortress - Tags",
		"Username":       user.UserName,
		"User":           user,
		"Tags":           tags,
		"SuccessMessage": successMessage,
	})
}

// handleDeleteTag handles deleting a tag
func handleDeleteTag(c *gin.Context, signInManager auth.SignInManager, tagManager documents.TagManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get tag ID from URL
	tagId := c.Param("id")
	if tagId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag ID is required"})
		return
	}

	// Delete the tag
	err = tagManager.DeleteTag(context.Background(), user.Id, tagId)
	if middleware.HandleErrorWithJson(c, err, "Failed to delete tag") {
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Tag deleted successfully"})
}

// handleEditTagPage handles the edit tag page (both create and edit)
func handleEditTagPage(c *gin.Context, signInManager auth.SignInManager, tagManager documents.TagManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	tagId := c.Query("id")

	// Initialize template data
	data := gin.H{
		"Title":    "Frozen Fortress - Edit Tag",
		"Username": user.UserName,
	}

	// If we have an ID, we're editing an existing tag
	if tagId != "" {
		tag, err := tagManager.GetTag(context.Background(), user.Id, tagId)
		if middleware.HandleErrorOnPage(c, err, "edit-tag.html", gin.H{}, "ErrorMessage") {
			return
		}

		data["TagId"] = tag.Id
		data["TagName"] = tag.Name
		data["TagColor"] = tag.Color
		data["CreatedAt"] = tag.CreatedAt.Format("2006-01-02 15:04:05")
		data["ModifiedAt"] = tag.ModifiedAt.Format("2006-01-02 15:04:05")
	}

	// Check for success message from redirect
	if successMsg := c.Query("success"); successMsg != "" {
		switch successMsg {
		case "created":
			data["SuccessMessage"] = "Tag created successfully!"
		case "updated":
			data["SuccessMessage"] = "Tag updated successfully!"
		}
	}

	c.HTML(http.StatusOK, "edit-tag.html", data)
}

// handleEditTagSubmit handles the form submission for creating/editing tags
func handleEditTagSubmit(c *gin.Context, signInManager auth.SignInManager, tagManager documents.TagManager, logger ccc.Logger) {
	// Get current user
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	tagId := c.PostForm("tagId")
	tagName := c.PostForm("tagName")
	tagColor := c.PostForm("tagColor")

	if tagId != "" {
		// Update existing tag
		updateRequest := documents.UpdateTagRequest{
			Name:  tagName,
			Color: tagColor,
		}

		err := tagManager.UpdateTag(context.Background(), user.Id, tagId, updateRequest)
		if middleware.HandleErrorOnPage(c, err, "edit-tag.html", gin.H{
			"TagId":    tagId,
			"TagName":  tagName,
			"TagColor": tagColor,
		}, "ErrorMessage") {
			return
		}

		// Redirect to tags page with success message
		c.Redirect(http.StatusSeeOther, "/tags?success=updated")
	} else {
		// Create new tag
		createRequest := documents.CreateTagRequest{
			Name:  tagName,
			Color: tagColor,
		}

		_, err := tagManager.CreateTag(context.Background(), user.Id, createRequest)
		if middleware.HandleErrorOnPage(c, err, "edit-tag.html", gin.H{
			"TagName":  tagName,
			"TagColor": tagColor,
		}, "ErrorMessage") {
			return
		}

		// Redirect to tags page with success message
		c.Redirect(http.StatusSeeOther, "/tags?success=created")
	}
}
