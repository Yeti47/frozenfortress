package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// lockCmd represents the lock command
var lockCmd = &cobra.Command{
	Use:   "lock <username_or_id>",
	Short: "Lock a user account",
	Long: `Lock a user account by username or user ID.

Examples:
  frozen-fortress user lock john.doe
  frozen-fortress user lock 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Resolve user identifier to get user info
		user, err := resolveUserIdentifier(identifier)
		if err != nil {
			return err
		}

		// Lock the user
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		success, err := userMgr.LockUser(user.Id)
		if err != nil {
			return err
		}

		if !success {
			return ccc.NewOperationFailedError("lock user", "operation returned false")
		}

		// Print success message
		output.PrintSuccess("User locked successfully", map[string]interface{}{
			"userId":   user.Id,
			"username": user.UserName,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(lockCmd)
}
