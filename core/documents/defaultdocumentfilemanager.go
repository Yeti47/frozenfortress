package documents

import (
	"context"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultDocumentFileManager implements DocumentFileManager interface
type DefaultDocumentFileManager struct {
	uowFactory  DocumentUnitOfWorkFactory
	fileCreator DocumentFileCreator
	logger      ccc.Logger
}

// NewDefaultDocumentFileManager creates a new DefaultDocumentFileManager instance
func NewDefaultDocumentFileManager(
	uowFactory DocumentUnitOfWorkFactory,
	fileCreator DocumentFileCreator,
	logger ccc.Logger,
) *DefaultDocumentFileManager {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultDocumentFileManager{
		uowFactory:  uowFactory,
		fileCreator: fileCreator,
		logger:      logger,
	}
}

// AddDocumentFile adds a new file to an existing document
func (m *DefaultDocumentFileManager) AddDocumentFile(
	ctx context.Context,
	userId, documentId string,
	request AddFileRequest,
	dataProtector dataprotection.DataProtector,
) (*DocumentFileDto, error) {
	if err := m.validateAddFileRequest(request); err != nil {
		return nil, err
	}

	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	uow := m.uowFactory.Create()
	var createdFile *DocumentFile
	var createdMetadata *DocumentFileMetadata

	err := uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Verify document exists and belongs to user
		document, docErr := uow.DocumentRepo().FindById(ctx, documentId)
		if docErr != nil {
			return ccc.NewDatabaseError("failed to find document", docErr)
		}
		if document == nil || document.UserId != userId {
			return ccc.NewResourceNotFoundError("document", documentId)
		}

		// Map AddFileRequest to CreateFileRequest
		createFileReq := CreateFileRequest{
			UserId:      userId,
			DocumentId:  documentId,
			FileName:    request.FileName,
			ContentType: request.ContentType,
			FileData:    request.FileData,
		}

		// Create the file using the file creator
		var createErr error
		createdFile, createdMetadata, createErr = m.fileCreator.CreateDocumentFile(ctx, uow, createFileReq, dataProtector)
		if createErr != nil {
			return ccc.NewDatabaseError("failed to create document file", createErr)
		}

		// Update document's modified time
		document.ModifiedAt = time.Now()
		if updateErr := uow.DocumentRepo().Update(ctx, document); updateErr != nil {
			return ccc.NewDatabaseError("failed to update document modified time", updateErr)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build and return the DTO
	return m.buildDocumentFileDto(createdFile, createdMetadata, dataProtector), nil
}

// GetDocumentFiles retrieves all files for a document
func (m *DefaultDocumentFileManager) GetDocumentFiles(
	ctx context.Context,
	userId, documentId string,
	dataProtector dataprotection.DataProtector,
) ([]*DocumentFileDto, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	uow := m.uowFactory.Create()

	// Verify document exists and belongs to user
	document, err := uow.DocumentRepo().FindById(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document", err)
	}
	if document == nil || document.UserId != userId {
		return nil, ccc.NewResourceNotFoundError("document", documentId)
	}

	// Get all files for the document
	files, err := uow.DocumentFileRepo().FindByDocumentId(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document files", err)
	}

	// Get metadata for all files
	metadata, err := uow.DocumentFileMetadataRepo().FindByDocumentId(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document file metadata", err)
	}

	// Create a map for quick metadata lookup
	metadataMap := make(map[string]*DocumentFileMetadata)
	for _, meta := range metadata {
		metadataMap[meta.DocumentFileId] = meta
	}

	// Build DTOs
	var fileDtos []*DocumentFileDto
	for _, file := range files {
		meta := metadataMap[file.Id]
		dto := m.buildDocumentFileDto(file, meta, dataProtector)
		fileDtos = append(fileDtos, dto)
	}

	return fileDtos, nil
}

// GetDocumentFile retrieves a single file by ID with full data
func (m *DefaultDocumentFileManager) GetDocumentFile(
	ctx context.Context,
	userId, documentId, fileId string,
	dataProtector dataprotection.DataProtector,
) (*DocumentFileDto, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}
	if fileId == "" {
		return nil, ccc.NewInvalidInputError("fileId", "cannot be empty")
	}

	uow := m.uowFactory.Create()

	// Verify document exists and belongs to user
	document, err := uow.DocumentRepo().FindById(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document", err)
	}
	if document == nil || document.UserId != userId {
		return nil, ccc.NewResourceNotFoundError("document", documentId)
	}

	// Get the specific file
	file, err := uow.DocumentFileRepo().FindById(ctx, fileId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document file", err)
	}
	if file == nil {
		return nil, ccc.NewResourceNotFoundError("document file", fileId)
	}
	if file.DocumentId != documentId {
		return nil, ccc.NewResourceNotFoundError("document file", fileId)
	}

	// Get metadata for the file
	metadata, err := uow.DocumentFileMetadataRepo().FindByDocumentFileId(ctx, fileId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document file metadata", err)
	}

	// Build and return the DTO with full data
	return m.buildDocumentFileDto(file, metadata, dataProtector), nil
}

// DeleteDocumentFile deletes a file from a document
func (m *DefaultDocumentFileManager) DeleteDocumentFile(
	ctx context.Context,
	userId, documentId, fileId string,
) error {
	if userId == "" {
		return ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return ccc.NewInvalidInputError("documentId", "cannot be empty")
	}
	if fileId == "" {
		return ccc.NewInvalidInputError("fileId", "cannot be empty")
	}

	uow := m.uowFactory.Create()

	err := uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Verify document exists and belongs to user
		document, err := uow.DocumentRepo().FindById(ctx, documentId)
		if err != nil {
			return ccc.NewDatabaseError("failed to find document", err)
		}
		if document == nil || document.UserId != userId {
			return ccc.NewResourceNotFoundError("document", documentId)
		}

		// Verify file exists and belongs to document
		file, err := uow.DocumentFileRepo().FindById(ctx, fileId)
		if err != nil {
			return ccc.NewDatabaseError("failed to find document file", err)
		}
		if file == nil {
			return ccc.NewResourceNotFoundError("document file", fileId)
		}
		if file.DocumentId != documentId {
			return ccc.NewResourceNotFoundError("document file", fileId)
		}

		// Delete file metadata
		if err := uow.DocumentFileMetadataRepo().Delete(ctx, fileId); err != nil {
			return ccc.NewDatabaseError("failed to delete document file metadata", err)
		}

		// Delete file preview
		if err := uow.DocumentFileRepo().DeletePreview(ctx, fileId); err != nil {
			return ccc.NewDatabaseError("failed to delete document file preview", err)
		}

		// Delete the file
		if err := uow.DocumentFileRepo().Delete(ctx, fileId); err != nil {
			return ccc.NewDatabaseError("failed to delete document file", err)
		}

		// Update document's modified time
		document.ModifiedAt = time.Now()
		if err := uow.DocumentRepo().Update(ctx, document); err != nil {
			return ccc.NewDatabaseError("failed to update document modified time", err)
		}

		return nil
	})

	return err
}

// validateAddFileRequest validates the add file request
func (m *DefaultDocumentFileManager) validateAddFileRequest(request AddFileRequest) error {
	if request.FileName == "" {
		return ccc.NewInvalidInputError("fileName", "cannot be empty")
	}
	if request.ContentType == "" {
		return ccc.NewInvalidInputError("contentType", "cannot be empty")
	}
	if len(request.FileData) == 0 {
		return ccc.NewInvalidInputError("fileData", "cannot be empty")
	}
	if len(request.FileData) > 100*1024*1024 { // 100MB limit
		return ccc.NewInvalidInputError("fileData", "file size exceeds 100MB limit")
	}
	return nil
}

// buildDocumentFileDto creates a DocumentFileDto from domain entities
func (m *DefaultDocumentFileManager) buildDocumentFileDto(
	file *DocumentFile,
	metadata *DocumentFileMetadata,
	dataProtector dataprotection.DataProtector,
) *DocumentFileDto {
	dto := &DocumentFileDto{
		Id:          file.Id,
		DocumentId:  file.DocumentId,
		ContentType: file.ContentType,
		FileSize:    file.FileSize,
		PageCount:   file.PageCount,
		CreatedAt:   file.CreatedAt,
		ModifiedAt:  file.ModifiedAt,
	}

	// Decrypt filename
	if decrypted, err := dataProtector.Unprotect(file.FileName); err == nil {
		dto.FileName = decrypted
	} else {
		m.logger.Warn("Failed to decrypt filename", "fileId", file.Id, "error", err)
		dto.FileName = "Encrypted File"
	}

	// Decrypt file data
	if decrypted, err := dataProtector.Unprotect(string(file.FileData)); err == nil {
		dto.FileData = []byte(decrypted)
	} else {
		m.logger.Warn("Failed to decrypt file data", "fileId", file.Id, "error", err)
		dto.FileData = nil
	}

	// Add metadata if available
	if metadata != nil {
		dto.Confidence = metadata.OcrConfidence

		// Decrypt extracted text
		if decrypted, err := dataProtector.Unprotect(metadata.ExtractedText); err == nil {
			dto.ExtractedText = decrypted
		} else {
			m.logger.Warn("Failed to decrypt extracted text", "fileId", file.Id, "error", err)
			dto.ExtractedText = ""
		}
	}

	return dto
}

// GetDocumentFilePreviews retrieves all files for a document with preview data but without full file content
func (m *DefaultDocumentFileManager) GetDocumentFilePreviews(
	ctx context.Context,
	userId, documentId string,
	dataProtector dataprotection.DataProtector,
) ([]*DocumentFilePreviewDto, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	uow := m.uowFactory.Create()

	// Verify document exists and belongs to user
	document, err := uow.DocumentRepo().FindById(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document", err)
	}
	if document == nil || document.UserId != userId {
		return nil, ccc.NewResourceNotFoundError("document", documentId)
	}

	// Get all file details (including metadata and preview) in one efficient query
	fileDetails, err := uow.DocumentFileRepo().FindDetailed(ctx, []string{documentId})
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document file details", err)
	}

	// Build preview DTOs
	var previewDtos []*DocumentFilePreviewDto
	for _, fileDetail := range fileDetails {
		dto := m.buildDocumentFilePreviewDto(fileDetail.File, fileDetail.Metadata, fileDetail.Preview, dataProtector)
		previewDtos = append(previewDtos, dto)
	}

	return previewDtos, nil
}

// buildDocumentFilePreviewDto creates a DocumentFilePreviewDto from domain entities
func (m *DefaultDocumentFileManager) buildDocumentFilePreviewDto(
	file *DocumentFile,
	metadata *DocumentFileMetadata,
	preview *DocumentFilePreview,
	dataProtector dataprotection.DataProtector,
) *DocumentFilePreviewDto {
	dto := &DocumentFilePreviewDto{
		Id:          file.Id,
		DocumentId:  file.DocumentId,
		ContentType: file.ContentType,
		FileSize:    file.FileSize,
		PageCount:   file.PageCount,
		CreatedAt:   file.CreatedAt,
		ModifiedAt:  file.ModifiedAt,
	}

	// Decrypt filename
	if decrypted, err := dataProtector.Unprotect(file.FileName); err == nil {
		dto.FileName = decrypted
	} else {
		m.logger.Warn("Failed to decrypt filename", "fileId", file.Id, "error", err)
		dto.FileName = "Encrypted File"
	}

	// Add metadata if available
	if metadata != nil {
		dto.Confidence = metadata.OcrConfidence

		// Decrypt extracted text
		if decrypted, err := dataProtector.Unprotect(metadata.ExtractedText); err == nil {
			dto.ExtractedText = decrypted
		} else {
			m.logger.Warn("Failed to decrypt extracted text", "fileId", file.Id, "error", err)
			dto.ExtractedText = ""
		}
	}

	// Add preview data if available
	if preview != nil {
		// Decrypt preview data
		if decrypted, err := dataProtector.Unprotect(string(preview.PreviewData)); err == nil {
			dto.Preview = &DocumentPreviewDto{
				DocumentFileId: preview.DocumentFileId,
				PreviewData:    []byte(decrypted),
				PreviewType:    preview.PreviewType,
				Width:          preview.Width,
				Height:         preview.Height,
			}
		} else {
			m.logger.Warn("Failed to decrypt preview data", "fileId", file.Id, "error", err)
			dto.Preview = nil
		}
	}

	return dto
}
