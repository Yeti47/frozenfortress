package auth

import (
	"time"
)

type User struct {
	Id           string
	UserName     string
	PasswordHash string
	PasswordSalt string
	Mek          string
	PdkSalt      string
	IsActive     bool
	IsLocked     bool
	CreatedAt    time.Time
	ModifiedAt   time.Time
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
