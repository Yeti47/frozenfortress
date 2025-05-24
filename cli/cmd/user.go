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

// userManager returns a singleton instance of the UserManager
var userManager = func() func() (auth.UserManager, error) {
	var instance auth.UserManager
	var once sync.Once

	return func() (auth.UserManager, error) {
		var err error
		once.Do(func() {
			var db *sql.DB
			db, err = database()
			if err != nil {
				return
			}

			// Create dependencies
			encryptionService := encryption.NewDefaultEncryptionService()

			userRepository, repoErr := auth.NewSQLiteUserRepository(db)
			if repoErr != nil {
				err = repoErr
				return
			}

			userIdGenerator := auth.NewUuidUserIdGenerator()
			securityService := auth.NewDefaultSecurityService(userRepository, encryptionService)

			// Create user manager
			instance = auth.NewDefaultUserManager(
				userRepository,
				userIdGenerator,
				encryptionService,
				securityService,
			)
		})
		return instance, err
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
