package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// SQLiteDocumentFileRepository implements DocumentFileRepository interface using SQLite.
type SQLiteDocumentFileRepository struct {
	db ccc.DBExecutor
}

// newSQLiteDocumentFileRepository creates a new SQLiteDocumentFileRepository instance.
func newSQLiteDocumentFileRepository(db ccc.DBExecutor) DocumentFileRepository {
	return &SQLiteDocumentFileRepository{
		db: db,
	}
}

// FindById finds a document file by its ID.
func (r *SQLiteDocumentFileRepository) FindById(ctx context.Context, fileId string) (*DocumentFile, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentFileRepository.FindById", "not implemented yet")
}

// FindByDocumentId finds all files for a document.
func (r *SQLiteDocumentFileRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFile, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentFileRepository.FindByDocumentId", "not implemented yet")
}

// Add adds a new document file.
func (r *SQLiteDocumentFileRepository) Add(ctx context.Context, file *DocumentFile) error {
	// TODO: Implement database insert
	return ccc.NewOperationFailedError("SQLiteDocumentFileRepository.Add", "not implemented yet")
}

// Update updates an existing document file.
func (r *SQLiteDocumentFileRepository) Update(ctx context.Context, file *DocumentFile) error {
	// TODO: Implement database update
	return ccc.NewOperationFailedError("SQLiteDocumentFileRepository.Update", "not implemented yet")
}

// Delete deletes a document file by its ID.
func (r *SQLiteDocumentFileRepository) Delete(ctx context.Context, fileId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentFileRepository.Delete", "not implemented yet")
}

// DeleteByDocumentId deletes all files for a document.
func (r *SQLiteDocumentFileRepository) DeleteByDocumentId(ctx context.Context, documentId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentFileRepository.DeleteByDocumentId", "not implemented yet")
}
