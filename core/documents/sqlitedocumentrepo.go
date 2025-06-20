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

// FindDetailed finds documents for a user with their tags and file counts
func (r *SQLiteDocumentRepository) FindDetailed(ctx context.Context, userId string, filters DocumentFilters) ([]*DocumentDetails, error) {
	var queryParts []string
	var args []interface{}

	// Build the main query with LEFT JOIN to get tags and file counts
	queryParts = append(queryParts, `
		SELECT 
			d.Id, d.UserId, d.Title, d.Description, d.CreatedAt, d.ModifiedAt,
			t.Id as TagId, t.Name as TagName, t.Color as TagColor, t.CreatedAt as TagCreatedAt, t.ModifiedAt as TagModifiedAt,
			COALESCE(fc.FileCount, 0) as FileCount
		FROM Document d
		LEFT JOIN DocumentTag dt ON d.Id = dt.DocumentId
		LEFT JOIN Tag t ON dt.TagId = t.Id
		LEFT JOIN (
			SELECT DocumentId, COUNT(*) as FileCount 
			FROM DocumentFile 
			GROUP BY DocumentId
		) fc ON d.Id = fc.DocumentId`)

	// Base WHERE clause
	whereParts := []string{"d.UserId = ?"}
	args = append(args, userId)

	// Add date range filters
	if filters.DateFrom != nil {
		whereParts = append(whereParts, "d.CreatedAt >= ?")
		args = append(args, ccc.FormatSQLiteTimestamp(*filters.DateFrom))
	}
	if filters.DateTo != nil {
		whereParts = append(whereParts, "d.CreatedAt <= ?")
		args = append(args, ccc.FormatSQLiteTimestamp(*filters.DateTo))
	}

	// Add tag filtering if specified
	if len(filters.TagIds) > 0 {
		// For tag filtering, we need to ensure the document has the specified tags
		// We'll use EXISTS subquery to avoid duplicates in the main result
		placeholders := make([]string, len(filters.TagIds))
		for i, tagId := range filters.TagIds {
			placeholders[i] = "?"
			args = append(args, tagId)
		}
		whereParts = append(whereParts,
			"EXISTS (SELECT 1 FROM DocumentTag dt2 WHERE dt2.DocumentId = d.Id AND dt2.TagId IN ("+strings.Join(placeholders, ",")+"))")
	}

	// Build final query
	queryParts = append(queryParts, "WHERE "+strings.Join(whereParts, " AND "))
	queryParts = append(queryParts, "ORDER BY d.ModifiedAt DESC, t.Name ASC")

	query := strings.Join(queryParts, " ")

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group results by document
	documentMap := make(map[string]*DocumentDetails)

	for rows.Next() {
		var doc Document
		var createdAtStr, modifiedAtStr string
		var tagId, tagName, tagColor, tagCreatedAtStr, tagModifiedAtStr sql.NullString
		var fileCount int

		err := rows.Scan(
			&doc.Id, &doc.UserId, &doc.Title, &doc.Description, &createdAtStr, &modifiedAtStr,
			&tagId, &tagName, &tagColor, &tagCreatedAtStr, &tagModifiedAtStr,
			&fileCount,
		)
		if err != nil {
			continue // Skip problematic rows
		}

		// Parse timestamps
		doc.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
		if err != nil {
			continue
		}
		doc.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
		if err != nil {
			continue
		}

		// Get or create document details
		detail, exists := documentMap[doc.Id]
		if !exists {
			detail = &DocumentDetails{
				Document:  &doc,
				Tags:      []*Tag{},
				FileCount: fileCount,
			}
			documentMap[doc.Id] = detail
		}

		// Add tag if it exists and we haven't seen it for this document
		if tagId.Valid && tagName.Valid {
			// Check if we already have this tag
			tagExists := false
			for _, existingTag := range detail.Tags {
				if existingTag.Id == tagId.String {
					tagExists = true
					break
				}
			}

			if !tagExists {
				tag := &Tag{
					Id:     tagId.String,
					UserId: userId, // We know this from the query
					Name:   tagName.String,
					Color:  tagColor.String,
				}

				if tagCreatedAtStr.Valid {
					tag.CreatedAt, _ = ccc.ParseSQLiteTimestamp(tagCreatedAtStr.String)
				}
				if tagModifiedAtStr.Valid {
					tag.ModifiedAt, _ = ccc.ParseSQLiteTimestamp(tagModifiedAtStr.String)
				}

				detail.Tags = append(detail.Tags, tag)
			}
		}
	}

	// Convert map to slice
	result := make([]*DocumentDetails, 0, len(documentMap))
	for _, detail := range documentMap {
		result = append(result, detail)
	}

	return result, rows.Err()
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
