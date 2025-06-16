package documents

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDocumentFileRepository implements DocumentFileRepository interface using SQLite.
type SQLiteDocumentFileRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for DocumentFile table queries (excludes preview fields)
	documentFileFieldList = `Id, DocumentId, FileName, ContentType, FileSize, PageCount, FileData, CreatedAt, ModifiedAt`
	// Field list for DocumentFilePreview queries (from DocumentFile table)
	documentFilePreviewFieldList = `Id, PreviewData, PreviewType, Width, Height`
)

// newSQLiteDocumentFileRepository creates a new SQLiteDocumentFileRepository instance.
func newSQLiteDocumentFileRepository(db ccc.DBExecutor) DocumentFileRepository {
	repo := &SQLiteDocumentFileRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
		}
	}

	return repo
}

// initializeTable creates the DocumentFile table if it doesn't exist
func (r *SQLiteDocumentFileRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS DocumentFile (
		Id TEXT PRIMARY KEY,
		DocumentId TEXT NOT NULL,
		FileName TEXT NOT NULL,
		ContentType TEXT NOT NULL,
		FileSize INTEGER NOT NULL,
		PageCount INTEGER DEFAULT 0,
		FileData BLOB NOT NULL,
		PreviewData BLOB,
		PreviewType TEXT,
		Width INTEGER DEFAULT 0,
		Height INTEGER DEFAULT 0,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_documentfile_documentid ON DocumentFile(DocumentId);
	CREATE INDEX IF NOT EXISTS idx_documentfile_created ON DocumentFile(CreatedAt);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraint from DocumentFile.DocumentId to Document.Id
	fkQuery := `
	ALTER TABLE DocumentFile ADD CONSTRAINT fk_documentfile_documentid 
	FOREIGN KEY (DocumentId) REFERENCES Document(Id) ON DELETE CASCADE;
	`
	_, fkErr := db.Exec(fkQuery)
	if fkErr != nil {
		// Log or ignore the error - foreign key constraint is optional
	}

	return nil
}

// FindById finds a document file by its ID.
func (r *SQLiteDocumentFileRepository) FindById(ctx context.Context, fileId string) (*DocumentFile, error) {
	query := `SELECT ` + documentFileFieldList + ` FROM DocumentFile WHERE Id = ?`
	row := r.db.QueryRowContext(ctx, query, fileId)
	return scanDocumentFile(row)
}

// FindByDocumentId finds all files for a document.
func (r *SQLiteDocumentFileRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*DocumentFile, error) {
	query := `SELECT ` + documentFileFieldList + ` FROM DocumentFile WHERE DocumentId = ? ORDER BY CreatedAt ASC`
	rows, err := r.db.QueryContext(ctx, query, documentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*DocumentFile
	for rows.Next() {
		file, err := scanDocumentFile(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// Add adds a new document file.
func (r *SQLiteDocumentFileRepository) Add(ctx context.Context, file *DocumentFile) error {
	query := `INSERT INTO DocumentFile (` + documentFileFieldList + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(file.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(file.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		file.Id,
		file.DocumentId,
		file.FileName,
		file.ContentType,
		file.FileSize,
		file.PageCount,
		file.FileData,
		createdAtStr,
		modifiedAtStr,
	)
	return err
}

// AddWithPreview adds a new document file with optional preview data in a single atomic operation.
// This prevents the ModifiedAt timestamp from being updated twice when creating a file with preview.
func (r *SQLiteDocumentFileRepository) AddWithPreview(ctx context.Context, file *DocumentFile, preview *DocumentFilePreview) error {
	// Build the full field list including preview fields
	fullFieldList := `Id, DocumentId, FileName, ContentType, FileSize, PageCount, FileData, PreviewData, PreviewType, Width, Height, CreatedAt, ModifiedAt`
	query := `INSERT INTO DocumentFile (` + fullFieldList + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(file.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(file.ModifiedAt)

	// Use preview data if provided, otherwise use null/default values
	var previewData []byte
	var previewType string
	var width, height int

	if preview != nil {
		previewData = preview.PreviewData
		previewType = preview.PreviewType
		width = preview.Width
		height = preview.Height
	}

	_, err := r.db.ExecContext(ctx, query,
		file.Id,
		file.DocumentId,
		file.FileName,
		file.ContentType,
		file.FileSize,
		file.PageCount,
		file.FileData,
		previewData,
		previewType,
		width,
		height,
		createdAtStr,
		modifiedAtStr,
	)
	return err
}

// Update updates an existing document file.
func (r *SQLiteDocumentFileRepository) Update(ctx context.Context, file *DocumentFile) error {
	query := `UPDATE DocumentFile SET FileName = ?, ContentType = ?, FileSize = ?, PageCount = ?, FileData = ?, ModifiedAt = ? WHERE Id = ?`

	modifiedAtStr := ccc.FormatSQLiteTimestamp(file.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		file.FileName,
		file.ContentType,
		file.FileSize,
		file.PageCount,
		file.FileData,
		modifiedAtStr,
		file.Id,
	)
	return err
}

// Delete deletes a document file by its ID.
func (r *SQLiteDocumentFileRepository) Delete(ctx context.Context, fileId string) error {
	query := `DELETE FROM DocumentFile WHERE Id = ?`
	_, err := r.db.ExecContext(ctx, query, fileId)
	return err
}

// DeleteByDocumentId deletes all files for a document.
func (r *SQLiteDocumentFileRepository) DeleteByDocumentId(ctx context.Context, documentId string) error {
	query := `DELETE FROM DocumentFile WHERE DocumentId = ?`
	_, err := r.db.ExecContext(ctx, query, documentId)
	return err
}

// FindDetailed finds all files for multiple documents along with their metadata in a single query.
// Returns DocumentFileDetails structs that combine file and metadata information.
// If a file has no metadata, the Metadata field will be nil.
// Supports batching with multiple document IDs for improved performance.
func (r *SQLiteDocumentFileRepository) FindDetailed(ctx context.Context, documentIds []string) ([]*DocumentFileDetails, error) {
	if len(documentIds) == 0 {
		return []*DocumentFileDetails{}, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(documentIds))
	args := make([]interface{}, len(documentIds))
	for i, id := range documentIds {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
	SELECT 
		df.Id, df.DocumentId, df.FileName, df.ContentType, df.FileSize, df.PageCount, df.FileData, df.CreatedAt, df.ModifiedAt,
		dfm.ExtractedText, dfm.OcrConfidence
	FROM DocumentFile df
	LEFT JOIN DocumentFileMetadata dfm ON df.Id = dfm.DocumentFileId
	WHERE df.DocumentId IN (%s)
	ORDER BY df.DocumentId, df.CreatedAt ASC`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fileDetails []*DocumentFileDetails

	for rows.Next() {
		file := &DocumentFile{}
		var createdAtStr, modifiedAtStr string
		var extractedText sql.NullString
		var ocrConfidence sql.NullFloat64

		err := rows.Scan(
			// DocumentFile fields
			&file.Id,
			&file.DocumentId,
			&file.FileName,
			&file.ContentType,
			&file.FileSize,
			&file.PageCount,
			&file.FileData,
			&createdAtStr,
			&modifiedAtStr,
			// DocumentFileMetadata fields (nullable)
			&extractedText,
			&ocrConfidence,
		)
		if err != nil {
			continue // Skip problematic rows
		}

		// Parse DocumentFile timestamps
		file.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
		if err != nil {
			continue
		}
		file.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
		if err != nil {
			continue
		}

		// Create DocumentFileDetails with the file
		fileDetail := &DocumentFileDetails{
			File: file,
		}

		// Handle metadata (might be null if no metadata exists)
		if extractedText.Valid || ocrConfidence.Valid {
			metadata := &DocumentFileMetadata{
				DocumentFileId: file.Id,
				ExtractedText:  extractedText.String,
				OcrConfidence:  float32(ocrConfidence.Float64),
			}
			fileDetail.Metadata = metadata
		}
		// If no metadata, fileDetail.Metadata remains nil

		fileDetails = append(fileDetails, fileDetail)
	}

	return fileDetails, rows.Err()
}

// scanDocumentFile scans a database row into a DocumentFile struct.
func scanDocumentFile(scanner ccc.RowScanner) (*DocumentFile, error) {
	file := &DocumentFile{}
	var createdAtStr, modifiedAtStr string

	err := scanner.Scan(
		&file.Id,
		&file.DocumentId,
		&file.FileName,
		&file.ContentType,
		&file.FileSize,
		&file.PageCount,
		&file.FileData,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	file.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
	if err != nil {
		return nil, err
	}
	file.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// GetPreview retrieves a document file preview by document file ID.
// Returns nil if no preview exists or if the document file doesn't exist.
func (r *SQLiteDocumentFileRepository) GetPreview(ctx context.Context, documentFileId string) (*DocumentFilePreview, error) {
	query := `SELECT ` + documentFilePreviewFieldList + ` FROM DocumentFile WHERE Id = ? AND PreviewData IS NOT NULL`
	row := r.db.QueryRowContext(ctx, query, documentFileId)
	return scanDocumentFilePreview(row)
}

// SetPreview sets the preview data for a document file.
// If the document file doesn't exist, the operation fails.
// This method handles both creating and updating preview data.
func (r *SQLiteDocumentFileRepository) SetPreview(ctx context.Context, preview *DocumentFilePreview, modifiedAt time.Time) error {
	// Update the preview fields and ModifiedAt timestamp
	query := `UPDATE DocumentFile SET PreviewData = ?, PreviewType = ?, Width = ?, Height = ?, ModifiedAt = ? WHERE Id = ?`
	modifiedAtStr := ccc.FormatSQLiteTimestamp(modifiedAt)

	result, err := r.db.ExecContext(ctx, query,
		preview.PreviewData,
		preview.PreviewType,
		preview.Width,
		preview.Height,
		modifiedAtStr,
		preview.DocumentFileId,
	)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows // Document file not found
	}

	return nil
}

// DeletePreview removes the preview data for a document file by nullifying the preview fields.
// This operation is idempotent - if the document file doesn't exist, nothing happens.
func (r *SQLiteDocumentFileRepository) DeletePreview(ctx context.Context, documentFileId string) error {
	// Null out the preview fields and update ModifiedAt timestamp
	query := `UPDATE DocumentFile SET PreviewData = NULL, PreviewType = NULL, Width = 0, Height = 0, ModifiedAt = ? WHERE Id = ?`
	modifiedAtStr := ccc.FormatSQLiteTimestamp(time.Now())

	_, err := r.db.ExecContext(ctx, query, modifiedAtStr, documentFileId)
	return err
}

// scanDocumentFilePreview scans a database row into a DocumentFilePreview struct.
func scanDocumentFilePreview(scanner ccc.RowScanner) (*DocumentFilePreview, error) {
	preview := &DocumentFilePreview{}

	err := scanner.Scan(
		&preview.DocumentFileId,
		&preview.PreviewData,
		&preview.PreviewType,
		&preview.Width,
		&preview.Height,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	return preview, nil
}
