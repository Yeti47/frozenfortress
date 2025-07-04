package documents

import (
	"context"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// ID Generators
type DocumentIdGenerator interface {
	GenerateId() string
}

type DocumentFileIdGenerator interface {
	GenerateId() string
}

type TagIdGenerator interface {
	GenerateId() string
}

type NoteIdGenerator interface {
	GenerateId() string
}

// Core Repository Interfaces - Simple CRUD operations only
type DocumentRepository interface {
	FindById(ctx context.Context, documentId string) (*Document, error)
	FindByUserId(ctx context.Context, userId string) ([]*Document, error)
	FindDetailed(ctx context.Context, userId string, filters DocumentFilters) ([]*DocumentDetails, error)
	Add(ctx context.Context, document *Document) error
	Update(ctx context.Context, document *Document) error
	Delete(ctx context.Context, documentId string) error
	GetFileCountByDocumentId(ctx context.Context, documentId string) (int, error)
}

type DocumentFileRepository interface {
	FindById(ctx context.Context, fileId string) (*DocumentFile, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFile, error)
	FindDetailed(ctx context.Context, documentIds []string) ([]*DocumentFileDetails, error)
	Add(ctx context.Context, file *DocumentFile) error
	AddWithPreview(ctx context.Context, file *DocumentFile, preview *DocumentFilePreview) error
	Update(ctx context.Context, file *DocumentFile) error
	Delete(ctx context.Context, fileId string) error
	DeleteByDocumentId(ctx context.Context, documentId string) error
	GetPreview(ctx context.Context, documentFileId string) (*DocumentFilePreview, error)
	SetPreview(ctx context.Context, preview *DocumentFilePreview, modifiedAt time.Time) error
	DeletePreview(ctx context.Context, documentFileId string) error
	FindOldestPreviewsByDocumentIds(ctx context.Context, documentIds []string) (map[string]*DocumentFilePreview, error)
}

type DocumentFileMetadataRepository interface {
	FindByDocumentFileId(ctx context.Context, fileId string) (*DocumentFileMetadata, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFileMetadata, error)
	FindExtended(ctx context.Context, documentIds []string) ([]*ExtendedDocumentFileMetadata, error)
	Add(ctx context.Context, metadata *DocumentFileMetadata) error
	Update(ctx context.Context, metadata *DocumentFileMetadata) error
	Delete(ctx context.Context, fileId string) error
	DeleteByDocumentId(ctx context.Context, documentId string) error
}

type TagRepository interface {
	FindById(ctx context.Context, tagId string) (*Tag, error)
	FindByUserId(ctx context.Context, userId string) ([]*Tag, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*Tag, error)
	FindByNameForUser(ctx context.Context, userId, name string) (*Tag, error)
	Add(ctx context.Context, tag *Tag) error
	Update(ctx context.Context, tag *Tag) error
	Delete(ctx context.Context, tagId string) error
}

type DocumentTagRepository interface {
	AddDocumentTag(ctx context.Context, documentId, tagId string) error
	RemoveDocumentTag(ctx context.Context, documentId, tagId string) error
	RemoveAllDocumentTags(ctx context.Context, documentId string) error
	FindDocumentsByTagId(ctx context.Context, tagId string) ([]*Document, error)
}

type NoteRepository interface {
	FindById(ctx context.Context, noteId string) (*Note, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*Note, error)
	Add(ctx context.Context, note *Note) error
	Update(ctx context.Context, note *Note) error
	Delete(ctx context.Context, noteId string) error
	DeleteByDocumentId(ctx context.Context, documentId string) error
}

// Unit of Work for transaction management
type DocumentUnitOfWork interface {
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error

	// Repository access within transaction
	DocumentRepo() DocumentRepository
	DocumentFileRepo() DocumentFileRepository
	DocumentFileMetadataRepo() DocumentFileMetadataRepository
	TagRepo() TagRepository
	DocumentTagRepo() DocumentTagRepository
	NoteRepo() NoteRepository

	// Fluent transaction execution
	Execute(ctx context.Context, fn func(uow DocumentUnitOfWork) error) error
	ExecuteWithResult(ctx context.Context, fn func(uow DocumentUnitOfWork) (interface{}, error)) (interface{}, error)
}

// DocumentUnitOfWorkFactory creates DocumentUnitOfWork instances
type DocumentUnitOfWorkFactory interface {
	Create() DocumentUnitOfWork
}

// OCR Service interface
type OCRService interface {
	IsOcrEnabled() bool
	ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error)
}

// Document Search Engine interface
type DocumentSearchEngine interface {
	SearchDocuments(ctx context.Context, userId string, request DocumentSearchRequest, dataProtector dataprotection.DataProtector) (*PaginatedDocumentSearchResponse, error)
}

// DocumentFileCreator is a domain service that handles the complete file creation workflow
// This ensures consistent behavior between DocumentManager.CreateDocument and DocumentFileManager.AddDocumentFile
type DocumentFileCreator interface {
	// CreateDocumentFile handles the complete file creation workflow including:
	// - File validation and processing
	// - File data encryption
	// - Text extraction (OCR) if applicable
	// - Preview generation if applicable
	// - Database persistence
	// This method operates within the provided UOW transaction scope to ensure atomicity
	CreateDocumentFile(
		ctx context.Context,
		uow DocumentUnitOfWork,
		request CreateFileRequest,
		dataProtector dataprotection.DataProtector,
	) (*DocumentFile, *DocumentFileMetadata, error)

	// ValidateFileRequest performs basic validation on file request data
	ValidateFileRequest(request CreateFileRequest) error
}

// High-level Document Manager - consumer-facing service.
// DocumentManager handles document CRUD operations
type DocumentManager interface {
	CreateDocument(ctx context.Context, userId string, request CreateDocumentRequest, dataProtector dataprotection.DataProtector) (*CreateDocumentResponse, error)
	GetDocument(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) (*DocumentDto, error)
	GetDocuments(ctx context.Context, userId string, request GetDocumentsRequest, dataProtector dataprotection.DataProtector) (*PaginatedDocumentResponse, error)
	UpdateDocument(ctx context.Context, userId, documentId string, request UpdateDocumentRequest, dataProtector dataprotection.DataProtector) error
	DeleteDocument(ctx context.Context, userId, documentId string) error
}

// High-level Document File Manager - consumer-facing service.
// DocumentFileManager handles file operations for documents
type DocumentFileManager interface {
	AddDocumentFile(ctx context.Context, userId, documentId string, request AddFileRequest, dataProtector dataprotection.DataProtector) (*DocumentFileDto, error)
	GetDocumentFiles(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) ([]*DocumentFileDto, error)
	GetDocumentFilePreviews(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) ([]*DocumentFilePreviewDto, error)
	GetDocumentFile(ctx context.Context, userId, documentId, fileId string, dataProtector dataprotection.DataProtector) (*DocumentFileDto, error)
	DeleteDocumentFile(ctx context.Context, userId, documentId, fileId string) error
}

// Tag Manager - dedicated service for tag CRUD operations
type TagManager interface {
	CreateTag(ctx context.Context, userId string, request CreateTagRequest) (*TagDto, error)
	GetTag(ctx context.Context, userId, tagId string) (*TagDto, error)
	GetUserTags(ctx context.Context, userId string) ([]*TagDto, error)
	UpdateTag(ctx context.Context, userId, tagId string, request UpdateTagRequest) error
	DeleteTag(ctx context.Context, userId, tagId string) error
}

// Note Manager - dedicated service for note CRUD operations
type NoteManager interface {
	CreateNote(ctx context.Context, request CreateNoteRequest, dataProtector dataprotection.DataProtector) (*CreateNoteResponse, error)
	GetDocumentNotes(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) ([]*NoteDto, error)
	UpdateNote(ctx context.Context, request UpdateNoteRequest, dataProtector dataprotection.DataProtector) error
	DeleteNote(ctx context.Context, userId, noteId string) error
}

// Document File Processing interfaces
type DocumentFileProcessor interface {
	// SupportsContentType checks if this processor can handle the given content type
	SupportsContentType(contentType string) bool

	// ExtractText extracts text from the file data and returns the text, confidence level, and page count
	ExtractText(ctx context.Context, fileData []byte) (text string, confidence float32, pageCount int, err error)

	// GeneratePreview creates a preview/thumbnail image for the file
	// Returns a PreviewGenerationResult containing the preview data (unencrypted), type, and dimensions
	GeneratePreview(ctx context.Context, fileData []byte) (*PreviewGenerationResult, error)
}

// DocumentFileProcessorFactory creates appropriate DocumentFileProcessor instances
type DocumentFileProcessorFactory interface {
	// GetProcessor returns a DocumentFileProcessor for the given content type
	// Returns an error if no processor is available for the content type
	GetProcessor(contentType string) (DocumentFileProcessor, error)
}

// DocumentListService provides a unified interface for listing documents with optional search functionality
// This service acts as a facade over DocumentManager and DocumentSearchEngine, delegating to the appropriate
// service based on whether a search term is provided
type DocumentListService interface {
	// GetDocumentList retrieves documents with optional search functionality
	// If request.SearchTerm is empty, it delegates to DocumentManager for regular filtering/pagination
	// If request.SearchTerm is provided, it delegates to DocumentSearchEngine for search functionality
	GetDocumentList(ctx context.Context, userId string, request DocumentListRequest, dataProtector dataprotection.DataProtector) (*DocumentListResponse, error)
}
