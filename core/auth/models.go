package auth

import (
    "time" 
)

type User struct {
    Id string,
    UserName string,
    PasswordHash string,
    PasswordSalt string,
    IsActive bool,
    IsLocked bool,
    CreatedAt time.Time,
    ModifiedAt time.Time
}

