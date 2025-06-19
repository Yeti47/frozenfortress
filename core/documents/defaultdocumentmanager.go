package documents

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultDocumentManager implements DocumentManager interface
type DefaultDocumentManager struct {
	uowFactory    DocumentUnitOfWorkFactory
	documentIdGen DocumentIdGenerator
	fileCreator   DocumentFileCreator
	logger        ccc.Logger
}

// NewDefaultDocumentManager creates a new DefaultDocumentManager instance
func NewDefaultDocumentManager(
	uowFactory DocumentUnitOfWorkFactory,
	documentIdGen DocumentIdGenerator,
	fileCreator DocumentFileCreator,
	logger ccc.Logger,
) *DefaultDocumentManager {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultDocumentManager{
		uowFactory:    uowFactory,
		documentIdGen: documentIdGen,
		fileCreator:   fileCreator,
		logger:        logger,
	}
}

// CreateDocument creates a new document
func (m *DefaultDocumentManager) CreateDocument(
	ctx context.Context,
	userId string,
	request CreateDocumentRequest,
	dataProtector dataprotection.DataProtector,
) (*CreateDocumentResponse, error) {
	if err := m.validateCreateDocumentRequest(request); err != nil {
		return nil, err
	}

	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}

	// Encrypt sensitive data
	encryptedTitle, err := dataProtector.Protect(request.Title)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt document title: %w", err)
	}

	encryptedDescription, err := dataProtector.Protect(request.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt document description: %w", err)
	}

	now := time.Now()
	documentId := m.documentIdGen.GenerateId()

	document := &Document{
		Id:          documentId,
		UserId:      userId,
		Title:       encryptedTitle,
		Description: encryptedDescription,
		CreatedAt:   now,
		ModifiedAt:  now,
	}

	uow := m.uowFactory.Create()
	err = uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Add the document
		if err := uow.DocumentRepo().Add(ctx, document); err != nil {
			return ccc.NewDatabaseError("failed to add document", err)
		}

		// Add document tags if provided
		if len(request.TagIds) > 0 {
			for _, tagId := range request.TagIds {
				// Verify tag exists and belongs to user
				tag, err := uow.TagRepo().FindById(ctx, tagId)
				if err != nil {
					return ccc.NewDatabaseError(fmt.Sprintf("failed to find tag %s", tagId), err)
				}
				if tag == nil {
					return ccc.NewResourceNotFoundError("tag", tagId)
				}
				if tag.UserId != userId {
					return ccc.NewUnauthorizedError("tag does not belong to user")
				}

				if err := uow.DocumentTagRepo().AddDocumentTag(ctx, documentId, tagId); err != nil {
					return ccc.NewDatabaseError("failed to add document tag", err)
				}
			}
		}

		// Add document files if provided
		if len(request.Files) > 0 {
			for _, fileRequest := range request.Files {
				// Map AddFileRequest to CreateFileRequest
				createFileReq := CreateFileRequest{
					UserId:      userId,
					DocumentId:  documentId,
					FileName:    fileRequest.FileName,
					ContentType: fileRequest.ContentType,
					FileData:    fileRequest.FileData,
				}

				_, _, err := m.fileCreator.CreateDocumentFile(ctx, uow, createFileReq, dataProtector)
				if err != nil {
					return ccc.NewDatabaseError("failed to create document file", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Return the document creation response
	return &CreateDocumentResponse{
		DocumentId: documentId,
	}, nil
}

// GetDocument retrieves a single document by ID
func (m *DefaultDocumentManager) GetDocument(
	ctx context.Context,
	userId, documentId string,
	dataProtector dataprotection.DataProtector,
) (*DocumentDto, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	// Read-only operations - no transaction needed
	uow := m.uowFactory.Create()

	// Find document
	document, err := uow.DocumentRepo().FindById(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document", err)
	}
	if document == nil {
		return nil, ccc.NewResourceNotFoundError("document", documentId)
	}
	if document.UserId != userId {
		return nil, ccc.NewUnauthorizedError("document does not belong to user")
	}

	// Get file count
	fileCount, err := uow.DocumentRepo().GetFileCountByDocumentId(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to get file count", err)
	}

	// Get tags
	tags, err := uow.TagRepo().FindByDocumentId(ctx, documentId)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to find document tags", err)
	}

	return m.buildDocumentDto(document, tags, fileCount, dataProtector)
}

// GetDocuments retrieves paginated documents for a user
func (m *DefaultDocumentManager) GetDocuments(
	ctx context.Context,
	userId string,
	request GetDocumentsRequest,
	dataProtector dataprotection.DataProtector,
) (*PaginatedDocumentResponse, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}

	// Validate and constrain page size
	if request.PageSize <= 0 {
		request.PageSize = 20 // Default page size
	}
	if request.PageSize > 100 {
		request.PageSize = 100 // Hard limit
	}

	if request.Page <= 0 {
		request.Page = 1
	}

	// Use the filters from the request
	filters := request.Filters

	// Get filtered documents with tags directly from repository (read-only operation)
	uow := m.uowFactory.Create()
	documentDetails, err := uow.DocumentRepo().FindDetailed(ctx, userId, filters)
	if err != nil {
		return nil, ccc.NewDatabaseError("failed to load detailed documents", err)
	}

	// Apply sorting directly on document details
	m.sortDocumentDetails(documentDetails, request.SortBy, request.SortAsc, dataProtector)

	// Calculate pagination
	totalCount := len(documentDetails)
	offset := (request.Page - 1) * request.PageSize
	end := offset + request.PageSize

	var pagedDetails []*DocumentDetails
	if offset >= totalCount {
		pagedDetails = []*DocumentDetails{} // Empty slice for out-of-range pages
	} else {
		if end > totalCount {
			end = totalCount
		}
		pagedDetails = documentDetails[offset:end]
	}

	// Build DTOs with full tag information
	documentDtos := make([]*DocumentDto, 0, len(pagedDetails))
	for _, detail := range pagedDetails {
		dto, err := m.buildDocumentDto(detail.Document, detail.Tags, detail.FileCount, dataProtector)
		if err != nil {
			m.logger.Error("Failed to build document DTO", "error", err, "documentId", detail.Document.Id)
			continue
		}
		documentDtos = append(documentDtos, dto)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(request.PageSize)))

	return &PaginatedDocumentResponse{
		Documents:  documentDtos,
		TotalCount: totalCount,
		Page:       request.Page,
		PageSize:   request.PageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateDocument updates an existing document
func (m *DefaultDocumentManager) UpdateDocument(
	ctx context.Context,
	userId, documentId string,
	request UpdateDocumentRequest,
	dataProtector dataprotection.DataProtector,
) error {
	if err := m.validateUpdateDocumentRequest(request); err != nil {
		return err
	}

	if userId == "" {
		return ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	// Encrypt sensitive data
	encryptedTitle, err := dataProtector.Protect(request.Title)
	if err != nil {
		return fmt.Errorf("failed to encrypt document title: %w", err)
	}

	encryptedDescription, err := dataProtector.Protect(request.Description)
	if err != nil {
		return fmt.Errorf("failed to encrypt document description: %w", err)
	}

	uow := m.uowFactory.Create()
	return uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Find existing document
		document, err := uow.DocumentRepo().FindById(ctx, documentId)
		if err != nil {
			return ccc.NewDatabaseError("failed to find document", err)
		}
		if document == nil {
			return ccc.NewResourceNotFoundError("document", documentId)
		}
		if document.UserId != userId {
			return ccc.NewUnauthorizedError("document does not belong to user")
		}

		// Update fields
		document.Title = encryptedTitle
		document.Description = encryptedDescription
		document.ModifiedAt = time.Now()

		// Save changes
		if err := uow.DocumentRepo().Update(ctx, document); err != nil {
			return ccc.NewDatabaseError("failed to update document", err)
		}

		return nil
	})
}

// DeleteDocument deletes a document and all its associated files
func (m *DefaultDocumentManager) DeleteDocument(ctx context.Context, userId, documentId string) error {
	if userId == "" {
		return ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	uow := m.uowFactory.Create()
	return uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Find document to verify ownership
		document, err := uow.DocumentRepo().FindById(ctx, documentId)
		if err != nil {
			return ccc.NewDatabaseError("failed to find document", err)
		}
		if document == nil {
			return ccc.NewResourceNotFoundError("document", documentId)
		}
		if document.UserId != userId {
			return ccc.NewUnauthorizedError("document does not belong to user")
		}

		// Delete document tags
		if err := uow.DocumentTagRepo().RemoveAllDocumentTags(ctx, documentId); err != nil {
			return ccc.NewDatabaseError("failed to remove document tags", err)
		}

		// Delete file metadata
		if err := uow.DocumentFileMetadataRepo().DeleteByDocumentId(ctx, documentId); err != nil {
			return ccc.NewDatabaseError("failed to delete file metadata", err)
		}

		// Delete files
		if err := uow.DocumentFileRepo().DeleteByDocumentId(ctx, documentId); err != nil {
			return ccc.NewDatabaseError("failed to delete document files", err)
		}

		// Delete document
		if err := uow.DocumentRepo().Delete(ctx, documentId); err != nil {
			return ccc.NewDatabaseError("failed to delete document", err)
		}

		return nil
	})
}

// Helper methods

func (m *DefaultDocumentManager) validateCreateDocumentRequest(request CreateDocumentRequest) error {
	const maxTitleLength = 200
	const maxDescriptionLength = 1000

	if request.Title == "" {
		return ccc.NewInvalidInputError("title", "cannot be empty")
	}
	if len(request.Title) > maxTitleLength {
		return ccc.NewInvalidInputErrorWithMessage("title", "exceeds maximum length", fmt.Sprintf("Title cannot be longer than %d characters", maxTitleLength))
	}
	if len(request.Description) > maxDescriptionLength {
		return ccc.NewInvalidInputErrorWithMessage("description", "exceeds maximum length", fmt.Sprintf("Description cannot be longer than %d characters", maxDescriptionLength))
	}

	return nil
}

func (m *DefaultDocumentManager) validateUpdateDocumentRequest(request UpdateDocumentRequest) error {
	const maxTitleLength = 200
	const maxDescriptionLength = 1000

	if request.Title == "" {
		return ccc.NewInvalidInputError("title", "cannot be empty")
	}
	if len(request.Title) > maxTitleLength {
		return ccc.NewInvalidInputErrorWithMessage("title", "exceeds maximum length", fmt.Sprintf("Title cannot be longer than %d characters", maxTitleLength))
	}
	if len(request.Description) > maxDescriptionLength {
		return ccc.NewInvalidInputErrorWithMessage("description", "exceeds maximum length", fmt.Sprintf("Description cannot be longer than %d characters", maxDescriptionLength))
	}

	return nil
}

func (m *DefaultDocumentManager) buildDocumentDto(document *Document, tags []*Tag, fileCount int, dataProtector dataprotection.DataProtector) (*DocumentDto, error) {
	// Decrypt sensitive data
	title, err := dataProtector.Unprotect(document.Title)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt document title: %w", err)
	}

	description, err := dataProtector.Unprotect(document.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt document description: %w", err)
	}

	// Build tag DTOs
	tagDtos := make([]*TagDto, 0, len(tags))
	for _, tag := range tags {
		tagDtos = append(tagDtos, &TagDto{
			Id:         tag.Id,
			Name:       tag.Name,
			Color:      tag.Color,
			CreatedAt:  tag.CreatedAt,
			ModifiedAt: tag.ModifiedAt,
		})
	}

	return &DocumentDto{
		Id:          document.Id,
		Title:       title,
		Description: description,
		FileCount:   fileCount,
		Tags:        tagDtos,
		CreatedAt:   document.CreatedAt,
		ModifiedAt:  document.ModifiedAt,
	}, nil
}

func (m *DefaultDocumentManager) sortDocumentDetails(documentDetails []*DocumentDetails, sortBy string, sortAsc bool, dataProtector dataprotection.DataProtector) {
	switch sortBy {
	case "title":
		m.sortDocumentDetailsByTitle(documentDetails, sortAsc, dataProtector)
	case "created_at":
		m.sortDocumentDetailsByCreatedAt(documentDetails, sortAsc)
	case "modified_at":
		m.sortDocumentDetailsByModifiedAt(documentDetails, sortAsc)
	default:
		// Default to modified_at descending
		m.sortDocumentDetailsByModifiedAt(documentDetails, false)
	}
}

func (m *DefaultDocumentManager) sortDocumentDetailsByTitle(documentDetails []*DocumentDetails, asc bool, dataProtector dataprotection.DataProtector) {
	sort.Slice(documentDetails, func(i, j int) bool {
		titleI, err := dataProtector.Unprotect(documentDetails[i].Document.Title)
		if err != nil {
			titleI = "" // Fallback
		}
		titleJ, err := dataProtector.Unprotect(documentDetails[j].Document.Title)
		if err != nil {
			titleJ = "" // Fallback
		}

		if asc {
			return strings.ToLower(titleI) < strings.ToLower(titleJ)
		}
		return strings.ToLower(titleI) > strings.ToLower(titleJ)
	})
}

func (m *DefaultDocumentManager) sortDocumentDetailsByCreatedAt(documentDetails []*DocumentDetails, asc bool) {
	sort.Slice(documentDetails, func(i, j int) bool {
		if asc {
			return documentDetails[i].Document.CreatedAt.Before(documentDetails[j].Document.CreatedAt)
		}
		return documentDetails[i].Document.CreatedAt.After(documentDetails[j].Document.CreatedAt)
	})
}

func (m *DefaultDocumentManager) sortDocumentDetailsByModifiedAt(documentDetails []*DocumentDetails, asc bool) {
	sort.Slice(documentDetails, func(i, j int) bool {
		if asc {
			return documentDetails[i].Document.ModifiedAt.Before(documentDetails[j].Document.ModifiedAt)
		}
		return documentDetails[i].Document.ModifiedAt.After(documentDetails[j].Document.ModifiedAt)
	})
}
