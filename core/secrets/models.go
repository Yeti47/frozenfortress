package secrets

import (
	"time"
)

// Secret describes a user's secret
type Secret struct {
	Id         string
	UserId     string
	Name       string
	Value      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}
