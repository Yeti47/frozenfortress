package auth

import (
	"time"
)

type User struct {
	Id             string
	UserName       string
	PasswordHash   string
	PasswordSalt   string
	EncryptionKey  string
	EncryptionSalt string
	IsActive       bool
	IsLocked       bool
	CreatedAt      time.Time
	ModifiedAt     time.Time
}

type SignInHistoryItem struct {
	Id           int64
	UserId       string
	UserName     string
	IPAddress    string
	UserAgent    string
	Successful   bool
	Timestamp    time.Time
	DenialReason string
}
