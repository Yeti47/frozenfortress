package auth

import (
	"time"
)

// SignInMethod represents the method used for sign-in
type SignInMethod string

const (
	SignInMethodPassword SignInMethod = "PASSWORD"
	SignInMethodRecovery SignInMethod = "RECOVERY"
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
	SignInMethod SignInMethod // SignInMethodPassword or SignInMethodRecovery
	Successful   bool
	Timestamp    time.Time
	DenialReason string
}
