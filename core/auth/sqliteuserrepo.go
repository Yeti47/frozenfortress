package auth

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)


type SQLiteUserRepository struct {
    db *sql.DB
}


func (repo *SQLiteUserRepository) FindById(id string) (*User, error) {
    
    const sql = `
    SELECT * FROM User WHERE Id = ?
    `
    transaction, err := repo.db.Begin()

    if err != nil {
        return (nil, err)
    }

    statement, err := transaction.Prepare(sql)

    defer statement.Close()

    if err != nil {
	return (nil, err)
    }





}
