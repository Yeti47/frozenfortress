package documents

import (
	"context"
	"database/sql"
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDocumentRepository implements DocumentRepository interface using SQLite.
type SQLiteDocumentRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for Document table queries
	documentFieldList = `Id, UserId, Title, Description, CreatedAt, ModifiedAt`
)

// newSQLiteDocumentRepository creates a new SQLiteDocumentRepository instance.
func newSQLiteDocumentRepository(db ccc.DBExecutor) DocumentRepository {
	repo := &SQLiteDocumentRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
			// In a real application, you'd want proper logging here
		}
	}

	return repo
}

// initializeTable creates the Document table if it doesn't exist
func (r *SQLiteDocumentRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS Document (
		Id TEXT PRIMARY KEY,
		UserId TEXT NOT NULL,
		Title TEXT NOT NULL,
		Description TEXT,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_document_userid ON Document(UserId);
	CREATE INDEX IF NOT EXISTS idx_document_created ON Document(CreatedAt);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraint from Document.UserId to User.Id
	// If this fails, it's okay and we can proceed anyway
	fkQuery := `
	ALTER TABLE Document ADD CONSTRAINT fk_document_userid 
	FOREIGN KEY (UserId) REFERENCES User(Id) ON DELETE CASCADE;
	`
	_, fkErr := db.Exec(fkQuery)
	if fkErr != nil {
		// Log or ignore the error - foreign key constraint is optional
		// The constraint might fail if:
		// - User table doesn't exist yet
		// - Constraint already exists
		// - SQLite was compiled without foreign key support
		// We continue anyway as this is not critical for basic functionality
	}

	return nil
}

// FindById finds a document by its ID.
func (r *SQLiteDocumentRepository) FindById(ctx context.Context, documentId string) (*Document, error) {
	query := `SELECT ` + documentFieldList + ` FROM Document WHERE Id = ?`
	row := r.db.QueryRowContext(ctx, query, documentId)
	return scanDocument(row)
}

// FindByUserId finds all documents for a user.
func (r *SQLiteDocumentRepository) FindByUserId(ctx context.Context, userId string) ([]*Document, error) {
	query := `SELECT ` + documentFieldList + ` FROM Document WHERE UserId = ? ORDER BY ModifiedAt DESC`
	rows, err := r.db.QueryContext(ctx, query, userId)
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

// Add adds a new document.
func (r *SQLiteDocumentRepository) Add(ctx context.Context, document *Document) error {
	query := `INSERT INTO Document (` + documentFieldList + `) VALUES (?, ?, ?, ?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(document.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(document.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		document.Id,
		document.UserId,
		document.Title,
		document.Description,
		createdAtStr,
		modifiedAtStr,
	)
	return err
}

// Update updates an existing document.
func (r *SQLiteDocumentRepository) Update(ctx context.Context, document *Document) error {
	query := `UPDATE Document SET Title = ?, Description = ?, ModifiedAt = ? WHERE Id = ?`

	modifiedAtStr := ccc.FormatSQLiteTimestamp(document.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		document.Title,
		document.Description,
		modifiedAtStr,
		document.Id,
	)
	return err
}

// Delete deletes a document by its ID.
func (r *SQLiteDocumentRepository) Delete(ctx context.Context, documentId string) error {
	query := `DELETE FROM Document WHERE Id = ?`
	_, err := r.db.ExecContext(ctx, query, documentId)
	return err
}

// GetFileCountByDocumentId returns the number of files for a document.
func (r *SQLiteDocumentRepository) GetFileCountByDocumentId(ctx context.Context, documentId string) (int, error) {
	query := `SELECT COUNT(*) FROM DocumentFile WHERE DocumentId = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, documentId).Scan(&count)
	return count, err
}

// FindByFilters finds documents for a user with optional date range and tag filtering.
func (r *SQLiteDocumentRepository) FindByFilters(ctx context.Context, userId string, filters SearchFilters) ([]*Document, error) {
	var queryParts []string
	var args []interface{}

	queryParts = append(queryParts, "SELECT "+documentFieldList+" FROM Document")

	// Base WHERE clause
	whereParts := []string{"UserId = ?"}
	args = append(args, userId)

	// Add date range filters
	if filters.DateFrom != nil {
		whereParts = append(whereParts, "CreatedAt >= ?")
		args = append(args, ccc.FormatSQLiteTimestamp(*filters.DateFrom))
	}
	if filters.DateTo != nil {
		whereParts = append(whereParts, "CreatedAt <= ?")
		args = append(args, ccc.FormatSQLiteTimestamp(*filters.DateTo))
	}

	// Add tag filtering if specified
	if len(filters.TagIds) > 0 {
		// Join with DocumentTag table to filter by tags
		queryParts = append(queryParts, "INNER JOIN DocumentTag dt ON Document.Id = dt.DocumentId")

		// Create placeholders for tag IDs
		placeholders := make([]string, len(filters.TagIds))
		for i, tagId := range filters.TagIds {
			placeholders[i] = "?"
			args = append(args, tagId)
		}
		whereParts = append(whereParts, "dt.TagId IN ("+strings.Join(placeholders, ",")+")")
	}

	// Build final query
	queryParts = append(queryParts, "WHERE "+strings.Join(whereParts, " AND "))
	queryParts = append(queryParts, "ORDER BY ModifiedAt DESC")

	query := strings.Join(queryParts, " ")

	rows, err := r.db.QueryContext(ctx, query, args...)
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

// scanDocument scans a database row into a Document struct.
// This function is package-accessible so it can be used by other repositories in the same package.
func scanDocument(scanner ccc.RowScanner) (*Document, error) {
	doc := &Document{}
	var createdAtStr, modifiedAtStr string

	err := scanner.Scan(
		&doc.Id,
		&doc.UserId,
		&doc.Title,
		&doc.Description,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	doc.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
	if err != nil {
		return nil, err
	}
	doc.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
	if err != nil {
		return nil, err
	}

	return doc, nil
}
