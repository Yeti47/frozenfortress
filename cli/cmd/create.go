package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <username> <password>",
	Short: "Create a new user account",
	Long: `Create a new user account with the specified username and password.

The username must be unique and the password must meet security requirements.
The new user will be created in an active, unlocked state.

Examples:
  frozen-fortress user create jonathan_smith My$ecureP@ssw0rd
  frozen-fortress user create mark_gordon An0therSecureP@ssw0rd!`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]
		password := args[1]

		// Get user manager
		userMgr, err := userManager()
		if err != nil {
			output.PrintError("Failed to get user manager", err)
			return err
		}

		// Create the user
		request := auth.CreateUserRequest{
			UserName: username,
			Password: password,
		}

		response, err := userMgr.CreateUser(request)
		if err != nil {
			output.PrintError("Failed to create user", err)
			return err
		}

		// Print success message
		output.PrintSuccess("User created successfully", map[string]interface{}{
			"userId":   response.UserId,
			"username": username,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(createCmd)
}
