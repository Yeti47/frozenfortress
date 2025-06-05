package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// unlockCmd represents the unlock command
var unlockCmd = &cobra.Command{
	Use:   "unlock <username_or_id>",
	Short: "Unlock a user account",
	Long: `Unlock a user account by username or user ID.

Examples:
  frozen-fortress user unlock john.doe
  frozen-fortress user unlock 12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Resolve user identifier to get user info
		user, err := resolveUserIdentifier(identifier)
		if err != nil {
			return err
		}

		// Unlock the user
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		success, err := userMgr.UnlockUser(user.Id)
		if err != nil {
			return err
		}

		if !success {
			return ccc.NewOperationFailedError("unlock user", "operation returned false")
		}

		// Print success message
		output.PrintSuccess("User unlocked successfully", map[string]interface{}{
			"userId":   user.Id,
			"username": user.UserName,
		})

		return nil
	},
}

func init() {
	userCmd.AddCommand(unlockCmd)
}
