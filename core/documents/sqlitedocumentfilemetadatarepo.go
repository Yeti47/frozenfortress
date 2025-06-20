package documents

import (
	"context"
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDocumentFileMetadataRepository implements DocumentFileMetadataRepository interface using SQLite.
type SQLiteDocumentFileMetadataRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for DocumentFileMetadata table queries
	documentFileMetadataFieldList = `DocumentFileId, ExtractedText, OcrConfidence`
)

// newSQLiteDocumentFileMetadataRepository creates a new SQLiteDocumentFileMetadataRepository instance.
func newSQLiteDocumentFileMetadataRepository(db ccc.DBExecutor) DocumentFileMetadataRepository {
	repo := &SQLiteDocumentFileMetadataRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
		}
	}

	return repo
}

// initializeTable creates the DocumentFileMetadata table if it doesn't exist
func (r *SQLiteDocumentFileMetadataRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS DocumentFileMetadata (
		DocumentFileId TEXT PRIMARY KEY,
		ExtractedText TEXT,
		OcrConfidence REAL DEFAULT 0.0
	);
	CREATE INDEX IF NOT EXISTS idx_documentfilemetadata_confidence ON DocumentFileMetadata(OcrConfidence);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraint from DocumentFileMetadata.DocumentFileId to DocumentFile.Id
	fkQuery := `
	ALTER TABLE DocumentFileMetadata ADD CONSTRAINT fk_documentfilemetadata_fileid 
	FOREIGN KEY (DocumentFileId) REFERENCES DocumentFile(Id) ON DELETE CASCADE;
	`
	_, fkErr := db.Exec(fkQuery)
	if fkErr != nil {
		// Log or ignore the error - foreign key constraint is optional
	}

	return nil
}

// FindByDocumentFileId finds metadata by document file ID.
func (r *SQLiteDocumentFileMetadataRepository) FindByDocumentFileId(ctx context.Context, fileId string) (*DocumentFileMetadata, error) {
	query := `SELECT ` + documentFileMetadataFieldList + ` FROM DocumentFileMetadata WHERE DocumentFileId = ?`
	row := r.db.QueryRowContext(ctx, query, fileId)
	return scanDocumentFileMetadata(row)
}

// FindByDocumentId finds all metadata for files in a document.
func (r *SQLiteDocumentFileMetadataRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFileMetadata, error) {
	query := `
	SELECT dfm.DocumentFileId, dfm.ExtractedText, dfm.OcrConfidence 
	FROM DocumentFileMetadata dfm
	INNER JOIN DocumentFile df ON dfm.DocumentFileId = df.Id
	WHERE df.DocumentId = ?
	ORDER BY dfm.OcrConfidence DESC`

	rows, err := r.db.QueryContext(ctx, query, documentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metadata []*DocumentFileMetadata
	for rows.Next() {
		meta, err := scanDocumentFileMetadata(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		metadata = append(metadata, meta)
	}
	return metadata, rows.Err()
}

// Add adds new document file metadata.
func (r *SQLiteDocumentFileMetadataRepository) Add(ctx context.Context, metadata *DocumentFileMetadata) error {
	query := `INSERT INTO DocumentFileMetadata (` + documentFileMetadataFieldList + `) VALUES (?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		metadata.DocumentFileId,
		metadata.ExtractedText,
		metadata.OcrConfidence,
	)
	return err
}

// Update updates existing document file metadata.
func (r *SQLiteDocumentFileMetadataRepository) Update(ctx context.Context, metadata *DocumentFileMetadata) error {
	query := `UPDATE DocumentFileMetadata SET ExtractedText = ?, OcrConfidence = ? WHERE DocumentFileId = ?`

	_, err := r.db.ExecContext(ctx, query,
		metadata.ExtractedText,
		metadata.OcrConfidence,
		metadata.DocumentFileId,
	)
	return err
}

// Delete deletes document file metadata by file ID.
func (r *SQLiteDocumentFileMetadataRepository) Delete(ctx context.Context, fileId string) error {
	query := `DELETE FROM DocumentFileMetadata WHERE DocumentFileId = ?`
	_, err := r.db.ExecContext(ctx, query, fileId)
	return err
}

// DeleteByDocumentId deletes all metadata for files in a document.
func (r *SQLiteDocumentFileMetadataRepository) DeleteByDocumentId(ctx context.Context, documentId string) error {
	query := `
	DELETE FROM DocumentFileMetadata 
	WHERE DocumentFileId IN (
		SELECT Id FROM DocumentFile WHERE DocumentId = ?
	)`
	_, err := r.db.ExecContext(ctx, query, documentId)
	return err
}

// scanDocumentFileMetadata scans a database row into a DocumentFileMetadata struct.
func scanDocumentFileMetadata(scanner ccc.RowScanner) (*DocumentFileMetadata, error) {
	meta := &DocumentFileMetadata{}

	err := scanner.Scan(
		&meta.DocumentFileId,
		&meta.ExtractedText,
		&meta.OcrConfidence,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	return meta, nil
}
