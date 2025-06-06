package auth

type CreateUserRequest struct {
	UserName string
	Password string
}

type CreateUserResponse struct {
	UserId       string
	RecoveryCode string
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

type GenerateRecoveryCodeRequest struct {
	UserId string
}

type GenerateRecoveryCodeResponse struct {
	RecoveryCode string
	Generated    string // timestamp when generated
}

type RecoverySignInRequest struct {
	UserName     string
	RecoveryCode string
	NewPassword  string
}

type RecoverySignInResponse struct {
	Success         bool
	User            UserDto
	NewRecoveryCode string // The new recovery code generated after successful recovery
	Error           string
}

type RecoverySignInResult struct {
	Success         bool
	User            *User
	Mek             string
	NewRecoveryCode string // The new recovery code generated after successful recovery
	ErrorMessage    string
}
