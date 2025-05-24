package secrets

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteSecretRepository implements the SecretRepository interface using SQLite.
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

// validSecretSortColumns maps filter SortBy values to actual database column names.
var validSecretSortColumns = map[string]string{
	"Id":         "Id",
	"Name":       "Name",
	"CreatedAt":  "CreatedAt",
	"ModifiedAt": "ModifiedAt",
}

// isValidSortColumn checks if the given sortBy string is a valid column name for sorting.
// It returns the actual database column name and true if valid, otherwise an empty string and false.
func isValidSortColumn(sortBy string) (string, bool) {
	col, ok := validSecretSortColumns[sortBy]
	return col, ok
}

// NewSQLiteSecretRepository creates a new instance of SQLiteSecretRepository.
// It uses an existing database connection and initializes the secrets table.
func NewSQLiteSecretRepository(db *sql.DB) (*SQLiteSecretRepository, error) { // Modified signature
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
	CREATE INDEX IF NOT EXISTS idx_secret_userid_name ON Secret(UserId, Name);
	`
	_, err := repo.db.Exec(query)
	if err != nil {
		return fmt.Errorf("executing schema creation: %w", err)
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

// FindByName retrieves a secret by user ID and secret name.
func (repo *SQLiteSecretRepository) FindByName(userId, secretName string) (*Secret, error) {
	query := fmt.Sprintf("SELECT %s FROM Secret WHERE UserId = ? AND Name = ?", secretFieldList)
	row := repo.db.QueryRow(query, userId, secretName)
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

// Filter retrieves secrets based on filter criteria, supporting pagination and sorting.
func (repo *SQLiteSecretRepository) Filter(filter SecretFilter) (secrets []*Secret, totalCount int, err error) {
	var whereClauses []string
	var args []any

	if filter.UserId != "" {
		whereClauses = append(whereClauses, "UserId = ?")
		args = append(args, filter.UserId)
	}
	if filter.Name != "" {
		whereClauses = append(whereClauses, "Name LIKE ?") // Use LIKE for partial matches on name
		args = append(args, "%"+filter.Name+"%")
	}

	whereCondition := ""
	if len(whereClauses) > 0 {
		whereCondition = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM Secret " + whereCondition
	err = repo.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("counting filtered secrets: %w", err)
	}

	if totalCount == 0 {
		return []*Secret{}, 0, nil // No secrets match, return early
	}

	// Build query for fetching secrets with pagination and sorting
	orderByClause := "ORDER BY CreatedAt DESC" // Default sort
	if filter.SortBy != "" {
		if col, ok := isValidSortColumn(filter.SortBy); ok {
			orderByClause = "ORDER BY " + col
			if !filter.SortAsc {
				orderByClause += " DESC"
			}
		}
	}

	limitClause := ""
	offsetClause := ""
	if filter.PageSize > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", filter.PageSize)
		if filter.Page > 0 { // Page is 1-based
			offsetClause = fmt.Sprintf("OFFSET %d", (filter.Page-1)*filter.PageSize)
		}
	}

	dataQuery := fmt.Sprintf("SELECT %s FROM Secret %s %s %s %s",
		secretFieldList,
		whereCondition,
		orderByClause,
		limitClause,
		offsetClause,
	)

	rows, err := repo.db.Query(dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying filtered secrets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		secret, scanErr := scanSecret(rows)
		if scanErr != nil {
			// Log or handle individual scan errors
			continue
		}
		secrets = append(secrets, secret)
	}
	if err = rows.Err(); err != nil {
		return nil, totalCount, fmt.Errorf("iterating filtered secret rows: %w", err)
	}

	return secrets, totalCount, nil
}
