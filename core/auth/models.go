package auth

import (
	"time"
)

type User struct {
	Id                string
	UserName          string
	PasswordHash      string
	PasswordSalt      string
	Mek               string
	PdkSalt           string
	IsActive          bool
	IsLocked          bool
	RecoveryCodeHash  string
	RecoveryCodeSalt  string
	RecoveryMek       string // MEK encrypted with recovery code for recovery purposes
	RecoveryGenerated time.Time
	RecoveryUsed      *time.Time
	CreatedAt         time.Time
	ModifiedAt        time.Time
}

type SignInHistoryItem struct {
	Id           int64
	UserId       string
	UserName     string
	IPAddress    string
	UserAgent    string
	ClientType   string
	Successful   bool
	Timestamp    time.Time
	DenialReason string
}
