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
	GetUserById(id string) (*UserDto, error)
	GetUserByUserName(userName string) (*UserDto, error)
	GetAllUsers() ([]*UserDto, error)
	ActivateUser(id string) (bool, error)
	DeactivateUser(id string) (bool, error)
	LockUser(id string) (bool, error)
	UnlockUser(id string) (bool, error)
	ChangePassword(request ChangePasswordRequest) (bool, error)
	VerifyUserPassword(userId string, password string) (bool, error)
}

type UserIdGenerator interface {
	GenerateUserId() string
}

type SignInManager interface {
	SignIn(w http.ResponseWriter, r *http.Request, request SignInRequest) (SignInResponse, error)
	SignOut(w http.ResponseWriter, r *http.Request, request SignOutRequest) error
	GetCurrentUser(r *http.Request) (*UserDto, error)
	IsSignedIn(r *http.Request) (bool, error)
}
