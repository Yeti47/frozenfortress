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
	VerifyPassword(userId string, password string) error
	GenerateRecoveryCode(request GenerateRecoveryCodeRequest) (GenerateRecoveryCodeResponse, error)
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
	GenerateId() string
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

// WrappingKeyStore holds the per-session key that wraps values kept in the server-side
// session store (such as the MEK). It must be kept apart from the session store, so
// that the session store's contents alone are never enough to unwrap those values.
type WrappingKeyStore interface {
	// Issue generates a new wrapping key, hands it to the client via w and returns it,
	// so values can be wrapped with it within the same request.
	Issue(w http.ResponseWriter, r *http.Request) (key string, err error)
	// Retrieve returns the wrapping key sent with r, or "" if there is none.
	Retrieve(r *http.Request) (key string, err error)
	// Delete tells the client via w to discard its wrapping key.
	Delete(w http.ResponseWriter, r *http.Request) error
}

// SessionKeyProvider is responsible for providing session signing and encryption keys.
type SessionKeyProvider interface {
	GetSigningKey() ([]byte, error)
	GetEncryptionKey() ([]byte, error)
}
