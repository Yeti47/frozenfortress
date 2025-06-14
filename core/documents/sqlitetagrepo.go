package documents

import (
	"context"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// SQLiteTagRepository implements TagRepository interface using SQLite.
type SQLiteTagRepository struct {
	db ccc.DBExecutor
}

// newSQLiteTagRepository creates a new SQLiteTagRepository instance.
func newSQLiteTagRepository(db ccc.DBExecutor) TagRepository {
	return &SQLiteTagRepository{
		db: db,
	}
}

// FindById finds a tag by its ID.
func (r *SQLiteTagRepository) FindById(ctx context.Context, tagId string) (*Tag, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteTagRepository.FindById", "not implemented yet")
}

// FindByUserId finds all tags for a user.
func (r *SQLiteTagRepository) FindByUserId(ctx context.Context, userId string) ([]*Tag, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteTagRepository.FindByUserId", "not implemented yet")
}

// FindByDocumentId finds all tags for a document.
func (r *SQLiteTagRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*Tag, error) {
	// TODO: Implement database query
	return nil, ccc.NewOperationFailedError("SQLiteTagRepository.FindByDocumentId", "not implemented yet")
}

// Add adds a new tag.
func (r *SQLiteTagRepository) Add(ctx context.Context, tag *Tag) error {
	// TODO: Implement database insert
	return ccc.NewOperationFailedError("SQLiteTagRepository.Add", "not implemented yet")
}

// Update updates an existing tag.
func (r *SQLiteTagRepository) Update(ctx context.Context, tag *Tag) error {
	// TODO: Implement database update
	return ccc.NewOperationFailedError("SQLiteTagRepository.Update", "not implemented yet")
}

// Delete deletes a tag by its ID.
func (r *SQLiteTagRepository) Delete(ctx context.Context, tagId string) error {
	// TODO: Implement database delete
	return ccc.NewOperationFailedError("SQLiteTagRepository.Delete", "not implemented yet")
}
