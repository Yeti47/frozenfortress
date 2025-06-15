package documents

import (
	"context"
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteTagRepository implements TagRepository interface using SQLite.
type SQLiteTagRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for Tag table queries
	tagFieldList = `Id, UserId, Name, Color, CreatedAt, ModifiedAt`
)

// newSQLiteTagRepository creates a new SQLiteTagRepository instance.
func newSQLiteTagRepository(db ccc.DBExecutor) TagRepository {
	repo := &SQLiteTagRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
		}
	}

	return repo
}

// initializeTable creates the Tag table if it doesn't exist
func (r *SQLiteTagRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS Tag (
		Id TEXT PRIMARY KEY,
		UserId TEXT NOT NULL,
		Name TEXT NOT NULL,
		Color TEXT NOT NULL DEFAULT '#007bff',
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_tag_userid ON Tag(UserId);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_user_name ON Tag(UserId, Name);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraint from Tag.UserId to User.Id
	fkQuery := `
	ALTER TABLE Tag ADD CONSTRAINT fk_tag_userid 
	FOREIGN KEY (UserId) REFERENCES User(Id) ON DELETE CASCADE;
	`
	_, fkErr := db.Exec(fkQuery)
	if fkErr != nil {
		// Log or ignore the error - foreign key constraint is optional
	}

	return nil
}

// FindById finds a tag by its ID.
func (r *SQLiteTagRepository) FindById(ctx context.Context, tagId string) (*Tag, error) {
	query := `SELECT ` + tagFieldList + ` FROM Tag WHERE Id = ?`
	row := r.db.QueryRowContext(ctx, query, tagId)
	return scanTag(row)
}

// FindByUserId finds all tags for a user.
func (r *SQLiteTagRepository) FindByUserId(ctx context.Context, userId string) ([]*Tag, error) {
	query := `SELECT ` + tagFieldList + ` FROM Tag WHERE UserId = ? ORDER BY Name ASC`
	rows, err := r.db.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// FindByDocumentId finds all tags for a document.
func (r *SQLiteTagRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*Tag, error) {
	query := `
	SELECT t.Id, t.UserId, t.Name, t.Color, t.CreatedAt, t.ModifiedAt 
	FROM Tag t
	INNER JOIN DocumentTag dt ON t.Id = dt.TagId
	WHERE dt.DocumentId = ?
	ORDER BY t.Name ASC`

	rows, err := r.db.QueryContext(ctx, query, documentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// Add adds a new tag.
func (r *SQLiteTagRepository) Add(ctx context.Context, tag *Tag) error {
	query := `INSERT INTO Tag (` + tagFieldList + `) VALUES (?, ?, ?, ?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(tag.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(tag.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		tag.Id,
		tag.UserId,
		tag.Name,
		tag.Color,
		createdAtStr,
		modifiedAtStr,
	)
	return err
}

// Update updates an existing tag.
func (r *SQLiteTagRepository) Update(ctx context.Context, tag *Tag) error {
	query := `UPDATE Tag SET Name = ?, Color = ?, ModifiedAt = ? WHERE Id = ?`

	modifiedAtStr := ccc.FormatSQLiteTimestamp(tag.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		tag.Name,
		tag.Color,
		modifiedAtStr,
		tag.Id,
	)
	return err
}

// Delete deletes a tag by its ID.
func (r *SQLiteTagRepository) Delete(ctx context.Context, tagId string) error {
	query := `DELETE FROM Tag WHERE Id = ?`
	_, err := r.db.ExecContext(ctx, query, tagId)
	return err
}

// scanTag scans a database row into a Tag struct.
func scanTag(scanner ccc.RowScanner) (*Tag, error) {
	tag := &Tag{}
	var createdAtStr, modifiedAtStr string

	err := scanner.Scan(
		&tag.Id,
		&tag.UserId,
		&tag.Name,
		&tag.Color,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	tag.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
	if err != nil {
		return nil, err
	}
	tag.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
	if err != nil {
		return nil, err
	}

	return tag, nil
}
