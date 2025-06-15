package documents

import (
	"context"
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDocumentFileRepository implements DocumentFileRepository interface using SQLite.
type SQLiteDocumentFileRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for DocumentFile table queries
	documentFileFieldList = `Id, DocumentId, FileName, ContentType, FileSize, PageCount, FileData, CreatedAt, ModifiedAt`
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
