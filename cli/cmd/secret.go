package cmd

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// secretRepository returns a singleton instance of the SecretRepository
var secretRepository = func() func() (secrets.SecretRepository, error) {
	var instance secrets.SecretRepository
	var once sync.Once
	var initErr error

	return func() (secrets.SecretRepository, error) {
		once.Do(func() {
			var db *sql.DB
			// Assuming database() function is accessible from this package (e.g., defined in root.go or a shared cmd file)
			db, initErr = database()
			if initErr != nil {
				return
			}
			instance, initErr = secrets.NewSQLiteSecretRepository(db)
		})
		return instance, initErr
	}
}()

// secretIdGenerator returns a singleton instance of the SecretIdGenerator
var secretIdGenerator = func() func() secrets.SecretIdGenerator {
	var instance secrets.SecretIdGenerator
	var once sync.Once

	return func() secrets.SecretIdGenerator {
		once.Do(func() {
			instance = secrets.NewUuidSecretIdGenerator()
		})
		return instance
	}
}()

// signInHandler returns a singleton instance of the SignInHandler
var signInHandler = func() func() (auth.SignInHandler, error) {
	var instance auth.SignInHandler
	var once sync.Once
	var initErr error

	return func() (auth.SignInHandler, error) {
		once.Do(func() {
			userRepo, err := userRepository()
			if err != nil {
				initErr = err
				return
			}

			db, err := database()
			if err != nil {
				initErr = err
				return
			}

			signInHistoryRepo, err := auth.NewSQLiteSignInHistoryItemRepository(db)
			if err != nil {
				initErr = err
				return
			}

			secService, err := securityService()
			if err != nil {
				initErr = err
				return
			}

			config := ccc.LoadConfigFromEnv()

			encServiceInstance := encryptionService()

			instance = auth.NewDefaultSignInHandler(
				userRepo,
				signInHistoryRepo,
				secService,
				encServiceInstance,
				config,
				logger,
			)
		})
		return instance, initErr
	}
}()

// secretManager returns a singleton instance of the SecretManager
var secretManager = func() func() (secrets.SecretManager, error) {
	var instance secrets.SecretManager
	var once sync.Once
	var initErr error

	return func() (secrets.SecretManager, error) {
		once.Do(func() {
			repo, err := secretRepository()
			if err != nil {
				initErr = err
				return
			}

			idGen := secretIdGenerator()

			userRepo, err := userRepository() // Using userRepository from user.go (package-level visibility)
			if err != nil {
				initErr = err
				return
			}

			instance = secrets.NewDefaultSecretManager(repo, idGen, userRepo, logger)
		})
		return instance, initErr
	}
}()

// promptForPassword handles reading the password from the user.
func promptForPassword() (string, error) {
	fmt.Print("Enter user password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password input
	if err != nil {
		// Fallback for environments where /dev/tty is not available (e.g. Git Bash, some CI)
		// This is less secure as input might be echoed or logged.
		fmt.Println("Warning: Could not read password securely. Falling back to standard input.")
		fmt.Print("Enter user password (input may be visible): ")
		reader := bufio.NewReader(os.Stdin)
		passwordStr, readErr := reader.ReadString('\n')
		if readErr != nil {
			return "", fmt.Errorf("password input failed during fallback: %w", readErr)
		}
		bytePassword = []byte(strings.TrimSpace(passwordStr))
		if len(bytePassword) == 0 {
			return "", fmt.Errorf("password input failed: password is empty after fallback")
		}
	}
	password := string(bytePassword)

	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	return password, nil
}

// authenticateUser verifies the user's credentials using the SignInHandler with proper security measures.
func authenticateUser(userDto auth.UserDto, password string) error {
	handler, err := signInHandler()
	if err != nil {
		return fmt.Errorf("failed to get sign-in handler: %w", err)
	}

	// Create SignInRequest
	request := auth.SignInRequest{
		UserName: userDto.UserName,
		Password: password,
	}

	// Create SignInContext for CLI client
	context := auth.SignInContext{
		ClientType: auth.ClientTypeCLI,
		IPAddress:  "", // Empty for CLI clients
		UserAgent:  "", // Empty for CLI clients
	}

	// Perform authentication
	result, err := handler.HandleSignIn(request, context)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("authentication failed: %s", result.ErrorMessage)
	}

	return nil
}

// prepareSecretOperation encapsulates common steps for secret operations.
func prepareSecretOperation(userIdentifier string) (auth.UserDto, dataprotection.DataProtector, secrets.SecretManager, error) {
	// 1. Resolve user identifier
	userDto, err := resolveUserIdentifier(userIdentifier) // from user.go
	if err != nil {
		return auth.UserDto{}, nil, nil, fmt.Errorf("failed to resolve user: %w", err)
	}

	// 2. Prompt for password
	password, err := promptForPassword()
	if err != nil {
		return auth.UserDto{}, nil, nil, err // Error already formatted
	}

	// 3. Authenticate user
	if err := authenticateUser(userDto, password); err != nil {
		return auth.UserDto{}, nil, nil, err // Error already formatted
	}

	// 4. Create DataProtector
	dataProtector, err := createDataProtector(userDto.Id, password)
	if err != nil {
		return auth.UserDto{}, nil, nil, fmt.Errorf("failed to create data protector: %w", err)
	}

	// 5. Get SecretManager instance
	sm, errManager := secretManager()
	if errManager != nil {
		return auth.UserDto{}, nil, nil, fmt.Errorf("failed to get secret manager: %w", errManager)
	}

	return userDto, dataProtector, sm, nil
}

// secretCmd represents the base command for secret operations
var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
	Long:  `Commands for managing secrets in the FrozenFortress system.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// secretAddCmd represents the command to add a new secret
var secretAddCmd = &cobra.Command{
	Use:   "add <user_identifier> <secret_name> <secret_value>",
	Short: "Add a new secret for a user. Requires user authentication.",
	Long:  `Adds a new secret for the specified user. You need to provide the user's identifier (username or ID), the secret name, and the secret value as arguments. This command requires user authentication.`,
	Args:  cobra.ExactArgs(3), // Expect exactly 3 arguments
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]
		secretName := args[1]
		secretValue := args[2]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Prepare request and create secret
		request := secrets.UpsertSecretRequest{
			SecretName:  secretName,
			SecretValue: secretValue,
		}

		createResp, err := secretManager.CreateSecret(userDto.Id, request, dataProtector)
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}

		fmt.Printf("Secret '%s' created successfully for user '%s' (ID: %s). Secret ID: %s\\n", secretName, userDto.UserName, userDto.Id, createResp.SecretId)
		return nil
	},
}

// secretEditCmd represents the command to edit an existing secret
var secretEditCmd = &cobra.Command{
	Use:   "edit <user_identifier> <secret_name> <new_secret_value>",
	Short: "Edit an existing secret's value. Requires user authentication.",
	Long:  `Updates the value of an existing secret for the specified user. The secret is identified by its current name. This command requires user authentication.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]
		secretName := args[1]
		newSecretValue := args[2]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Get the secret by name
		secretDto, err := secretManager.GetSecretByName(userDto.Id, secretName, dataProtector)
		if err != nil {
			// Return the error directly, as the root command handles ApiErrors.
			return err
		}
		secretIdToUpdate := secretDto.Id

		// Prepare request and update secret
		updateRequest := secrets.UpsertSecretRequest{
			SecretName:  secretDto.Name, // Keep the original name
			SecretValue: newSecretValue,
		}

		success, err := secretManager.UpdateSecret(userDto.Id, secretIdToUpdate, updateRequest, dataProtector)
		if err != nil {
			return fmt.Errorf("failed to update secret \"%s\": %w", secretName, err)
		}

		if !success {
			// It's possible for UpdateSecret to return false without an error if the underlying repo layer does so.
			return fmt.Errorf("update operation for secret \"%s\" reported no success but no error either", secretName)
		}

		fmt.Printf("Secret '%s' updated successfully for user '%s'.\\n", secretName, userDto.UserName)
		return nil
	},
}

// secretRenameCmd represents the command to rename an existing secret
var secretRenameCmd = &cobra.Command{
	Use:   "rename <user_identifier> <old_secret_name> <new_secret_name>",
	Short: "Rename an existing secret. Requires user authentication.",
	Long:  `Renames an existing secret for the specified user. The secret is identified by its current name. This command requires user authentication.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]
		oldSecretName := args[1]
		newSecretName := args[2]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Get the secret by its old name to retrieve its ID and current value
		secretDto, err := secretManager.GetSecretByName(userDto.Id, oldSecretName, dataProtector)
		if err != nil {
			// Error is returned directly as root command handles ApiErrors
			return fmt.Errorf("failed to find secret '%s' to rename: %w", oldSecretName, err)
		}

		// Prepare request with the new name and existing value, then update secret
		updateRequest := secrets.UpsertSecretRequest{
			SecretName:  newSecretName,
			SecretValue: secretDto.Value, // Use the existing value
		}

		success, err := secretManager.UpdateSecret(userDto.Id, secretDto.Id, updateRequest, dataProtector)
		if err != nil {
			return fmt.Errorf("failed to rename secret '%s' to '%s': %w", oldSecretName, newSecretName, err)
		}

		if !success {
			return fmt.Errorf("rename operation for secret '%s' to '%s' reported no success but no error either", oldSecretName, newSecretName)
		}

		fmt.Printf("Secret '%s' successfully renamed to '%s' for user '%s'.\\n", oldSecretName, newSecretName, userDto.UserName)
		return nil
	},
}

// secretListCmd represents the command to list a user's secrets
var secretListCmd = &cobra.Command{
	Use:   "list <user_identifier>",
	Short: "List a user's secrets (names only). Requires user authentication.",
	Long:  `Lists the names of all secrets belonging to the specified user. This command requires user authentication.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Prepare request to get secrets
		getSecretsRequest := secrets.GetSecretsRequest{
			PageSize: 100000,
			Page:     1,
			SortBy:   "Name",
			SortAsc:  true,
		}

		paginatedResponse, err := secretManager.GetSecrets(userDto.Id, getSecretsRequest, dataProtector)
		if err != nil {
			return fmt.Errorf("failed to get secrets for user '%s': %w", userDto.UserName, err)
		}

		if len(paginatedResponse.Secrets) == 0 {
			fmt.Printf("No secrets found for user '%s'.\\n", userDto.UserName)
			return nil
		}

		fmt.Printf("Secrets for user '%s':\\n", userDto.UserName)
		for _, secret := range paginatedResponse.Secrets {
			fmt.Printf("- %s\\n", secret.Name)
		}

		return nil
	},
}

// secretGetCmd represents the command to get a specific secret's value
var secretGetCmd = &cobra.Command{
	Use:   "get <user_identifier> <secret_name>",
	Short: "Get a specific secret's value. Requires user authentication.",
	Long:  `Retrieves and displays the name and value of a specific secret for the specified user. This command requires user authentication.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]
		secretName := args[1]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Get the secret by name
		secretDto, err := secretManager.GetSecretByName(userDto.Id, secretName, dataProtector)
		if err != nil {
			// Return the error directly, as the root command handles ApiErrors.
			return err // This might be a ccc.ResourceNotFoundError if secret doesn't exist
		}

		fmt.Printf("Secret Details for user '%s':\\n", userDto.UserName)
		fmt.Printf("  Name:  %s\\n", secretDto.Name)
		fmt.Printf("  Value: %s\\n", secretDto.Value)

		return nil
	},
}

// secretDeleteCmd represents the command to delete a secret
var secretDeleteCmd = &cobra.Command{
	Use:   "delete <user_identifier> <secret_name>",
	Short: "Delete a specific secret. Requires user authentication.",
	Long:  `Deletes a specific secret for the specified user. The secret is identified by its name. This command requires user authentication.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		userIdentifier := args[0]
		secretName := args[1]

		userDto, dataProtector, secretManager, err := prepareSecretOperation(userIdentifier)
		if err != nil {
			return err
		}

		// Get the secret by name to retrieve its ID
		secretDto, err := secretManager.GetSecretByName(userDto.Id, secretName, dataProtector)
		if err != nil {
			// Return the error directly, as the root command handles ApiErrors (e.g. ccc.ResourceNotFoundError)
			return fmt.Errorf("failed to find secret '%s' to delete: %w", secretName, err)
		}

		// Delete the secret
		success, err := secretManager.DeleteSecret(userDto.Id, secretDto.Id)
		if err != nil {
			return fmt.Errorf("failed to delete secret '%s': %w", secretName, err)
		}

		if !success {
			// This case might indicate an issue if the secret was not found by ID during deletion,
			// though GetSecretByName should have caught it earlier if the name was wrong.
			// Or, it could be a repository layer issue where no rows were affected but no error occurred.
			// Unlikely to occurr, but we handle it gracefully.
			return fmt.Errorf("delete operation for secret '%s' reported no success but no error either", secretName)
		}

		fmt.Printf("Secret '%s' deleted successfully for user '%s'.\\n", secretName, userDto.UserName)
		return nil
	},
}

// createDataProtector creates a DataProtector instance for the given user and password
func createDataProtector(userId, password string) (dataprotection.DataProtector, error) {
	encService := encryptionService()
	secService, err := securityService()
	if err != nil {
		return nil, err
	}

	userRepo, err := userRepository()
	if err != nil {
		return nil, err
	}

	return dataprotection.NewPasswordDataProtector(encService, secService, userRepo, userId, password), nil
}

func init() {
	secretCmd.AddCommand(secretAddCmd)
	secretCmd.AddCommand(secretEditCmd)
	secretCmd.AddCommand(secretRenameCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretDeleteCmd)

	rootCmd.AddCommand(secretCmd)
}
