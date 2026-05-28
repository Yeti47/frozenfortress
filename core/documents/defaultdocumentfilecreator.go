package documents

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultDocumentFileCreator implements DocumentFileCreator interface
type DefaultDocumentFileCreator struct {
	fileIdGen           DocumentFileIdGenerator
	docProcessorFactory DocumentFileProcessorFactory
	logger              ccc.Logger
}

// NewDefaultDocumentFileCreator creates a new DefaultDocumentFileCreator instance
func NewDefaultDocumentFileCreator(
	fileIdGen DocumentFileIdGenerator,
	docProcessorFactory DocumentFileProcessorFactory,
	logger ccc.Logger,
) *DefaultDocumentFileCreator {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultDocumentFileCreator{
		fileIdGen:           fileIdGen,
		docProcessorFactory: docProcessorFactory,
		logger:              logger,
	}
}

// CreateDocumentFile handles the complete file creation workflow
func (c *DefaultDocumentFileCreator) CreateDocumentFile(
	ctx context.Context,
	uow DocumentUnitOfWork,
	request CreateFileRequest,
	dataProtector dataprotection.DataProtector,
	ocrDispatcher OCRDispatcher,
) (*DocumentFile, *DocumentFileMetadata, error) {
	// Validate the request
	if err := c.ValidateFileRequest(request); err != nil {
		return nil, nil, err
	}

	// Verify document exists and belongs to user
	document, err := uow.DocumentRepo().FindById(ctx, request.DocumentId)
	if err != nil {
		return nil, nil, ccc.NewDatabaseError("failed to find document", err)
	}
	if document == nil || document.UserId != request.UserId {
		return nil, nil, ccc.NewResourceNotFoundError("document", request.DocumentId)
	}

	// Generate file ID
	fileId := c.fileIdGen.GenerateId()

	// Sanitize the filename to prevent path traversal attacks.
	sanitizedFilename := filepath.Base(request.FileName)

	// Encrypt file name
	encryptedFileName, err := dataProtector.Protect(sanitizedFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt file name: %w", err)
	}

	// Encrypt file data
	encryptedFileData, err := dataProtector.Protect(string(request.FileData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt file data: %w", err)
	}

	now := time.Now()

	// Create DocumentFile entity
	documentFile := &DocumentFile{
		Id:          fileId,
		DocumentId:  request.DocumentId,
		FileName:    encryptedFileName,
		ContentType: request.ContentType,
		FileSize:    int64(len(request.FileData)),
		PageCount:   initialPageCount(request.ContentType),
		FileData:    []byte(encryptedFileData),
		CreatedAt:   now,
		ModifiedAt:  now,
	}

	// Process file for additional metadata and preview
	processor, err := c.docProcessorFactory.GetProcessor(request.ContentType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get processor for content type %s: %w", request.ContentType, err)
	}

	// Generate preview if the processor supports it
	var preview *DocumentFilePreview
	previewResult, err := processor.GeneratePreview(ctx, request.FileData)
	if err != nil {
		c.logger.Warn("Failed to generate preview", "error", err, "fileId", fileId, "contentType", request.ContentType)
		// Continue without preview - not critical for file creation
	} else if previewResult != nil {
		// Encrypt preview data if it exists
		var encryptedPreviewData string
		if len(previewResult.PreviewData) > 0 {
			encryptedPreviewData, err = dataProtector.Protect(string(previewResult.PreviewData))
			if err != nil {
				c.logger.Error("Failed to encrypt preview data", "error", err, "fileId", fileId)
				// Continue without preview
			} else {
				preview = &DocumentFilePreview{
					DocumentFileId: fileId,
					PreviewData:    []byte(encryptedPreviewData),
					PreviewType:    previewResult.PreviewType,
					Width:          previewResult.Width,
					Height:         previewResult.Height,
				}
			}
		} else if previewResult.PreviewType != "" {
			// For non-image previews (like PDF), just store the type without data
			preview = &DocumentFilePreview{
				DocumentFileId: fileId,
				PreviewData:    nil,
				PreviewType:    previewResult.PreviewType,
				Width:          previewResult.Width,
				Height:         previewResult.Height,
			}
		}
	}

	// Persist DocumentFile with preview if available
	if preview != nil {
		if err := uow.DocumentFileRepo().AddWithPreview(ctx, documentFile, preview); err != nil {
			return nil, nil, ccc.NewDatabaseError("failed to add document file with preview", err)
		}
	} else {
		if err := uow.DocumentFileRepo().Add(ctx, documentFile); err != nil {
			return nil, nil, ccc.NewDatabaseError("failed to add document file", err)
		}
	}

	startedAt := time.Now()
	documentFileMetadata := &DocumentFileMetadata{
		DocumentFileId: fileId,
		OcrStatus:      OcrStatusProcessing,
		OcrStartedAt:   &startedAt,
	}
	if err := uow.DocumentFileMetadataRepo().Add(ctx, documentFileMetadata); err != nil {
		return nil, nil, ccc.NewDatabaseError("failed to add document file metadata", err)
	}

	if ocrDispatcher != nil {
		ocrDispatcher.Enqueue(OCRDispatchRequest{
			DocumentFileId: fileId,
			ContentType:    request.ContentType,
			FileData:       request.FileData,
			Processor:      processor,
			DataProtector:  dataProtector,
			StartedAt:      startedAt,
		})
	}

	c.logger.Info("Successfully created document file", "fileId", fileId, "documentId", request.DocumentId, "fileName", request.FileName)

	return documentFile, documentFileMetadata, nil
}

func initialPageCount(contentType string) int {
	if strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return 1
	}
	return 0
}

// ValidateFileRequest performs basic validation on file request data
func (c *DefaultDocumentFileCreator) ValidateFileRequest(request CreateFileRequest) error {
	const maxFileNameLength = 255
	const maxFileSize = 100 * 1024 * 1024 // 100MB

	if request.UserId == "" {
		return ccc.NewInvalidInputErrorWithMessage("userId", "cannot be empty", "User ID is required")
	}
	if request.DocumentId == "" {
		return ccc.NewInvalidInputErrorWithMessage("documentId", "cannot be empty", "Document ID is required")
	}
	if request.FileName == "" {
		return ccc.NewInvalidInputErrorWithMessage("fileName", "cannot be empty", "File name is required")
	}
	if len(request.FileName) > maxFileNameLength {
		return ccc.NewInvalidInputErrorWithMessage("fileName", "exceeds maximum length", fmt.Sprintf("File name cannot be longer than %d characters", maxFileNameLength))
	}
	if request.ContentType == "" {
		return ccc.NewInvalidInputErrorWithMessage("contentType", "cannot be empty", "Content type is required")
	}
	if len(request.FileData) == 0 {
		return ccc.NewInvalidInputErrorWithMessage("fileData", "cannot be empty", "File data is required")
	}
	if len(request.FileData) > maxFileSize {
		return ccc.NewInvalidInputErrorWithMessage("fileData", "exceeds maximum size", fmt.Sprintf("File cannot be larger than %d bytes", maxFileSize))
	}

	return nil
}
