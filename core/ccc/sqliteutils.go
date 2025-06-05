package ccc

import (
	"fmt"
	"time"
)

// ParseSQLiteTimestamp parses a timestamp string from SQLite using multiple formats
// to handle different SQLite driver behaviors and format variations.
func ParseSQLiteTimestamp(timestampStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",      // Standard SQLite format
		time.RFC3339,               // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05Z",     // RFC3339 without timezone
		"2006-01-02T15:04:05",      // ISO format without timezone
		time.RFC3339Nano,           // With nanoseconds
		"2006-01-02T15:04:05.000Z", // With milliseconds
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp '%s' with any known format", timestampStr)
}

// FormatSQLiteTimestamp formats a time.Time to the standard SQLite timestamp format.
func FormatSQLiteTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
