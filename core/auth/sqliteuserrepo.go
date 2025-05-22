package auth

import (
	"database/sql"
	"fmt"

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
func NewSQLiteUserRepository(db *sql.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{db: db}
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
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, userFieldList)

	statement, err := repo.db.Prepare(insertSql)

	if err != nil {
		return false, fmt.Errorf("preparing statement: %w", err)
	}

	defer statement.Close()

	result, err := statement.Exec(
		user.Id,
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.EncryptionKey,
		user.EncryptionSalt,
		user.IsActive,
		user.IsLocked,
		user.CreatedAt,
		user.ModifiedAt,
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

	result, err := statement.Exec(
		user.UserName,
		user.PasswordHash,
		user.PasswordSalt,
		user.EncryptionKey,
		user.EncryptionSalt,
		user.IsActive,
		user.IsLocked,
		user.CreatedAt,
		user.ModifiedAt,
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
	err := scanner.Scan(
		&user.Id,
		&user.UserName,
		&user.PasswordHash,
		&user.PasswordSalt,
		&user.EncryptionKey,
		&user.EncryptionSalt,
		&user.IsActive,
		&user.IsLocked,
		&user.CreatedAt,
		&user.ModifiedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	}

	if err != nil {
		return nil, fmt.Errorf("reading user from database: %w", err)
	}

	return user, nil
}
