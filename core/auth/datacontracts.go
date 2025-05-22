package auth

type CreateUserRequest struct {
	UserName string
	Password string
}

type CreateUserResponse struct {
	UserId string
}
