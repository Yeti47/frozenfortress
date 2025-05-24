package secrets

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteSecretRepository implements the SecretRepository interface using SQLite.
// This repository is encryption-agnostic - it stores and retrieves data as-is without
// any knowledge of encryption. Secret names and values are expected to be pre-encrypted
// when stored and will be returned in their encrypted form when retrieved.
// All encryption/decryption logic is handled at the application layer.
type SQLiteSecretRepository struct {
	db *sql.DB
}

// rowScanner interface for scanning rows from the database, used by scanSecret.
type rowScanner interface {
	Scan(dest ...any) error
}

const (
	// secretFieldList defines the column order for secret queries.
	secretFieldList = `Id, UserId, Name, Value, CreatedAt, ModifiedAt`
)

// NewSQLiteSecretRepository creates a new instance of SQLiteSecretRepository.
// It uses an existing database connection and initializes the secrets table.
// This repository is encryption-agnostic and will store/retrieve data exactly as provided,
// without performing any encryption or decryption operations.
func NewSQLiteSecretRepository(db *sql.DB) (*SQLiteSecretRepository, error) {
	repo := &SQLiteSecretRepository{db: db}

	if err := repo.initializeTable(); err != nil {
		// Do not close db here as it's managed externally
		return nil, fmt.Errorf("initializing secret table: %w", err)
	}

	return repo, nil
}

// initializeTable creates the Secret table if it doesn't already exist.
func (repo *SQLiteSecretRepository) initializeTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS Secret (
		Id TEXT PRIMARY KEY,
		UserId TEXT NOT NULL,
		Name TEXT NOT NULL,
		Value TEXT NOT NULL,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_secret_userid ON Secret(UserId);
	`
	_, err := repo.db.Exec(query)
	if err != nil {
		return fmt.Errorf("executing schema creation: %w", err)
	}

	// Try to add foreign key constraint from Secret[dot]UserId to User[dot]Id
	// If this fails, it's okay and we can proceed anyway
	fkQuery := `
	ALTER TABLE Secret ADD CONSTRAINT fk_secret_userid 
	FOREIGN KEY (UserId) REFERENCES User(Id) ON DELETE CASCADE;
	`
	_, fkErr := repo.db.Exec(fkQuery)
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

// scanSecret scans a database row into a Secret struct.
func scanSecret(scanner rowScanner) (*Secret, error) {
	secret := &Secret{}
	var createdAtStr, modifiedAtStr string

	err := scanner.Scan(
		&secret.Id,
		&secret.UserId,
		&secret.Name,
		&secret.Value,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("scanning secret row: %w", err)
	}

	secret.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing CreatedAt for secret %s: %w", secret.Id, err)
	}
	secret.ModifiedAt, err = time.Parse("2006-01-02 15:04:05", modifiedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing ModifiedAt for secret %s: %w", secret.Id, err)
	}

	return secret, nil
}

// FindById retrieves a secret by its ID.
func (repo *SQLiteSecretRepository) FindById(secretId string) (*Secret, error) {
	query := fmt.Sprintf("SELECT %s FROM Secret WHERE Id = ?", secretFieldList)
	row := repo.db.QueryRow(query, secretId)
	return scanSecret(row)
}

// FindByUserId retrieves all secrets for a given user ID.
func (repo *SQLiteSecretRepository) FindByUserId(userId string) ([]*Secret, error) {
	query := fmt.Sprintf("SELECT %s FROM Secret WHERE UserId = ?", secretFieldList)
	rows, err := repo.db.Query(query, userId)
	if err != nil {
		return nil, fmt.Errorf("querying secrets by user ID %s: %w", userId, err)
	}
	defer rows.Close()

	var secrets []*Secret
	for rows.Next() {
		secret, err := scanSecret(rows)
		if err != nil {
			// Log or handle individual scan errors, potentially skipping problematic rows
			continue
		}
		secrets = append(secrets, secret)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating secret rows for user ID %s: %w", userId, err)
	}
	return secrets, nil
}

// FindByIdForUser retrieves a secret by user ID and secret ID.
func (repo *SQLiteSecretRepository) FindByIdForUser(userId, secretId string) (*Secret, error) {

	query := fmt.Sprintf("SELECT %s FROM Secret WHERE UserId = ? AND Id = ?", secretFieldList)

	row := repo.db.QueryRow(query, userId, secretId)

	return scanSecret(row)
}

// Add adds a new secret to the database.
func (repo *SQLiteSecretRepository) Add(secret *Secret) (bool, error) {
	query := fmt.Sprintf("INSERT INTO Secret (%s) VALUES (?, ?, ?, ?, ?, ?)", secretFieldList)

	stmt, err := repo.db.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("preparing add secret statement: %w", err)
	}
	defer stmt.Close()

	createdAtStr := secret.CreatedAt.Format("2006-01-02 15:04:05")
	modifiedAtStr := secret.ModifiedAt.Format("2006-01-02 15:04:05")

	result, err := stmt.Exec(
		secret.Id,
		secret.UserId,
		secret.Name,
		secret.Value,
		createdAtStr,
		modifiedAtStr,
	)
	if err != nil {
		return false, fmt.Errorf("executing add secret statement for ID %s: %w", secret.Id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected after adding secret ID %s: %w", secret.Id, err)
	}
	return rowsAffected > 0, nil
}

// Remove deletes a secret by its ID.
func (repo *SQLiteSecretRepository) Remove(secretId string) (bool, error) {
	query := "DELETE FROM Secret WHERE Id = ?"
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("preparing remove secret statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(secretId)
	if err != nil {
		return false, fmt.Errorf("executing remove secret statement for ID %s: %w", secretId, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected after removing secret ID %s: %w", secretId, err)
	}
	return rowsAffected > 0, nil
}

// Update modifies an existing secret in the database.
func (repo *SQLiteSecretRepository) Update(secret *Secret) (bool, error) {
	query := `
	UPDATE Secret SET 
		UserId = ?, 
		Name = ?, 
		Value = ?, 
		CreatedAt = ?, 
		ModifiedAt = ?
	WHERE Id = ?`

	stmt, err := repo.db.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("preparing update secret statement: %w", err)
	}
	defer stmt.Close()

	createdAtStr := secret.CreatedAt.Format("2006-01-02 15:04:05")
	modifiedAtStr := secret.ModifiedAt.Format("2006-01-02 15:04:05")

	result, err := stmt.Exec(
		secret.UserId,
		secret.Name,
		secret.Value,
		createdAtStr,
		modifiedAtStr,
		secret.Id,
	)
	if err != nil {
		return false, fmt.Errorf("executing update secret statement for ID %s: %w", secret.Id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected after updating secret ID %s: %w", secret.Id, err)
	}
	return rowsAffected > 0, nil
}
