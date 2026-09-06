package scanhandoff

import (
	"io"
	"net/http"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/scanhandoff"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/middleware"
	documentsview "github.com/Yeti47/frozenfortress/frozenfortress/webui/views/documents"
	"github.com/gin-gonic/gin"
)

// startHandoffRequest is the JSON body for POST /api/scan-handoff/start.
type startHandoffRequest struct {
	Key string `json:"key"`
}

// RegisterRoutes registers the scan-handoff routes with the provided Gin router.
//
// The upload route is deliberately NOT behind AuthMiddleware: it's called by the
// companion app, which has no browser session/cookie. Its handoff token is the
// credential instead - see core/scanhandoff for the full security reasoning.
func RegisterRoutes(router *gin.Engine, signInManager auth.SignInManager, handoffService scanhandoff.ScanHandoffService, logger ccc.Logger) {
	router.POST("/api/scan-handoff/start", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleStartHandoff(c, signInManager, handoffService, logger)
	})
	router.POST("/api/scan-handoff/:token/upload", func(c *gin.Context) {
		handleUploadScan(c, handoffService, logger)
	})
	router.GET("/api/scan-handoff/:token/status", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleHandoffStatus(c, signInManager, handoffService, logger)
	})
	router.GET("/api/scan-handoff/:token/file", middleware.AuthMiddleware(signInManager), func(c *gin.Context) {
		handleFetchScan(c, signInManager, handoffService, logger)
	})
}

// handleStartHandoff handles POST requests to start a new scan handoff.
func handleStartHandoff(c *gin.Context, signInManager auth.SignInManager, handoffService scanhandoff.ScanHandoffService, logger ccc.Logger) {
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	var req startHandoffRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
		c.JSON(400, gin.H{"success": false, "error": "A key is required"})
		return
	}

	token, err := handoffService.StartHandoff(c.Request.Context(), user.Id, req.Key)
	if err != nil {
		logger.Error("Failed to start scan handoff", "user_id", user.Id, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to start scan handoff") {
			return
		}
	}

	logger.Info("Scan handoff started", "user_id", user.Id, "token", token)

	c.JSON(200, gin.H{
		"success":          true,
		"token":            token,
		"expiresInSeconds": int(scanhandoff.HandoffTTL.Seconds()),
	})
}

// handleUploadScan handles POST requests from the companion app to upload an
// encrypted scan. Not authenticated by session - the token in the path is the
// credential (see RegisterRoutes).
func handleUploadScan(c *gin.Context, handoffService scanhandoff.ScanHandoffService, logger ccc.Logger) {
	token := c.Param("token")
	if token == "" {
		c.JSON(400, gin.H{"success": false, "error": "Token is required"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		logger.Error("Failed to get uploaded scan", "token", token, "error", err)
		c.JSON(400, gin.H{"success": false, "error": "No file uploaded"})
		return
	}

	if fileHeader.Size > documentsview.MaxFileSize {
		logger.Warn("Rejected scan upload that exceeds size limit", "token", token, "size", fileHeader.Size, "max_size", documentsview.MaxFileSize)
		c.JSON(400, gin.H{"success": false, "error": "File is too large"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		logger.Error("Failed to open uploaded scan", "token", token, "error", err)
		c.JSON(500, gin.H{"success": false, "error": "Failed to read uploaded file"})
		return
	}
	defer file.Close()

	cipherBlob, err := io.ReadAll(file)
	if err != nil {
		logger.Error("Failed to read uploaded scan", "token", token, "error", err)
		c.JSON(500, gin.H{"success": false, "error": "Failed to read uploaded file"})
		return
	}

	if err := handoffService.UploadScan(c.Request.Context(), token, fileHeader.Filename, cipherBlob); err != nil {
		logger.Error("Failed to upload scan", "token", token, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to upload scan") {
			return
		}
	}

	logger.Info("Scan uploaded", "token", token)

	c.JSON(200, gin.H{"success": true})
}

// handleHandoffStatus handles GET requests polling a handoff's state.
func handleHandoffStatus(c *gin.Context, signInManager auth.SignInManager, handoffService scanhandoff.ScanHandoffService, logger ccc.Logger) {
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(400, gin.H{"success": false, "error": "Token is required"})
		return
	}

	state, err := handoffService.GetStatus(c.Request.Context(), token, user.Id)
	if err != nil {
		if middleware.HandleErrorWithJson(c, err, "Failed to get scan handoff status") {
			return
		}
	}

	c.JSON(200, gin.H{"success": true, "state": string(state)})
}

// handleFetchScan handles GET requests to fetch and consume the decrypted scan.
func handleFetchScan(c *gin.Context, signInManager auth.SignInManager, handoffService scanhandoff.ScanHandoffService, logger ccc.Logger) {
	user, err := signInManager.GetCurrentUser(c.Request)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "error": "Authentication required"})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(400, gin.H{"success": false, "error": "Token is required"})
		return
	}

	fileName, plainData, err := handoffService.FetchAndConsume(c.Request.Context(), token, user.Id)
	if err != nil {
		logger.Error("Failed to fetch scan", "token", token, "user_id", user.Id, "error", err)
		if middleware.HandleErrorWithJson(c, err, "Failed to fetch scan") {
			return
		}
	}

	logger.Info("Scan fetched and consumed", "token", token, "user_id", user.Id)

	c.Header("X-Scan-Filename", fileName)
	c.Data(200, http.DetectContentType(plainData), plainData)
}
