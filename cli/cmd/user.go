package cmd

import (
	"database/sql"
	"sync"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/utils"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/spf13/cobra"
)

// encryptionService returns a singleton instance of the EncryptionService
var encryptionService = func() func() encryption.EncryptionService {
	var instance encryption.EncryptionService
	var once sync.Once

	return func() encryption.EncryptionService {
		once.Do(func() {
			instance = encryption.NewDefaultEncryptionService()
		})
		return instance
	}
}()

// userRepository returns a singleton instance of the UserRepository
var userRepository = func() func() (auth.UserRepository, error) {
	var instance auth.UserRepository
	var once sync.Once
	var initErr error

	return func() (auth.UserRepository, error) {
		once.Do(func() {
			var db *sql.DB

			db, initErr = database()
			if initErr != nil {
				return
			}
			instance, initErr = auth.NewSQLiteUserRepository(db)
		})
		return instance, initErr
	}
}()

// securityService returns a singleton instance of the SecurityService
var securityService = func() func() (auth.SecurityService, error) {
	var instance auth.SecurityService
	var once sync.Once
	var initErr error

	return func() (auth.SecurityService, error) {
		once.Do(func() {
			repoInstance, err := userRepository()
			if err != nil {
				initErr = err
				return
			}

			encServiceInstance := encryptionService()

			instance = auth.NewDefaultSecurityService(repoInstance, encServiceInstance, logger)
		})
		return instance, initErr
	}
}()

// userManager returns a singleton instance of the UserManager
var userManager = func() func() (auth.UserManager, error) {
	var instance auth.UserManager
	var once sync.Once

	return func() (auth.UserManager, error) {
		var initErr error
		once.Do(func() {

			repoInstance, err := userRepository()
			if err != nil {
				initErr = err
				return
			}

			encServiceInstance := encryptionService()

			secServiceInstance, err := securityService()
			if err != nil {
				initErr = err
				return
			}

			userIdGenerator := ccc.NewUuidGenerator()

			// Create user manager using singleton dependencies
			instance = auth.NewDefaultUserManager(
				repoInstance,
				userIdGenerator,
				encServiceInstance,
				secServiceInstance,
				logger,
			)
		})
		return instance, initErr
	}
}()

// userCmd represents the user command group
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management commands",
	Long: `Commands for managing users in the FrozenFortress system.

You can specify users by either their username or user ID.
The system will automatically detect which type of identifier you're using.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show help when user command is called without subcommands
		cmd.Help()
	},
}

// resolveUserIdentifier determines if the identifier is a username or user ID
// and returns the user DTO
func resolveUserIdentifier(identifier string) (auth.UserDto, error) {
	if identifier == "" {
		return auth.UserDto{}, ccc.NewInvalidInputError("user identifier", "cannot be empty")
	}

	userMgr, err := userManager()
	if err != nil {
		return auth.UserDto{}, err
	}

	identifierType := utils.DetectUserIdentifierType(identifier)

	switch identifierType {
	case utils.UserIdentifierTypeUsername:
		return userMgr.GetUserByUserName(identifier)
	case utils.UserIdentifierTypeID:
		return userMgr.GetUserById(identifier)
	default:
		// Try ID first, then username
		user, err := userMgr.GetUserById(identifier)
		if err != nil && !ccc.IsNotFound(err) {
			return auth.UserDto{}, err
		}
		if err == nil {
			return user, nil
		}

		return userMgr.GetUserByUserName(identifier)
	}
}

func init() {
	rootCmd.AddCommand(userCmd)
}

// createDataProtector creates a DataProtector instance for the given user and password
// This function has been moved to secret.go
