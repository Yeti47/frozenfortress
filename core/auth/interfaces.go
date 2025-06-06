package auth

import (
	"net/http"
)

type UserRepository interface {
	FindById(id string) (*User, error)
	FindByUserName(userName string) (*User, error)
	GetAll() []*User
	Add(user *User) (bool, error)
	Remove(id string) (bool, error)
	Update(user *User) (bool, error)
}

type SignInHistoryItemRepository interface {
	Add(historyItem *SignInHistoryItem) error
	GetByUserId(userId string) ([]*SignInHistoryItem, error)
	GetByUserName(userName string) ([]*SignInHistoryItem, error)
	GetRecentFailedSignInsByUserName(userName string, minutesBack int) ([]*SignInHistoryItem, error)
	GetRecentFailedSignInsByUserId(userId string, minutesBack int) ([]*SignInHistoryItem, error)
}

type UserManager interface {
	CreateUser(request CreateUserRequest) (CreateUserResponse, error)
	GetUserById(id string) (UserDto, error)
	GetUserByUserName(userName string) (UserDto, error)
	GetAllUsers() ([]UserDto, error)
	ActivateUser(id string) (bool, error)
	DeactivateUser(id string) (bool, error)
	LockUser(id string) (bool, error)
	UnlockUser(id string) (bool, error)
	ChangePassword(request ChangePasswordRequest) (bool, error)
	IsValidUsername(userName string) bool
	IsValidPassword(password string) (bool, error)
	DeleteUser(id string) (bool, error)
	VerifyPassword(userId string, password string) (bool, error)
	GenerateRecoveryCode(request GenerateRecoveryCodeRequest) (GenerateRecoveryCodeResponse, error)
	GetRecoveryCodeStatus(userId string) (RecoveryCodeStatus, error)
	VerifyRecoveryCode(userId string, recoveryCode string) (bool, error)
}

type SecurityService interface {
	// LockUser locks the user account. User is passed by value to avoid side effects.
	LockUser(user User) (bool, error)
	// UnlockUser unlocks the user account. User is passed by value to avoid side effects.
	UnlockUser(user User) (bool, error)
	// VerifyUserPassword verifies the user's password.
	VerifyUserPassword(user User, password string) (bool, error)
	// UncoverMek reads the user's MEK (Master Encryption Key) from the database.
	UncoverMek(user User, password string) (string, error)
	// EncryptMek encrypts the user's MEK (Master Encryption Key) using the provided password.
	EncryptMek(plainMek string, password string) (encryptedMek string, salt string, err error)
	// GenerateEncryptedMek generates an encrypted MEK using the user's password.
	GenerateEncryptedMek(password string) (encryptedMek string, salt string, err error)
	// EncryptMekWithRecoveryCode encrypts the MEK with recovery code for recovery purposes.
	EncryptMekWithRecoveryCode(plainMek string, recoveryCode string, salt string) (encryptedMek string, err error)
	// GenerateRecoveryCode generates a new recovery code for a user.
	GenerateRecoveryCode() (recoveryCode string, hash string, salt string, err error)
	// VerifyRecoveryCode verifies a recovery code against the stored hash.
	VerifyRecoveryCode(user User, recoveryCode string) (bool, error)
	// RecoverMek recovers the user's MEK using recovery code and re-encrypts with new password.
	RecoverMek(user User, recoveryCode string, newPassword string) (newMek string, newPdkSalt string, err error)
}

type UserIdGenerator interface {
	GenerateUserId() string
}

type SignInHandler interface {
	HandleSignIn(request SignInRequest, context SignInContext) (SignInResult, error)
	HandleRecoverySignIn(request RecoverySignInRequest, context SignInContext) (RecoverySignInResult, error)
}

type SignInManager interface {
	SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error)
	RecoverySignIn(w http.ResponseWriter, r *http.Request, request RecoverySignInRequest) (RecoverySignInResponse, error)
	SignOut(w http.ResponseWriter, r *http.Request) error
	GetCurrentUser(r *http.Request) (UserDto, error)
	IsSignedIn(r *http.Request) (bool, error)
}

type MekStore interface {
	Store(w http.ResponseWriter, r *http.Request, mek string) error
	Retrieve(r *http.Request) (string, error)
	Delete(w http.ResponseWriter, r *http.Request) error
}

// SessionKeyProvider is responsible for providing session signing and encryption keys.
type SessionKeyProvider interface {
	GetSigningKey() ([]byte, error)
	GetEncryptionKey() ([]byte, error)
}
