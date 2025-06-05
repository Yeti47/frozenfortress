package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use:   "activate <username_or_id>",
	Short: "Activate a user account",
	Long: `Activate a user account by username or user ID.

Examples:
  frozen-fortress user activate john.doe
  frozen-fortress user activate 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Resolve user identifier to get user info
		user, err := resolveUserIdentifier(identifier)
		if err != nil {
			return err
		}

		// Activate the user
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		success, err := userMgr.ActivateUser(user.Id)
		if err != nil {
			return err
		}

		if !success {
			return ccc.NewOperationFailedError("activate user", "operation returned false")
		}

		// Print success message
		output.PrintSuccess("User activated successfully", map[string]interface{}{
			"userId":   user.Id,
			"username": user.UserName,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(activateCmd)
}
