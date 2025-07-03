package documents

import (
	"context"
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteNoteRepository implements NoteRepository interface using SQLite.
type SQLiteNoteRepository struct {
	db ccc.DBExecutor
}

const (
	// Field list for Note table queries
	noteFieldList = `Id, DocumentId, UserId, Content, CreatedAt, ModifiedAt`
)

// newSQLiteNoteRepository creates a new SQLiteNoteRepository instance.
func newSQLiteNoteRepository(db ccc.DBExecutor) NoteRepository {
	repo := &SQLiteNoteRepository{db: db}

	// Initialize table if we have a *sql.DB (not transaction)
	if sqlDB, ok := db.(*sql.DB); ok {
		if err := repo.initializeTable(sqlDB); err != nil {
			// Log error but don't fail - table might already exist
		}
	}

	return repo
}

// initializeTable creates the Note table if it doesn't exist
func (r *SQLiteNoteRepository) initializeTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS Note (
		Id TEXT PRIMARY KEY,
		DocumentId TEXT NOT NULL,
		UserId TEXT NOT NULL,
		Content TEXT NOT NULL,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_note_documentid ON Note(DocumentId);
	CREATE INDEX IF NOT EXISTS idx_note_userid ON Note(UserId);
	`
	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	// Try to add foreign key constraints
	fkQueries := []string{
		`ALTER TABLE Note ADD CONSTRAINT fk_note_documentid 
		FOREIGN KEY (DocumentId) REFERENCES Document(Id) ON DELETE CASCADE;`,
		`ALTER TABLE Note ADD CONSTRAINT fk_note_userid 
		FOREIGN KEY (UserId) REFERENCES User(Id) ON DELETE CASCADE;`,
	}

	for _, fkQuery := range fkQueries {
		_, fkErr := db.Exec(fkQuery)
		if fkErr != nil {
			// Log or ignore the error - foreign key constraints are optional
		}
	}

	return nil
}

// FindById finds a note by its ID.
func (r *SQLiteNoteRepository) FindById(ctx context.Context, noteId string) (*Note, error) {
	query := `SELECT ` + noteFieldList + ` FROM Note WHERE Id = ?`
	row := r.db.QueryRowContext(ctx, query, noteId)
	return scanNote(row)
}

// FindByDocumentId finds all notes for a document.
func (r *SQLiteNoteRepository) FindByDocumentId(ctx context.Context, documentId string) ([]*Note, error) {
	query := `SELECT ` + noteFieldList + ` FROM Note WHERE DocumentId = ? ORDER BY CreatedAt DESC`
	rows, err := r.db.QueryContext(ctx, query, documentId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			continue // Skip problematic rows
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

// Add adds a new note.
func (r *SQLiteNoteRepository) Add(ctx context.Context, note *Note) error {
	query := `INSERT INTO Note (` + noteFieldList + `) VALUES (?, ?, ?, ?, ?, ?)`

	createdAtStr := ccc.FormatSQLiteTimestamp(note.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(note.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		note.Id,
		note.DocumentId,
		note.UserId,
		note.Content,
		createdAtStr,
		modifiedAtStr,
	)
	return err
}

// Update updates an existing note.
func (r *SQLiteNoteRepository) Update(ctx context.Context, note *Note) error {
	query := `UPDATE Note SET Content = ?, ModifiedAt = ? WHERE Id = ?`

	modifiedAtStr := ccc.FormatSQLiteTimestamp(note.ModifiedAt)

	_, err := r.db.ExecContext(ctx, query,
		note.Content,
		modifiedAtStr,
		note.Id,
	)
	return err
}

// Delete deletes a note by its ID.
func (r *SQLiteNoteRepository) Delete(ctx context.Context, noteId string) error {
	query := `DELETE FROM Note WHERE Id = ?`
	_, err := r.db.ExecContext(ctx, query, noteId)
	return err
}

// DeleteByDocumentId deletes all notes for a document.
func (r *SQLiteNoteRepository) DeleteByDocumentId(ctx context.Context, documentId string) error {
	query := `DELETE FROM Note WHERE DocumentId = ?`
	_, err := r.db.ExecContext(ctx, query, documentId)
	return err
}

// scanNote scans a database row into a Note struct.
func scanNote(scanner ccc.RowScanner) (*Note, error) {
	note := &Note{}
	var createdAtStr, modifiedAtStr string

	err := scanner.Scan(
		&note.Id,
		&note.DocumentId,
		&note.UserId,
		&note.Content,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	note.CreatedAt, err = ccc.ParseSQLiteTimestamp(createdAtStr)
	if err != nil {
		return nil, err
	}
	note.ModifiedAt, err = ccc.ParseSQLiteTimestamp(modifiedAtStr)
	if err != nil {
		return nil, err
	}

	return note, nil
}
