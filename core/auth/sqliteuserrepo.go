package auth

import (
	"database/sql"
	"fmt"
	"time" // Added time import

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteUserRepository struct {
	db *sql.DB
}

// Interface for scanning rows from the database
type rowScanner interface {
	Scan(dest ...any) error
}

const (
	// Field list for User table queries
	userFieldList = `
    Id, 
    UserName, 
    PasswordHash, 
    PasswordSalt,
	EncryptionKey,
	EncryptionSalt,
    IsActive, 
    IsLocked,
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
		EncryptionKey TEXT NOT NULL,
		EncryptionSalt TEXT NOT NULL,
		IsActive INTEGER NOT NULL,
		IsLocked INTEGER NOT NULL,
		CreatedAt TIMESTAMP NOT NULL,
		ModifiedAt TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_user_username ON User(UserName);
	`
	_, err := repo.db.Exec(query)
	return err
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
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userFieldList)

	statement, err := repo.db.Prepare(insertSql)

	if err != nil {
		return false, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	// Format timestamps
	createdAtStr := user.CreatedAt.Format("2006-01-02 15:04:05")
	modifiedAtStr := user.ModifiedAt.Format("2006-01-02 15:04:05")

	result, err := statement.Exec(
		user.Id,
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.EncryptionKey,
		user.EncryptionSalt,
		user.IsActive,
		user.IsLocked,
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

	deleteSql := `
	DELETE FROM User 
	WHERE Id = ?
	`

	statement, err := repo.db.Prepare(deleteSql)

	if err != nil {
		return false, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	result, err := statement.Exec(id)

	if err != nil {
		return false, fmt.Errorf("executing statement: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("getting rows affected: %w", err)
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
		EncryptionKey = ?,
		EncryptionSalt = ?,
		IsActive = ?, 
		IsLocked = ?, 
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
	createdAtStr := user.CreatedAt.Format("2006-01-02 15:04:05")
	modifiedAtStr := user.ModifiedAt.Format("2006-01-02 15:04:05")

	result, err := statement.Exec(
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.EncryptionKey,
		user.EncryptionSalt,
		user.IsActive,
		user.IsLocked,
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
func scanUser(scanner rowScanner) (*User, error) {
	user := &User{}
	var createdAtStr string  // Temporary string for scanning
	var modifiedAtStr string // Temporary string for scanning
	err := scanner.Scan(
		&user.Id,
		&user.UserName,
		&user.PasswordHash,
		&user.PasswordSalt,
		&user.EncryptionKey,
		&user.EncryptionSalt,
		&user.IsActive,
		&user.IsLocked,
		&createdAtStr,
		&modifiedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	}

	if err != nil {
		return nil, fmt.Errorf("reading user from database: %w", err)
	}

	// Parse timestamps
	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing CreatedAt timestamp: %w", err)
	}
	user.CreatedAt = createdAt

	modifiedAt, err := time.Parse("2006-01-02 15:04:05", modifiedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing ModifiedAt timestamp: %w", err)
	}
	user.ModifiedAt = modifiedAt

	return user, nil
}
