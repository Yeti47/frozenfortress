package auth

type CreateUserRequest struct {
	UserName string
	Password string
}

type CreateUserResponse struct {
	UserId string
}

type ChangePasswordRequest struct {
	UserId      string
	OldPassword string
	NewPassword string
}

type UserDto struct {
	UserId     string
	UserName   string
	IsActive   bool
	IsLocked   bool
	CreatedAt  string
	ModifiedAt string
}
