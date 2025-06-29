package auth

import (
	"database/sql"
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteUserRepository struct {
	db *sql.DB
}

const (
	// Field list for User table queries
	userFieldList = `
    Id, 
    UserName, 
    PasswordHash, 
    PasswordSalt,
	Mek,
	PdkSalt,
    IsActive, 
    IsLocked,
    RecoveryCodeHash,
    RecoveryCodeSalt,
    RecoveryMek,
    RecoveryGenerated,
    CreatedAt,
    ModifiedAt`
)

// Creates a new instance of SQLiteUserRepository
func NewSQLiteUserRepository(db *sql.DB) (*SQLiteUserRepository, error) { // Modified signature
	repo := &SQLiteUserRepository{db: db}

	if err := repo.initializeTable(); err != nil { // Added table initialization
		return nil, err
	}

	return repo, nil
}

// initializeTable creates the user table if it doesn't exist
func (repo *SQLiteUserRepository) initializeTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS User (
		Id TEXT PRIMARY KEY,
		UserName TEXT NOT NULL UNIQUE,
		PasswordHash TEXT NOT NULL,
		PasswordSalt TEXT NOT NULL,
		Mek TEXT NOT NULL,
		PdkSalt TEXT NOT NULL,
		IsActive INTEGER NOT NULL,
		IsLocked INTEGER NOT NULL,
		RecoveryCodeHash TEXT,
		RecoveryCodeSalt TEXT,
		RecoveryMek TEXT,
		RecoveryGenerated TIMESTAMP,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_user_username ON User(UserName);
	`
	_, err := repo.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

// Retrieves a user by their ID
func (repo *SQLiteUserRepository) FindById(id string) (*User, error) {

	selectSql := fmt.Sprintf(`
    SELECT %s
    FROM User 
    WHERE Id = ?
    `, userFieldList)

	statement, err := repo.db.Prepare(selectSql)

	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	row := statement.QueryRow(id)

	return scanUser(row)
}

// Retrieves a user by their username
func (repo *SQLiteUserRepository) FindByUserName(userName string) (*User, error) {

	selectSql := fmt.Sprintf(`
    SELECT %s
    FROM User 
    WHERE UserName = ?
    `, userFieldList)

	statement, err := repo.db.Prepare(selectSql)

	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	row := statement.QueryRow(userName)

	return scanUser(row)
}

// Retrieves all users
func (repo *SQLiteUserRepository) GetAll() []*User {

	selectSql := fmt.Sprintf(`
	SELECT %s
	FROM User 
	`, userFieldList)

	statement, err := repo.db.Prepare(selectSql)

	if err != nil {
		return nil
	}

	defer statement.Close()

	rows, err := statement.Query()

	if err != nil {
		return nil
	}

	defer rows.Close()

	var users []*User

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			continue
		}
		users = append(users, user)
	}

	return users

}

// Adds a new user to the database
func (repo *SQLiteUserRepository) Add(user *User) (bool, error) {

	insertSql := fmt.Sprintf(`
	INSERT INTO User (
		%s
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userFieldList)

	statement, err := repo.db.Prepare(insertSql)

	if err != nil {
		return false, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	// Format timestamps
	createdAtStr := ccc.FormatSQLiteTimestamp(user.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(user.ModifiedAt)

	// Format recovery timestamps - handle nil values
	var recoveryGeneratedStr string

	if !user.RecoveryGenerated.IsZero() {
		recoveryGeneratedStr = ccc.FormatSQLiteTimestamp(user.RecoveryGenerated)
	}

	result, err := statement.Exec(
		user.Id,
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.Mek,
		user.PdkSalt,
		user.IsActive,
		user.IsLocked,
		user.RecoveryCodeHash,
		user.RecoveryCodeSalt,
		user.RecoveryMek,
		recoveryGeneratedStr,
		createdAtStr,
		modifiedAtStr,
	)

	if err != nil {
		return false, fmt.Errorf("executing statement: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}

// Removes a user from the database
func (repo *SQLiteUserRepository) Remove(id string) (bool, error) {

	// Start a transaction to ensure all deletions succeed or all fail
	tx, err := repo.db.Begin()
	if err != nil {
		return false, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if tx.Commit() succeeds

	// Delete all document-related entities belonging to this user
	// We need to delete in the correct order to handle foreign key constraints

	// 1. Delete DocumentFileMetadata for all files in documents owned by this user
	deleteDocumentFileMetadataSql := `
	DELETE FROM DocumentFileMetadata 
	WHERE DocumentFileId IN (
		SELECT df.Id FROM DocumentFile df 
		INNER JOIN Document d ON df.DocumentId = d.Id 
		WHERE d.UserId = ?
	)`
	_, err = tx.Exec(deleteDocumentFileMetadataSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting document file metadata: %w", err)
	}

	// 2. Delete DocumentFiles for all documents owned by this user
	deleteDocumentFilesSql := `
	DELETE FROM DocumentFile 
	WHERE DocumentId IN (
		SELECT Id FROM Document WHERE UserId = ?
	)`
	_, err = tx.Exec(deleteDocumentFilesSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting document files: %w", err)
	}

	// 3. Delete DocumentTag associations for documents owned by this user
	deleteDocumentTagsSql := `
	DELETE FROM DocumentTag 
	WHERE DocumentId IN (
		SELECT Id FROM Document WHERE UserId = ?
	)`
	_, err = tx.Exec(deleteDocumentTagsSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting document tags: %w", err)
	}

	// 4. Delete all Documents owned by this user
	deleteDocumentsSql := `DELETE FROM Document WHERE UserId = ?`
	_, err = tx.Exec(deleteDocumentsSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting documents: %w", err)
	}

	// 5. Delete all Tags owned by this user
	deleteTagsSql := `DELETE FROM Tag WHERE UserId = ?`
	_, err = tx.Exec(deleteTagsSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting tags: %w", err)
	}

	// 6. Delete all secrets belonging to this user
	deleteSecretsSql := `DELETE FROM Secret WHERE UserId = ?`
	_, err = tx.Exec(deleteSecretsSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting user secrets: %w", err)
	}

	// 7. Finally, delete the user
	deleteUserSql := `DELETE FROM User WHERE Id = ?`
	result, err := tx.Exec(deleteUserSql, id)
	if err != nil {
		return false, fmt.Errorf("deleting user: %w", err)
	}

	// Check if the user was actually deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("committing transaction: %w", err)
	}

	return rowsAffected > 0, nil
}

// Updates a user in the database
func (repo *SQLiteUserRepository) Update(user *User) (bool, error) {

	const updateSql = `
	UPDATE User 
	SET 
		UserName = ?, 
		PasswordHash = ?, 
		PasswordSalt = ?, 
		Mek = ?,
		PdkSalt = ?,
		IsActive = ?, 
		IsLocked = ?, 
		RecoveryCodeHash = ?,
		RecoveryCodeSalt = ?,
		RecoveryMek = ?,
		RecoveryGenerated = ?,
		CreatedAt = ?, 
		ModifiedAt = ?
	WHERE Id = ?
	`

	statement, err := repo.db.Prepare(updateSql)

	if err != nil {
		return false, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	// Format timestamps
	createdAtStr := ccc.FormatSQLiteTimestamp(user.CreatedAt)
	modifiedAtStr := ccc.FormatSQLiteTimestamp(user.ModifiedAt)

	// Format recovery timestamps - handle nil values
	var recoveryGeneratedStr string

	if !user.RecoveryGenerated.IsZero() {
		recoveryGeneratedStr = ccc.FormatSQLiteTimestamp(user.RecoveryGenerated)
	}

	result, err := statement.Exec(
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.Mek,
		user.PdkSalt,
		user.IsActive,
		user.IsLocked,
		user.RecoveryCodeHash,
		user.RecoveryCodeSalt,
		user.RecoveryMek,
		recoveryGeneratedStr,
		createdAtStr,
		modifiedAtStr,
		user.Id,
	)

	if err != nil {
		return false, fmt.Errorf("executing statement: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected: %w", err)
	}

	return rowsAffected > 0, nil

}

// Scans a row into a User struct
func scanUser(scanner ccc.RowScanner) (*User, error) {
	user := &User{}
	var createdAtStr string                 // Temporary string for scanning
	var modifiedAtStr string                // Temporary string for scanning
	var recoveryGeneratedStr sql.NullString // Temporary string for scanning recovery timestamp

	err := scanner.Scan(
		&user.Id,
		&user.UserName,
		&user.PasswordHash,
		&user.PasswordSalt,
		&user.Mek,
		&user.PdkSalt,
		&user.IsActive,
		&user.IsLocked,
		&user.RecoveryCodeHash,
		&user.RecoveryCodeSalt,
		&user.RecoveryMek,
		&recoveryGeneratedStr,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	}

	if err != nil {
		return nil, fmt.Errorf("reading user from database: %w", err)
	}

	// Parse timestamps - try multiple formats to handle different SQLite driver behaviors
	createdAt, err := ccc.ParseSQLiteTimestamp(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing CreatedAt timestamp: %w", err)
	}
	user.CreatedAt = createdAt

	modifiedAt, err := ccc.ParseSQLiteTimestamp(modifiedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing ModifiedAt timestamp: %w", err)
	}
	user.ModifiedAt = modifiedAt

	// Parse recovery timestamps
	if recoveryGeneratedStr.Valid && recoveryGeneratedStr.String != "" {
		recoveryGenerated, err := ccc.ParseSQLiteTimestamp(recoveryGeneratedStr.String)
		if err != nil {
			return nil, fmt.Errorf("parsing RecoveryGenerated timestamp: %w", err)
		}
		user.RecoveryGenerated = recoveryGenerated
	}

	return user, nil
}
