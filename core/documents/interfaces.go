package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// ID Generators
type DocumentIdGenerator interface {
	GenerateDocumentId() string
}

type DocumentFileIdGenerator interface {
	GenerateDocumentFileId() string
}

type TagIdGenerator interface {
	GenerateTagId() string
}

// Core Repository Interfaces - Simple CRUD operations only
type DocumentRepository interface {
	FindById(ctx context.Context, documentId string) (*Document, error)
	FindByUserId(ctx context.Context, userId string) ([]*Document, error)
	Add(ctx context.Context, document *Document) error
	Update(ctx context.Context, document *Document) error
	Delete(ctx context.Context, documentId string) error
	GetFileCountByDocumentId(ctx context.Context, documentId string) (int, error)
}

type DocumentFileRepository interface {
	FindById(ctx context.Context, fileId string) (*DocumentFile, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFile, error)
	Add(ctx context.Context, file *DocumentFile) error
	Update(ctx context.Context, file *DocumentFile) error
	Delete(ctx context.Context, fileId string) error
	DeleteByDocumentId(ctx context.Context, documentId string) error
}

type DocumentFileMetadataRepository interface {
	FindByDocumentFileId(ctx context.Context, fileId string) (*DocumentFileMetadata, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFileMetadata, error)
	Add(ctx context.Context, metadata *DocumentFileMetadata) error
	Update(ctx context.Context, metadata *DocumentFileMetadata) error
	Delete(ctx context.Context, fileId string) error
	DeleteByDocumentId(ctx context.Context, documentId string) error
}

type TagRepository interface {
	FindById(ctx context.Context, tagId string) (*Tag, error)
	FindByUserId(ctx context.Context, userId string) ([]*Tag, error)
	FindByDocumentId(ctx context.Context, documentId string) ([]*Tag, error)
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
	ExtractText(ctx context.Context, imageData []byte) (text string, confidence float32, err error)
}

// Document Search DAO - focused data access for search operations
type DocumentSearchDAO interface {
	SearchDocumentsByText(ctx context.Context, userId, searchTerm string, filters SearchFilters) ([]*DocumentSearchResult, error)
	IndexDocument(ctx context.Context, documentId string, searchableText string) error
	RemoveFromIndex(ctx context.Context, documentId string) error
	UpdateIndex(ctx context.Context, documentId string, searchableText string) error
}

// Document Search Engine interface
type DocumentSearchEngine interface {
	SearchDocuments(ctx context.Context, userId, searchTerm string, filters SearchFilters, dataProtector dataprotection.DataProtector) ([]*DocumentSearchResult, error)
}

// High-level Document Manager - consumer-facing service
type DocumentManager interface {
	// Document operations
	CreateDocument(ctx context.Context, userId string, request CreateDocumentRequest, dataProtector dataprotection.DataProtector) (*DocumentDto, error)
	GetDocument(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) (*DocumentDto, error)
	GetDocuments(ctx context.Context, userId string, request GetDocumentsRequest, dataProtector dataprotection.DataProtector) (*PaginatedDocumentResponse, error)
	UpdateDocument(ctx context.Context, userId, documentId string, request UpdateDocumentRequest, dataProtector dataprotection.DataProtector) error
	DeleteDocument(ctx context.Context, userId, documentId string) error

	// File operations
	AddDocumentFile(ctx context.Context, userId, documentId string, request AddFileRequest, dataProtector dataprotection.DataProtector) (*DocumentFileDto, error)
	GetDocumentFiles(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) ([]*DocumentFileDto, error)
	GetDocumentFile(ctx context.Context, userId, documentId, fileId string, dataProtector dataprotection.DataProtector) (*DocumentFileDto, error)
	DeleteDocumentFile(ctx context.Context, userId, documentId, fileId string) error

	// Document tagging operations (managing tags on documents)
	TagDocument(ctx context.Context, userId, documentId, tagId string) error
	UntagDocument(ctx context.Context, userId, documentId, tagId string) error
	GetDocumentTags(ctx context.Context, userId, documentId string) ([]*TagDto, error)
}

// Tag Manager - dedicated service for tag CRUD operations
type TagManager interface {
	CreateTag(ctx context.Context, userId string, request CreateTagRequest) (*TagDto, error)
	GetTag(ctx context.Context, userId, tagId string) (*TagDto, error)
	GetUserTags(ctx context.Context, userId string) ([]*TagDto, error)
	UpdateTag(ctx context.Context, userId, tagId string, request UpdateTagRequest) error
	DeleteTag(ctx context.Context, userId, tagId string) error
}
