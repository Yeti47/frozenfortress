package auth

type UserRepository interface {
    FindById(id string) (*User, error),
    FindByUserName(userName string) (*User, error),
    GetAll() []*User,
    Add(user *User) (bool, error),
    Remove(id string) (bool, error),
    Update(user *User) (bool, error)
}

