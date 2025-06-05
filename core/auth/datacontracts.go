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
	Id         string
	UserName   string
	IsActive   bool
	IsLocked   bool
	CreatedAt  string
	ModifiedAt string
}

type SignInRequest struct {
	UserName string
	Password string
}

type SignInResponse struct {
	Success bool
	User    UserDto
	Error   string // empty if no error
}

type SignInResult struct {
	Success      bool
	User         *User  // nil if sign-in failed
	Mek          string // empty if sign-in failed
	ErrorMessage string
}

type SignInContext struct {
	ClientType ClientType
	IPAddress  string // IP address of the client making the request (if applicable)
	UserAgent  string // User-Agent string of the client making the request (if applicable)
}

// ClientType represents the type of client making a sign-in request
type ClientType string

const (
	ClientTypeUnknown ClientType = ""
	ClientTypeWeb     ClientType = "WEB"
	ClientTypeCLI     ClientType = "CLI"
	ClientTypeOther   ClientType = "OTHER"
)
