package utils

import (
	"regexp"
)

// UserIdentifierType represents whether an identifier is a username or user ID
type UserIdentifierType int

const (
	UserIdentifierTypeUnknown UserIdentifierType = iota
	UserIdentifierTypeUsername
	UserIdentifierTypeID
)

// DetectUserIdentifierType attempts to determine if an identifier is a username or user ID
// This is a heuristic based on common patterns:
// - UUIDs (user IDs) contain hyphens and are longer
// - Usernames are alphanumeric with underscores, 3-20 characters
func DetectUserIdentifierType(identifier string) UserIdentifierType {
	if identifier == "" {
		return UserIdentifierTypeUnknown
	}

	// Check if it looks like a UUID (contains hyphens and is 36 characters)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if uuidPattern.MatchString(identifier) {
		return UserIdentifierTypeID
	}

	// Check if it looks like a username (alphanumeric with underscores, 3-20 chars)
	usernamePattern := regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
	if usernamePattern.MatchString(identifier) {
		return UserIdentifierTypeUsername
	}

	// If it doesn't match either pattern, assume it's an ID
	// (could be a different ID format)
	return UserIdentifierTypeID
}
