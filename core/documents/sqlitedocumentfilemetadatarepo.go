package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// SQLiteDocumentFileMetadataRepository implements DocumentFileMetadataRepository interface using SQLite.
type SQLiteDocumentFileMetadataRepository struct {
	db ccc.DBExecutor
}

// newSQLiteDocumentFileMetadataRepository creates a new SQLiteDocumentFileMetadataRepository instance.
func newSQLiteDocumentFileMetadataRepository(db ccc.DBExecutor) DocumentFileMetadataRepository {
	return &SQLiteDocumentFileMetadataRepository{
		db: db,
	}
}

// FindByDocumentFileId finds metadata by document file ID.
func (r *SQLiteDocumentFileMetadataRepository) FindByDocumentFileId(ctx context.Context, fileId string) (*DocumentFileMetadata, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.FindByDocumentFileId", "not implemented yet")
}

// FindByDocumentId finds all metadata for files in a document.
func (r *SQLiteDocumentFileMetadataRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFileMetadata, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.FindByDocumentId", "not implemented yet")
}

// Add adds new document file metadata.
func (r *SQLiteDocumentFileMetadataRepository) Add(ctx context.Context, metadata *DocumentFileMetadata) error {
	// TODO: Implement database insert
	return ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.Add", "not implemented yet")
}

// Update updates existing document file metadata.
func (r *SQLiteDocumentFileMetadataRepository) Update(ctx context.Context, metadata *DocumentFileMetadata) error {
	// TODO: Implement database update
	return ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.Update", "not implemented yet")
}

// Delete deletes document file metadata by file ID.
func (r *SQLiteDocumentFileMetadataRepository) Delete(ctx context.Context, fileId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.Delete", "not implemented yet")
}

// DeleteByDocumentId deletes all metadata for files in a document.
func (r *SQLiteDocumentFileMetadataRepository) DeleteByDocumentId(ctx context.Context, documentId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentFileMetadataRepository.DeleteByDocumentId", "not implemented yet")
}
