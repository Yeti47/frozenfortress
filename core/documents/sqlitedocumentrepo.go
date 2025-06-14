package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// SQLiteDocumentRepository implements DocumentRepository interface using SQLite.
type SQLiteDocumentRepository struct {
	db ccc.DBExecutor
}

// newSQLiteDocumentRepository creates a new SQLiteDocumentRepository instance.
func newSQLiteDocumentRepository(db ccc.DBExecutor) DocumentRepository {
	return &SQLiteDocumentRepository{
		db: db,
	}
}

// FindById finds a document by its ID.
func (r *SQLiteDocumentRepository) FindById(ctx context.Context, documentId string) (*Document, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentRepository.FindById", "not implemented yet")
}

// FindByUserId finds all documents for a user.
func (r *SQLiteDocumentRepository) FindByUserId(ctx context.Context, userId string) ([]*Document, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteDocumentRepository.FindByUserId", "not implemented yet")
}

// Add adds a new document.
func (r *SQLiteDocumentRepository) Add(ctx context.Context, document *Document) error {
	// TODO: Implement database insert
	return ccc.NewOperationFailedError("SQLiteDocumentRepository.Add", "not implemented yet")
}

// Update updates an existing document.
func (r *SQLiteDocumentRepository) Update(ctx context.Context, document *Document) error {
	// TODO: Implement database update
	return ccc.NewOperationFailedError("SQLiteDocumentRepository.Update", "not implemented yet")
}

// Delete deletes a document by its ID.
func (r *SQLiteDocumentRepository) Delete(ctx context.Context, documentId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteDocumentRepository.Delete", "not implemented yet")
}

// GetFileCountByDocumentId returns the number of files for a document.
func (r *SQLiteDocumentRepository) GetFileCountByDocumentId(ctx context.Context, documentId string) (int, error) {
	// TODO: Implement database query
	return 0, ccc.NewOperationFailedError("SQLiteDocumentRepository.GetFileCountByDocumentId", "not implemented yet")
}
