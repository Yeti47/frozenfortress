package auth

type UserRepository interface {
	FindById(id string) (*User, error)
	FindByUserName(userName string) (*User, error)
	GetAll() []*User
	Add(user *User) (bool, error)
	Remove(id string) (bool, error)
	Update(user *User) (bool, error)
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
}

type UserIdGenerator interface {
	GenerateUserId() string
}
