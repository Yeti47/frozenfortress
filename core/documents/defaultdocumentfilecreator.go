package documents

import (
	"context"
	"fmt"
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
	if document == nil {
		return nil, nil, ccc.NewResourceNotFoundError("document", request.DocumentId)
	}
	if document.UserId != request.UserId {
		return nil, nil, ccc.NewUnauthorizedError("document does not belong to user")
	}

	// Generate file ID
	fileId := c.fileIdGen.GenerateId()

	// Encrypt file name
	encryptedFileName, err := dataProtector.Protect(request.FileName)
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
		PageCount:   0, // Will be set by processor if applicable
		FileData:    []byte(encryptedFileData),
		CreatedAt:   now,
		ModifiedAt:  now,
	}

	// Process file for additional metadata (page count, etc.)
	processor, err := c.docProcessorFactory.GetProcessor(request.ContentType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get processor for content type %s: %w", request.ContentType, err)
	}

	_, _, pageCount, err := processor.ExtractText(ctx, request.FileData)
	if err != nil {
		c.logger.Warn("Failed to process file", "error", err, "fileId", fileId, "contentType", request.ContentType)
		// Continue without processing result - not critical for file creation
	} else {
		documentFile.PageCount = pageCount
	}

	// Persist DocumentFile
	if err := uow.DocumentFileRepo().Add(ctx, documentFile); err != nil {
		return nil, nil, ccc.NewDatabaseError("failed to add document file", err)
	}

	// Extract text using processor (handles OCR for applicable file types)
	var documentFileMetadata *DocumentFileMetadata
	extractedText, confidence, _, err := processor.ExtractText(ctx, request.FileData)
	if err != nil {
		c.logger.Warn("Failed to extract text from file", "error", err, "fileId", fileId)
		// Continue without text extraction - not critical for file creation
	} else if extractedText != "" {
		// Encrypt extracted text
		encryptedText, err := dataProtector.Protect(extractedText)
		if err != nil {
			c.logger.Error("Failed to encrypt extracted text", "error", err, "fileId", fileId)
			// Continue without storing text extraction result
		} else {
			documentFileMetadata = &DocumentFileMetadata{
				DocumentFileId: fileId,
				ExtractedText:  encryptedText,
				OcrConfidence:  confidence,
			}

			// Persist metadata
			if err := uow.DocumentFileMetadataRepo().Add(ctx, documentFileMetadata); err != nil {
				c.logger.Error("Failed to add document file metadata", "error", err, "fileId", fileId)
				// Return error since metadata persistence failed
				return nil, nil, ccc.NewDatabaseError("failed to add document file metadata", err)
			}
		}
	}

	c.logger.Info("Successfully created document file", "fileId", fileId, "documentId", request.DocumentId, "fileName", request.FileName)

	return documentFile, documentFileMetadata, nil
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
