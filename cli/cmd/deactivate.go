package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// deactivateCmd represents the deactivate command
var deactivateCmd = &cobra.Command{
	Use:   "deactivate <username_or_id>",
	Short: "Deactivate a user account",
	Long: `Deactivate a user account by username or user ID.

Examples:
  frozen-fortress user deactivate john.doe
  frozen-fortress user deactivate 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Resolve user identifier to get user info
		user, err := resolveUserIdentifier(identifier)
		if err != nil {
			return err
		}

		// Deactivate the user
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		success, err := userMgr.DeactivateUser(user.Id)
		if err != nil {
			return err
		}

		if !success {
			return ccc.NewOperationFailedError("deactivate user", "operation returned false")
		}

		// Print success message
		output.PrintSuccess("User deactivated successfully", map[string]interface{}{
			"userId":   user.Id,
			"username": user.UserName,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(deactivateCmd)
}
