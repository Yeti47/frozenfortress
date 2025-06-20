package documents

import (
	"context"
	"database/sql"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDocumentTagRepository implements DocumentTagRepository interface using SQLite.
type SQLiteDocumentTagRepository struct {
	db ccc.DBExecutor
}

// newSQLiteDocumentTagRepository creates a new SQLiteDocumentTagRepository instance.
func newSQLiteDocumentTagRepository(db ccc.DBExecutor) DocumentTagRepository {
	repo := &SQLiteDocumentTagRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
		}
	}

	return repo
}

// initializeTable creates the DocumentTag junction table if it doesn't exist
func (r *SQLiteDocumentTagRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS DocumentTag (
		DocumentId TEXT NOT NULL,
		TagId TEXT NOT NULL,
		CreatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (DocumentId, TagId)
	);
	CREATE INDEX IF NOT EXISTS idx_documenttag_documentid ON DocumentTag(DocumentId);
	CREATE INDEX IF NOT EXISTS idx_documenttag_tagid ON DocumentTag(TagId);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraints
	fkQuery1 := `
	ALTER TABLE DocumentTag ADD CONSTRAINT fk_documenttag_documentid 
	FOREIGN KEY (DocumentId) REFERENCES Document(Id) ON DELETE CASCADE;
	`
	_, fkErr1 := db.Exec(fkQuery1)
	if fkErr1 != nil {
		// Log or ignore the error - foreign key constraint is optional
	}

	fkQuery2 := `
	ALTER TABLE DocumentTag ADD CONSTRAINT fk_documenttag_tagid 
	FOREIGN KEY (TagId) REFERENCES Tag(Id) ON DELETE CASCADE;
	`
	_, fkErr2 := db.Exec(fkQuery2)
	if fkErr2 != nil {
		// Log or ignore the error - foreign key constraint is optional
	}

	return nil
}

// AddDocumentTag adds a tag to a document.
func (r *SQLiteDocumentTagRepository) AddDocumentTag(ctx context.Context, documentId, tagId string) error {
	query := `INSERT OR IGNORE INTO DocumentTag (DocumentId, TagId, CreatedAt) VALUES (?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(time.Now())

	_, err := r.db.ExecContext(ctx, query, documentId, tagId, createdAtStr)
	return err
}

// RemoveDocumentTag removes a tag from a document.
func (r *SQLiteDocumentTagRepository) RemoveDocumentTag(ctx context.Context, documentId, tagId string) error {
	query := `DELETE FROM DocumentTag WHERE DocumentId = ? AND TagId = ?`
	_, err := r.db.ExecContext(ctx, query, documentId, tagId)
	return err
}

// RemoveAllDocumentTags removes all tags from a document.
func (r *SQLiteDocumentTagRepository) RemoveAllDocumentTags(ctx context.Context, documentId string) error {
	query := `DELETE FROM DocumentTag WHERE DocumentId = ?`
	_, err := r.db.ExecContext(ctx, query, documentId)
	return err
}

// FindDocumentsByTagId finds all documents that have a specific tag.
func (r *SQLiteDocumentTagRepository) FindDocumentsByTagId(ctx context.Context, tagId string) ([]*Document, error) {
	query := `
	SELECT d.Id, d.UserId, d.Title, d.Description, d.CreatedAt, d.ModifiedAt 
	FROM Document d
	INNER JOIN DocumentTag dt ON d.Id = dt.DocumentId
	WHERE dt.TagId = ?
	ORDER BY d.ModifiedAt DESC`

	rows, err := r.db.QueryContext(ctx, query, tagId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documents []*Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		documents = append(documents, doc)
	}
	return documents, rows.Err()
}
