package auth

import (
	"database/sql"
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteSignInHistoryItemRepository implements SignInHistoryItemRepository using SQLite
type SQLiteSignInHistoryItemRepository struct {
	db *sql.DB
}

// NewSQLiteSignInHistoryItemRepository creates a new SQLite-backed sign-in history repository
func NewSQLiteSignInHistoryItemRepository(db *sql.DB) (*SQLiteSignInHistoryItemRepository, error) {
	repo := &SQLiteSignInHistoryItemRepository{
		db: db,
	}

	if err := repo.initializeTable(); err != nil {
		return nil, err
	}

	return repo, nil
}

// initializeTable creates the sign-in history table if it doesn't exist
func (r *SQLiteSignInHistoryItemRepository) initializeTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS sign_in_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		user_name TEXT,
		ip_address TEXT,
		user_agent TEXT,
		client_type TEXT,
		successful INTEGER NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		denial_reason TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_sign_in_history_user_id ON sign_in_history(user_id);
	CREATE INDEX IF NOT EXISTS idx_sign_in_history_user_name ON sign_in_history(user_name);
	CREATE INDEX IF NOT EXISTS idx_sign_in_history_timestamp ON sign_in_history(timestamp);
	`

	_, err := r.db.Exec(query)
	return err
}

// Add inserts a new sign-in history record
func (r *SQLiteSignInHistoryItemRepository) Add(historyItem *SignInHistoryItem) error {
	query := `
	INSERT INTO sign_in_history (
		user_id, user_name, ip_address, user_agent, client_type, successful, timestamp, denial_reason
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	successful := 0
	if historyItem.Successful {
		successful = 1
	}

	// Format timestamp in SQLite format
	timestampStr := ccc.FormatSQLiteTimestamp(historyItem.Timestamp)

	result, err := r.db.Exec(
		query,
		historyItem.UserId,
		historyItem.UserName,
		historyItem.IPAddress,
		historyItem.UserAgent,
		historyItem.ClientType,
		successful,
		timestampStr,
		historyItem.DenialReason,
	)

	if err != nil {
		return err
	}

	// Get the auto-generated ID and set it on the history item
	lastId, err := result.LastInsertId()
	if err != nil {
		return err
	}

	historyItem.Id = lastId
	return nil
}

// GetByUserId retrieves all sign-in history for a specific user ID
func (r *SQLiteSignInHistoryItemRepository) GetByUserId(userId string) ([]*SignInHistoryItem, error) {
	query := `
	SELECT id, user_id, user_name, ip_address, user_agent, client_type, successful, timestamp, denial_reason
	FROM sign_in_history 
	WHERE user_id = ?
	ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetByUserName retrieves all sign-in history for a specific username
func (r *SQLiteSignInHistoryItemRepository) GetByUserName(userName string) ([]*SignInHistoryItem, error) {
	query := `
	SELECT id, user_id, user_name, ip_address, user_agent, client_type, successful, timestamp, denial_reason
	FROM sign_in_history 
	WHERE user_name = ?
	ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(query, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetRecentFailedSignInsByUserName retrieves recent failed sign-in attempts for a username
func (r *SQLiteSignInHistoryItemRepository) GetRecentFailedSignInsByUserName(userName string, minutesBack int) ([]*SignInHistoryItem, error) {
	query := `
	SELECT id, user_id, user_name, ip_address, user_agent, client_type, successful, timestamp, denial_reason
	FROM sign_in_history 
	WHERE user_name = ? 
	AND successful = 0 
	AND timestamp >= datetime('now', ?)
	ORDER BY timestamp DESC
	`

	timeConstraint := fmt.Sprintf("-%d minutes", minutesBack)
	rows, err := r.db.Query(query, userName, timeConstraint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetRecentFailedSignInsByUserId retrieves recent failed sign-in attempts for a user ID
func (r *SQLiteSignInHistoryItemRepository) GetRecentFailedSignInsByUserId(userId string, minutesBack int) ([]*SignInHistoryItem, error) {
	query := `
	SELECT id, user_id, user_name, ip_address, user_agent, client_type, successful, timestamp, denial_reason
	FROM sign_in_history 
	WHERE user_id = ? 
	AND successful = 0 
	AND timestamp >= datetime('now', ?)
	ORDER BY timestamp DESC
	`

	timeConstraint := fmt.Sprintf("-%d minutes", minutesBack)
	rows, err := r.db.Query(query, userId, timeConstraint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// scanRows is a helper function to scan result rows into SignInHistoryItem objects
func (r *SQLiteSignInHistoryItemRepository) scanRows(rows *sql.Rows) ([]*SignInHistoryItem, error) {
	var result []*SignInHistoryItem

	for rows.Next() {
		var item SignInHistoryItem
		var successful int
		var timestampStr string

		err := rows.Scan(
			&item.Id,
			&item.UserId,
			&item.UserName,
			&item.IPAddress,
			&item.UserAgent,
			&item.ClientType,
			&successful,
			&timestampStr,
			&item.DenialReason,
		)

		if err != nil {
			return nil, err
		}

		// Parse the timestamp
		timestamp, err := ccc.ParseSQLiteTimestamp(timestampStr)
		if err != nil {
			return nil, err
		}

		item.Timestamp = timestamp
		item.Successful = successful == 1
		result = append(result, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
