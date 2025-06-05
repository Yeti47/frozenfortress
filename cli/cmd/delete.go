package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <username_or_id>",
	Short: "Delete a user account",
	Long: `Delete a user account by username or user ID.

WARNING: This action is irreversible and will permanently remove the user account
and all associated data from the system.

Examples:
  frozen-fortress user delete john.doe
  frozen-fortress user delete 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Resolve user identifier to get user info
		user, err := resolveUserIdentifier(identifier)
		if err != nil {
			return err
		}

		// Delete the user
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		success, err := userMgr.DeleteUser(user.Id)
		if err != nil {
			return err
		}

		if !success {
			return ccc.NewOperationFailedError("delete user", "user not found or already deleted")
		}

		// Print success message
		output.PrintSuccess("User deleted successfully", map[string]any{
			"userId":   user.Id,
			"username": user.UserName,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(deleteCmd)
}
