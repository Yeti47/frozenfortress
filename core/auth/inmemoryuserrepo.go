package auth

import "slices"

type InMemoryUserRepository struct {
	users []*User
}

// Creates a new instance of InMemoryUserRepository
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users: make([]*User, 0),
	}
}

// Retrieves a user by their ID
func (repo *InMemoryUserRepository) FindById(id string) (*User, error) {

	for _, user := range repo.users {
		if user.Id == id {
			return user, nil
		}
	}
	return nil, nil
}

// Retrieves a user by their username
func (repo *InMemoryUserRepository) FindByUserName(userName string) (*User, error) {

	for _, user := range repo.users {
		if user.UserName == userName {
			return user, nil
		}
	}
	return nil, nil
}

// Retrieves all users
func (repo *InMemoryUserRepository) GetAll() []*User {
	return repo.users
}

// Adds a new user to the repository
func (repo *InMemoryUserRepository) Add(user *User) (bool, error) {
	repo.users = append(repo.users, user)
	return true, nil
}

// Updates an existing user in the repository
func (repo *InMemoryUserRepository) Update(user *User) (bool, error) {

	for i, u := range repo.users {
		if u.Id == user.Id {
			repo.users[i] = user
			return true, nil
		}
	}

	return false, nil
}

// Deletes a user from the repository
func (repo *InMemoryUserRepository) Remove(user *User) (bool, error) {

	for i, u := range repo.users {
		if u.Id == user.Id {
			repo.users = slices.Delete(repo.users, i, i+1)
			return true, nil
		}
	}

	return false, nil
}
