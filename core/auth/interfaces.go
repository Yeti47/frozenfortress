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
}

type UserIdGenerator interface {
	GenerateUserId() string
}

type SignInManager interface {
	SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error)
	SignOut(w http.ResponseWriter, r *http.Request) error
	GetCurrentUser(r *http.Request) (UserDto, error)
	IsSignedIn(r *http.Request) (bool, error)
}

type MekStore interface {
	Store(w http.ResponseWriter, r *http.Request, mek string) error
	Retrieve(r *http.Request) (string, error)
	Delete(w http.ResponseWriter, r *http.Request) error
}
