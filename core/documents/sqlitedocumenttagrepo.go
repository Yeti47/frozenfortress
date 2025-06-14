package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// SQLiteDocumentTagRepository implements DocumentTagRepository interface using SQLite.
type SQLiteDocumentTagRepository struct {
	db ccc.DBExecutor
}

// newSQLiteDocumentTagRepository creates a new SQLiteDocumentTagRepository instance.
func newSQLiteDocumentTagRepository(db ccc.DBExecutor) DocumentTagRepository {
	return &SQLiteDocumentTagRepository{
		db: db,
	}
}

// AddDocumentTag adds a tag to a document.
func (r *SQLiteDocumentTagRepository) AddDocumentTag(ctx context.Context, documentId, tagId string) error {
	// TODO: Implement database insert
	return ccc.NewOperationFailedError("SQLiteDocumentTagRepository.AddDocumentTag", "not implemented yet")
}

// RemoveDocumentTag removes a tag from a document.
func (r *SQLiteDocumentTagRepository) RemoveDocumentTag(ctx context.Context, documentId, tagId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentTagRepository.RemoveDocumentTag", "not implemented yet")
}

// RemoveAllDocumentTags removes all tags from a document.
func (r *SQLiteDocumentTagRepository) RemoveAllDocumentTags(ctx context.Context, documentId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentTagRepository.RemoveAllDocumentTags", "not implemented yet")
}

// FindDocumentsByTagId finds all documents that have a specific tag.
func (r *SQLiteDocumentTagRepository) FindDocumentsByTagId(ctx context.Context, tagId string) ([]*Document, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentTagRepository.FindDocumentsByTagId", "not implemented yet")
}
